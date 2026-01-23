package workflows

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/bunnydb/bunnydb/flow/activities"
	"github.com/bunnydb/bunnydb/flow/model"
)

const (
	// Signal names
	SignalPause       = "pause"
	SignalResume      = "resume"
	SignalTerminate   = "terminate"
	SignalResync      = "resync"
	SignalResyncTable = "resync-table"
	SignalRetryNow    = "retry-now"
	SignalSyncSchema  = "sync-schema"

	// Query names
	QueryFlowState = "flow-state"
)

// CDCFlowInput is the input to the CDC flow workflow
type CDCFlowInput struct {
	MirrorName      string
	SourcePeer      string
	DestinationPeer string
	TableMappings   []model.TableMapping

	// Options
	DoInitialSnapshot             bool
	MaxBatchSize                  uint32
	IdleTimeoutSeconds            uint64
	SnapshotNumRowsPerPartition   uint32
	SnapshotMaxParallelWorkers    uint32
	SnapshotNumTablesInParallel   uint32

	// Schema replication options
	ReplicateIndexes     bool
	ReplicateForeignKeys bool

	// Resync strategy: "truncate" (default) or "swap" (zero-downtime)
	ResyncStrategy model.ResyncStrategy
}

// CDCFlowWorkflow is the main CDC replication workflow
func CDCFlowWorkflow(
	ctx workflow.Context,
	input *CDCFlowInput,
	state *model.CDCFlowState,
) (*model.CDCFlowState, error) {
	if input == nil {
		return nil, errors.New("invalid input: nil")
	}

	logger := workflow.GetLogger(ctx)

	// Initialize state if not provided (first run)
	if state == nil {
		state = model.NewCDCFlowState(input.MirrorName)
		state.SyncFlowOptions.BatchSize = input.MaxBatchSize
		state.SyncFlowOptions.IdleTimeoutSeconds = input.IdleTimeoutSeconds
		for _, tm := range input.TableMappings {
			state.SyncFlowOptions.TableMappings = append(state.SyncFlowOptions.TableMappings, tm)
		}
	}

	// Register query handler for state
	if err := workflow.SetQueryHandler(ctx, QueryFlowState, func() (*model.CDCFlowState, error) {
		return state, nil
	}); err != nil {
		return state, fmt.Errorf("failed to set query handler: %w", err)
	}

	// Set up signal channels
	pauseChan := workflow.GetSignalChannel(ctx, SignalPause)
	resumeChan := workflow.GetSignalChannel(ctx, SignalResume)
	terminateChan := workflow.GetSignalChannel(ctx, SignalTerminate)
	resyncChan := workflow.GetSignalChannel(ctx, SignalResync)
	resyncTableChan := workflow.GetSignalChannel(ctx, SignalResyncTable)
	retryNowChan := workflow.GetSignalChannel(ctx, SignalRetryNow)
	syncSchemaChan := workflow.GetSignalChannel(ctx, SignalSyncSchema)

	// Handle paused state
	if state.ActiveSignal == model.PauseSignal {
		state.UpdateStatus(model.MirrorStatusPaused)
		logger.Info("mirror is paused, waiting for resume signal")

		selector := workflow.NewSelector(ctx)
		selector.AddReceive(ctx.Done(), func(c workflow.ReceiveChannel, more bool) {})
		selector.AddReceive(resumeChan, func(c workflow.ReceiveChannel, more bool) {
			var payload model.SignalPayload
			c.Receive(ctx, &payload)
			state.ActiveSignal = model.NoopSignal
			logger.Info("received resume signal")
		})
		selector.AddReceive(terminateChan, func(c workflow.ReceiveChannel, more bool) {
			var payload model.SignalPayload
			c.Receive(ctx, &payload)
			state.ActiveSignal = model.TerminateSignal
			logger.Info("received terminate signal while paused")
		})
		selector.AddReceive(resyncChan, func(c workflow.ReceiveChannel, more bool) {
			var payload model.SignalPayload
			c.Receive(ctx, &payload)
			state.ActiveSignal = model.ResyncSignal
			state.IsResync = true
			logger.Info("received resync signal while paused")
		})

		// Wait for signal
		for state.ActiveSignal == model.PauseSignal && ctx.Err() == nil {
			selector.Select(ctx)
		}

		if ctx.Err() != nil {
			state.UpdateStatus(model.MirrorStatusTerminated)
			return state, ctx.Err()
		}

		if state.ActiveSignal == model.TerminateSignal {
			return state, workflow.NewContinueAsNewError(ctx, DropFlowWorkflow, &DropFlowInput{
				MirrorName: input.MirrorName,
				IsResync:   false,
			})
		}

		if state.ActiveSignal == model.ResyncSignal {
			if input.ResyncStrategy == model.ResyncStrategySwap {
				return state, workflow.NewContinueAsNewError(ctx, FullSwapResyncWorkflow, &FullSwapResyncInput{
					MirrorName:      input.MirrorName,
					SourcePeer:      input.SourcePeer,
					DestinationPeer: input.DestinationPeer,
					TableMappings:   input.TableMappings,
					CDCInput:        input,
				})
			}
			return state, workflow.NewContinueAsNewError(ctx, DropFlowWorkflow, &DropFlowInput{
				MirrorName: input.MirrorName,
				IsResync:   true,
				Config:     input,
			})
		}

		// Resume - continue as new
		state.UpdateStatus(model.MirrorStatusRunning)
		return state, workflow.NewContinueAsNewError(ctx, CDCFlowWorkflow, input, state)
	}

	// Setup phase (if not already set up)
	if state.Status == model.MirrorStatusCreated {
		state.UpdateStatus(model.MirrorStatusSettingUp)

		setupCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
			StartToCloseTimeout: 1 * time.Hour,
			HeartbeatTimeout:    5 * time.Minute,
			RetryPolicy: &temporal.RetryPolicy{
				InitialInterval:    1 * time.Minute,
				MaximumAttempts:    20,
				BackoffCoefficient: 2,
			},
		})

		// Run setup activity
		var setupOutput *activities.SetupOutput
		err := workflow.ExecuteActivity(setupCtx, activities.SetupMirrorActivity, &activities.SetupInput{
			MirrorName:           input.MirrorName,
			SourcePeer:           input.SourcePeer,
			DestinationPeer:      input.DestinationPeer,
			TableMappings:        input.TableMappings,
			ReplicateIndexes:     input.ReplicateIndexes,
			ReplicateForeignKeys: input.ReplicateForeignKeys,
		}).Get(setupCtx, &setupOutput)

		if err != nil {
			state.UpdateStatus(model.MirrorStatusFailed)
			state.RecordError(err.Error())
			return state, fmt.Errorf("setup failed: %w", err)
		}

		state.SlotName = setupOutput.SlotName
		state.PublicationName = setupOutput.PublicationName
		state.SyncFlowOptions.SrcTableIDNameMapping = setupOutput.SrcTableIDNameMapping

		// Run snapshot if requested
		if input.DoInitialSnapshot {
			state.UpdateStatus(model.MirrorStatusSnapshot)

			snapshotCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
				WorkflowID: fmt.Sprintf("snapshot-%s", input.MirrorName),
				RetryPolicy: &temporal.RetryPolicy{
					MaximumAttempts: 3,
				},
			})

			err := workflow.ExecuteChildWorkflow(snapshotCtx, SnapshotFlowWorkflow, &SnapshotFlowInput{
				MirrorName:                  input.MirrorName,
				SourcePeer:                  input.SourcePeer,
				DestinationPeer:             input.DestinationPeer,
				TableMappings:               input.TableMappings,
				SlotName:                    setupOutput.SlotName,
				SnapshotName:                setupOutput.SnapshotName,
				NumRowsPerPartition:         input.SnapshotNumRowsPerPartition,
				MaxParallelWorkers:          input.SnapshotMaxParallelWorkers,
				NumTablesInParallel:         input.SnapshotNumTablesInParallel,
				ReplicateIndexes:            input.ReplicateIndexes,
				ReplicateForeignKeys:        input.ReplicateForeignKeys,
			}).Get(snapshotCtx, nil)

			if err != nil {
				state.UpdateStatus(model.MirrorStatusFailed)
				state.RecordError(err.Error())
				return state, fmt.Errorf("snapshot failed: %w", err)
			}
		}

		state.UpdateStatus(model.MirrorStatusRunning)
		state.ClearError()
	}

	// Main CDC loop
	logger.Info("starting CDC sync loop", slog.String("mirror", input.MirrorName))

	// Create a cancellable context for the sync activity so we can cancel it on signals
	syncCtx, cancelSync := workflow.WithCancel(ctx)
	syncCtx = workflow.WithActivityOptions(syncCtx, workflow.ActivityOptions{
		StartToCloseTimeout: 365 * 24 * time.Hour, // Long-running
		HeartbeatTimeout:    1 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 1, // Handle retries in the activity
		},
		WaitForCancellation: true, // Wait for activity to acknowledge cancellation
	})

	// Start sync activity
	syncFuture := workflow.ExecuteActivity(syncCtx, activities.SyncFlowActivity, &activities.SyncInput{
		MirrorName:      input.MirrorName,
		SourcePeer:      input.SourcePeer,
		DestinationPeer: input.DestinationPeer,
		SlotName:        state.SlotName,
		PublicationName: state.PublicationName,
		LastLSN:         state.LastLSN,
		BatchSize:       state.SyncFlowOptions.BatchSize,
		IdleTimeout:     state.SyncFlowOptions.IdleTimeoutSeconds,
		TableMappings:   state.SyncFlowOptions.TableMappings,
	})
	_ = cancelSync // Will be used in signal handlers

	var finished bool
	var syncErr error

	// Main selector for signals and sync completion
	selector := workflow.NewSelector(ctx)

	selector.AddReceive(ctx.Done(), func(c workflow.ReceiveChannel, more bool) {
		finished = true
	})

	selector.AddFuture(syncFuture, func(f workflow.Future) {
		var syncOutput *activities.SyncOutput
		syncErr = f.Get(ctx, &syncOutput)
		if syncErr != nil {
			// Only record error if it's not a cancellation (signal-triggered)
			if !temporal.IsCanceledError(syncErr) && state.ActiveSignal == model.NoopSignal {
				logger.Error("sync error", slog.Any("error", syncErr))
				state.RecordError(syncErr.Error())
			} else {
				// Cancelled due to signal - not an error
				syncErr = nil
				logger.Info("sync cancelled due to signal")
			}
		} else if syncOutput != nil {
			state.LastLSN = syncOutput.LastLSN
			state.LastSyncBatchID = syncOutput.BatchID
			state.ClearError()
		}
		finished = true
	})

	selector.AddReceive(pauseChan, func(c workflow.ReceiveChannel, more bool) {
		var payload model.SignalPayload
		c.Receive(ctx, &payload)
		state.ActiveSignal = model.PauseSignal
		state.UpdateStatus(model.MirrorStatusPausing)
		cancelSync() // Cancel the running sync activity
		finished = true
		logger.Info("received pause signal")
	})

	selector.AddReceive(terminateChan, func(c workflow.ReceiveChannel, more bool) {
		var payload model.SignalPayload
		c.Receive(ctx, &payload)
		state.ActiveSignal = model.TerminateSignal
		cancelSync() // Cancel the running sync activity
		finished = true
		logger.Info("received terminate signal")
	})

	selector.AddReceive(resyncChan, func(c workflow.ReceiveChannel, more bool) {
		var payload model.SignalPayload
		c.Receive(ctx, &payload)
		state.ActiveSignal = model.ResyncSignal
		state.IsResync = true
		cancelSync() // Cancel the running sync activity
		finished = true
		logger.Info("received resync signal")
	})

	selector.AddReceive(resyncTableChan, func(c workflow.ReceiveChannel, more bool) {
		var payload model.SignalPayload
		c.Receive(ctx, &payload)
		state.ActiveSignal = model.ResyncTableSignal
		state.ResyncTableName = payload.TableName
		cancelSync() // Cancel the running sync activity
		finished = true
		logger.Info("received table resync signal", slog.String("table", payload.TableName))
	})

	selector.AddReceive(retryNowChan, func(c workflow.ReceiveChannel, more bool) {
		var payload model.SignalPayload
		c.Receive(ctx, &payload)
		state.ActiveSignal = model.RetryNowSignal
		state.ClearError() // Reset error count AND error message for immediate retry
		cancelSync()       // Cancel the running sync activity
		finished = true
		logger.Info("received retry-now signal")
	})

	selector.AddReceive(syncSchemaChan, func(c workflow.ReceiveChannel, more bool) {
		var payload model.SignalPayload
		c.Receive(ctx, &payload)
		state.ActiveSignal = model.SyncSchemaSignal
		cancelSync() // Cancel the running sync activity
		finished = true
		logger.Info("received sync-schema signal")
	})

	// Wait for sync or signal
	for !finished {
		selector.Select(ctx)
	}

	// Handle different termination cases
	if ctx.Err() != nil {
		state.UpdateStatus(model.MirrorStatusTerminated)
		return state, ctx.Err()
	}

	switch state.ActiveSignal {
	case model.TerminateSignal:
		return state, workflow.NewContinueAsNewError(ctx, DropFlowWorkflow, &DropFlowInput{
			MirrorName: input.MirrorName,
			IsResync:   false,
		})

	case model.ResyncSignal:
		if input.ResyncStrategy == model.ResyncStrategySwap {
			return state, workflow.NewContinueAsNewError(ctx, FullSwapResyncWorkflow, &FullSwapResyncInput{
				MirrorName:      input.MirrorName,
				SourcePeer:      input.SourcePeer,
				DestinationPeer: input.DestinationPeer,
				TableMappings:   input.TableMappings,
				CDCInput:        input,
			})
		}
		return state, workflow.NewContinueAsNewError(ctx, DropFlowWorkflow, &DropFlowInput{
			MirrorName: input.MirrorName,
			IsResync:   true,
			Config:     input,
		})

	case model.ResyncTableSignal:
		// Handle table-level resync
		return state, workflow.NewContinueAsNewError(ctx, TableResyncWorkflow, &TableResyncInput{
			MirrorName:      input.MirrorName,
			TableName:       state.ResyncTableName,
			SourcePeer:      input.SourcePeer,
			DestinationPeer: input.DestinationPeer,
			CDCInput:        input,
			CDCState:        state,
		})

	case model.PauseSignal:
		state.UpdateStatus(model.MirrorStatusPaused)
		return state, workflow.NewContinueAsNewError(ctx, CDCFlowWorkflow, input, state)

	case model.RetryNowSignal:
		// Drop replication slot first to release the connection, then restart CDC
		// This ensures clean restart without "slot is active" errors
		state.ActiveSignal = model.NoopSignal
		state.ClearError() // Reset error count for fresh retry
		return state, workflow.NewContinueAsNewError(ctx, DropFlowWorkflow, &DropFlowInput{
			MirrorName: input.MirrorName,
			IsResync:   true, // Restart CDC after dropping
			Config:     input,
		})

	case model.SyncSchemaSignal:
		// First run schema sync activity to compare and apply DDL changes
		state.ActiveSignal = model.NoopSignal
		state.UpdateStatus(model.MirrorStatusSyncingSchema)

		syncSchemaCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
			StartToCloseTimeout: 30 * time.Minute,
			HeartbeatTimeout:    5 * time.Minute,
			RetryPolicy: &temporal.RetryPolicy{
				InitialInterval: 30 * time.Second,
				MaximumAttempts: 3,
			},
		})

		var syncOutput activities.SyncSchemaOutput
		err := workflow.ExecuteActivity(syncSchemaCtx, activities.SyncSchemaActivity, &activities.SyncSchemaInput{
			MirrorName:       input.MirrorName,
			SourcePeer:       input.SourcePeer,
			DestinationPeer:  input.DestinationPeer,
			TableMappings:    input.TableMappings,
			ReplicateIndexes: input.ReplicateIndexes,
		}).Get(ctx, &syncOutput)
		if err != nil {
			logger.Error("schema sync activity failed", slog.Any("error", err))
			state.SetError(fmt.Sprintf("schema sync failed: %v", err))
		}

		// Drop replication slot and restart CDC to pick up new columns
		return state, workflow.NewContinueAsNewError(ctx, DropFlowWorkflow, &DropFlowInput{
			MirrorName: input.MirrorName,
			IsResync:   true,
			Config:     input,
		})
	}

	// Handle sync error with backoff
	if syncErr != nil {
		// Calculate backoff based on error count
		backoff := time.Duration(1+min(state.ErrorCount, 9)) * time.Minute
		logger.Info("sync failed, will retry after backoff",
			slog.Any("error", syncErr),
			slog.Duration("backoff", backoff))

		// Sleep for backoff duration
		_ = workflow.Sleep(ctx, backoff)
	}

	// Continue as new for the next iteration
	return state, workflow.NewContinueAsNewError(ctx, CDCFlowWorkflow, input, state)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
