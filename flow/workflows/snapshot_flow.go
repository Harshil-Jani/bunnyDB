package workflows

import (
	"fmt"
	"log/slog"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/bunnydb/bunnydb/flow/activities"
	"github.com/bunnydb/bunnydb/flow/model"
)

// SnapshotFlowInput is the input for snapshot workflow
type SnapshotFlowInput struct {
	MirrorName          string
	SourcePeer          string
	DestinationPeer     string
	TableMappings       []model.TableMapping
	SlotName            string
	SnapshotName        string
	NumRowsPerPartition uint32
	MaxParallelWorkers  uint32
	NumTablesInParallel uint32
	ReplicateIndexes    bool
	ReplicateForeignKeys bool
}

// SnapshotFlowWorkflow performs the initial snapshot
func SnapshotFlowWorkflow(ctx workflow.Context, input *SnapshotFlowInput) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("starting snapshot workflow",
		slog.String("mirror", input.MirrorName),
		slog.Int("tables", len(input.TableMappings)))

	// Set default values
	if input.NumRowsPerPartition == 0 {
		input.NumRowsPerPartition = 250000
	}
	if input.MaxParallelWorkers == 0 {
		input.MaxParallelWorkers = 8
	}
	if input.NumTablesInParallel == 0 {
		input.NumTablesInParallel = 4
	}

	activityOpts := workflow.ActivityOptions{
		StartToCloseTimeout: 24 * time.Hour,
		HeartbeatTimeout:    5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    1 * time.Minute,
			BackoffCoefficient: 2,
			MaximumAttempts:    5,
		},
	}

	// Step 1: Drop foreign keys on destination (deferred FK strategy)
	if input.ReplicateForeignKeys {
		logger.Info("dropping foreign keys on destination for initial sync")

		dropFKCtx := workflow.WithActivityOptions(ctx, activityOpts)
		err := workflow.ExecuteActivity(dropFKCtx, activities.DropForeignKeys, &activities.DropFKInput{
			MirrorName:      input.MirrorName,
			DestinationPeer: input.DestinationPeer,
			TableMappings:   input.TableMappings,
		}).Get(dropFKCtx, nil)

		if err != nil {
			return fmt.Errorf("failed to drop foreign keys: %w", err)
		}
	}

	// Step 2: Clone tables in parallel
	logger.Info("starting parallel table cloning",
		slog.Int("parallelism", int(input.NumTablesInParallel)))

	// Create a semaphore to limit parallel table clones
	sem := make(chan struct{}, input.NumTablesInParallel)
	errChan := make(chan error, len(input.TableMappings))
	doneChan := make(chan struct{}, len(input.TableMappings))

	for _, tm := range input.TableMappings {
		mapping := tm // Capture for closure

		// Spawn child workflow for each table
		workflow.Go(ctx, func(ctx workflow.Context) {
			sem <- struct{}{} // Acquire semaphore
			defer func() { <-sem }() // Release semaphore

			tableName := mapping.FullSourceName()
			logger.Info("cloning table", slog.String("table", tableName))

			childOpts := workflow.ChildWorkflowOptions{
				WorkflowID: fmt.Sprintf("clone-%s-%s", input.MirrorName, tableName),
				RetryPolicy: &temporal.RetryPolicy{
					MaximumAttempts: 3,
				},
			}
			childCtx := workflow.WithChildOptions(ctx, childOpts)

			err := workflow.ExecuteChildWorkflow(childCtx, CloneTableWorkflow, &CloneTableInput{
				MirrorName:          input.MirrorName,
				SourcePeer:          input.SourcePeer,
				DestinationPeer:     input.DestinationPeer,
				TableMapping:        mapping,
				SnapshotName:        input.SnapshotName,
				NumRowsPerPartition: input.NumRowsPerPartition,
				MaxParallelWorkers:  input.MaxParallelWorkers,
			}).Get(childCtx, nil)

			if err != nil {
				errChan <- fmt.Errorf("failed to clone table %s: %w", tableName, err)
			}
			doneChan <- struct{}{}
		})
	}

	// Wait for all tables to complete
	for i := 0; i < len(input.TableMappings); i++ {
		select {
		case err := <-errChan:
			// Log error but continue with other tables
			logger.Error("table clone error", slog.Any("error", err))
		case <-doneChan:
			// Table completed
		}
	}

	// Check for any errors
	select {
	case err := <-errChan:
		return err
	default:
	}

	// Step 3: Create indexes on destination
	if input.ReplicateIndexes {
		logger.Info("creating indexes on destination")

		createIdxCtx := workflow.WithActivityOptions(ctx, activityOpts)
		err := workflow.ExecuteActivity(createIdxCtx, activities.CreateIndexes, &activities.CreateIndexesInput{
			MirrorName:      input.MirrorName,
			SourcePeer:      input.SourcePeer,
			DestinationPeer: input.DestinationPeer,
			TableMappings:   input.TableMappings,
			Concurrent:      false, // Use blocking for initial sync
		}).Get(createIdxCtx, nil)

		if err != nil {
			return fmt.Errorf("failed to create indexes: %w", err)
		}
	}

	// Step 4: Recreate foreign keys on destination (with validation)
	if input.ReplicateForeignKeys {
		logger.Info("recreating foreign keys on destination")

		createFKCtx := workflow.WithActivityOptions(ctx, activityOpts)
		err := workflow.ExecuteActivity(createFKCtx, activities.RecreateForeignKeys, &activities.RecreateFKInput{
			MirrorName:      input.MirrorName,
			SourcePeer:      input.SourcePeer,
			DestinationPeer: input.DestinationPeer,
			TableMappings:   input.TableMappings,
			MakeDeferrable:  true,
			Validate:        true,
		}).Get(createFKCtx, nil)

		if err != nil {
			return fmt.Errorf("failed to recreate foreign keys: %w", err)
		}
	}

	logger.Info("snapshot workflow completed successfully")
	return nil
}

// CloneTableInput is the input for cloning a single table
type CloneTableInput struct {
	MirrorName          string
	SourcePeer          string
	DestinationPeer     string
	TableMapping        model.TableMapping
	SnapshotName        string
	NumRowsPerPartition uint32
	MaxParallelWorkers  uint32
}

// CloneTableWorkflow clones a single table from source to destination
func CloneTableWorkflow(ctx workflow.Context, input *CloneTableInput) error {
	logger := workflow.GetLogger(ctx)
	tableName := input.TableMapping.FullSourceName()
	logger.Info("starting table clone", slog.String("table", tableName))

	activityOpts := workflow.ActivityOptions{
		StartToCloseTimeout: 12 * time.Hour,
		HeartbeatTimeout:    5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    1 * time.Minute,
			MaximumAttempts:    5,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, activityOpts)

	// Get partition key and count
	var partitions *activities.PartitionInfo
	err := workflow.ExecuteActivity(ctx, activities.GetPartitionInfo, &activities.GetPartitionInfoInput{
		MirrorName:          input.MirrorName,
		SourcePeer:          input.SourcePeer,
		TableMapping:        input.TableMapping,
		NumRowsPerPartition: input.NumRowsPerPartition,
	}).Get(ctx, &partitions)

	if err != nil {
		return fmt.Errorf("failed to get partition info: %w", err)
	}

	// If no partitions or single partition, do full table copy
	if partitions == nil || partitions.NumPartitions <= 1 {
		logger.Info("copying table without partitioning", slog.String("table", tableName))

		err := workflow.ExecuteActivity(ctx, activities.CopyTable, &activities.CopyTableInput{
			MirrorName:      input.MirrorName,
			SourcePeer:      input.SourcePeer,
			DestinationPeer: input.DestinationPeer,
			TableMapping:    input.TableMapping,
			SnapshotName:    input.SnapshotName,
		}).Get(ctx, nil)

		if err != nil {
			return fmt.Errorf("failed to copy table: %w", err)
		}
	} else {
		// Copy partitions in parallel
		logger.Info("copying table with partitioning",
			slog.String("table", tableName),
			slog.Int("partitions", int(partitions.NumPartitions)),
			slog.Int("workers", int(input.MaxParallelWorkers)))

		sem := make(chan struct{}, input.MaxParallelWorkers)
		errChan := make(chan error, partitions.NumPartitions)
		doneChan := make(chan struct{}, partitions.NumPartitions)

		for i := uint32(0); i < partitions.NumPartitions; i++ {
			partNum := i

			workflow.Go(ctx, func(ctx workflow.Context) {
				sem <- struct{}{}
				defer func() { <-sem }()

				err := workflow.ExecuteActivity(ctx, activities.CopyPartition, &activities.CopyPartitionInput{
					MirrorName:      input.MirrorName,
					SourcePeer:      input.SourcePeer,
					DestinationPeer: input.DestinationPeer,
					TableMapping:    input.TableMapping,
					SnapshotName:    input.SnapshotName,
					PartitionKey:    partitions.PartitionKey,
					PartitionNum:    partNum,
					TotalPartitions: partitions.NumPartitions,
					MinValue:        partitions.MinValue,
					MaxValue:        partitions.MaxValue,
				}).Get(ctx, nil)

				if err != nil {
					errChan <- err
				}
				doneChan <- struct{}{}
			})
		}

		// Wait for all partitions
		for i := uint32(0); i < partitions.NumPartitions; i++ {
			select {
			case err := <-errChan:
				return fmt.Errorf("failed to copy partition: %w", err)
			case <-doneChan:
			}
		}
	}

	logger.Info("table clone completed", slog.String("table", tableName))
	return nil
}
