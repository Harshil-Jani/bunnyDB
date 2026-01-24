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

// TableResyncInput is the input for table-level resync
type TableResyncInput struct {
	MirrorName      string
	TableName       string // schema.table format
	SourcePeer      string
	DestinationPeer string
	CDCInput        *CDCFlowInput  // Original CDC input to resume after resync
	CDCState        *model.CDCFlowState
}

// TableResyncWorkflow resyncs a single table without disrupting the full mirror
func TableResyncWorkflow(ctx workflow.Context, input *TableResyncInput) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("starting table resync",
		slog.String("mirror", input.MirrorName),
		slog.String("table", input.TableName))

	activityOpts := workflow.ActivityOptions{
		StartToCloseTimeout: 12 * time.Hour,
		HeartbeatTimeout:    5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    1 * time.Minute,
			MaximumAttempts:    5,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, activityOpts)

	// Find the table mapping for this table
	var tableMapping *model.TableMapping
	for _, tm := range input.CDCState.SyncFlowOptions.TableMappings {
		if tm.FullSourceName() == input.TableName || tm.FullDestinationName() == input.TableName {
			tableMapping = &tm
			break
		}
	}

	if tableMapping == nil {
		return fmt.Errorf("table %s not found in mirror configuration", input.TableName)
	}

	// Branch based on resync strategy
	if input.CDCInput != nil && input.CDCInput.ResyncStrategy == model.ResyncStrategySwap {
		return tableResyncSwap(ctx, input, tableMapping)
	}

	// Step 1: Mark table as resyncing in catalog
	err := workflow.ExecuteActivity(ctx, activities.UpdateTableSyncStatusActivity, &activities.UpdateTableStatusInput{
		MirrorName: input.MirrorName,
		TableName:  input.TableName,
		Status:     "RESYNCING",
	}).Get(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to update table status: %w", err)
	}

	// Step 2: Drop FKs referencing this table on destination
	logger.Info("dropping foreign keys for table", slog.String("table", input.TableName))
	err = workflow.ExecuteActivity(ctx, activities.DropTableForeignKeysActivity, &activities.DropTableFKInput{
		MirrorName:      input.MirrorName,
		DestinationPeer: input.DestinationPeer,
		TableName:       tableMapping.FullDestinationName(),
	}).Get(ctx, nil)
	if err != nil {
		logger.Warn("failed to drop FKs for table (may not exist)", slog.Any("error", err))
	}

	// Step 3: Truncate destination table
	logger.Info("truncating destination table", slog.String("table", tableMapping.FullDestinationName()))
	err = workflow.ExecuteActivity(ctx, activities.TruncateTableActivity, &activities.TruncateTableInput{
		MirrorName:      input.MirrorName,
		DestinationPeer: input.DestinationPeer,
		TableName:       tableMapping.FullDestinationName(),
	}).Get(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to truncate table: %w", err)
	}

	// Step 4: Copy table data
	logger.Info("copying table data", slog.String("table", input.TableName))
	err = workflow.ExecuteActivity(ctx, activities.CopyTableActivity, &activities.CopyTableInput{
		MirrorName:      input.MirrorName,
		SourcePeer:      input.SourcePeer,
		DestinationPeer: input.DestinationPeer,
		TableMapping:    *tableMapping,
	}).Get(ctx, nil)
	if err != nil {
		// Mark as error and continue with CDC
		_ = workflow.ExecuteActivity(ctx, activities.UpdateTableSyncStatusActivity, &activities.UpdateTableStatusInput{
			MirrorName:   input.MirrorName,
			TableName:    input.TableName,
			Status:       "ERROR",
			ErrorMessage: err.Error(),
		}).Get(ctx, nil)
		return fmt.Errorf("failed to copy table: %w", err)
	}

	// Step 6: Recreate indexes for this table
	logger.Info("recreating indexes for table", slog.String("table", input.TableName))
	err = workflow.ExecuteActivity(ctx, activities.CreateTableIndexesActivity, &activities.CreateTableIndexesInput{
		MirrorName:      input.MirrorName,
		SourcePeer:      input.SourcePeer,
		DestinationPeer: input.DestinationPeer,
		TableMapping:    *tableMapping,
		Concurrent:      true,
	}).Get(ctx, nil)
	if err != nil {
		logger.Warn("failed to create indexes", slog.Any("error", err))
	}

	// Step 7: Recreate FKs for this table
	logger.Info("recreating foreign keys for table", slog.String("table", input.TableName))
	err = workflow.ExecuteActivity(ctx, activities.RecreateTableForeignKeysActivity, &activities.RecreateTableFKInput{
		MirrorName:      input.MirrorName,
		SourcePeer:      input.SourcePeer,
		DestinationPeer: input.DestinationPeer,
		TableName:       tableMapping.FullDestinationName(),
		MakeDeferrable:  true,
		Validate:        true,
	}).Get(ctx, nil)
	if err != nil {
		logger.Warn("failed to recreate FKs", slog.Any("error", err))
	}

	// Step 8: Mark table as synced
	err = workflow.ExecuteActivity(ctx, activities.UpdateTableSyncStatusActivity, &activities.UpdateTableStatusInput{
		MirrorName: input.MirrorName,
		TableName:  input.TableName,
		Status:     "SYNCED",
	}).Get(ctx, nil)
	if err != nil {
		logger.Warn("failed to update table status", slog.Any("error", err))
	}

	// Step 9: Drop replication slot to release the old connection, then restart CDC fresh
	logger.Info("table resync completed, dropping slot for clean restart",
		slog.String("mirror", input.MirrorName),
		slog.String("table", input.TableName))

	input.CDCState.ActiveSignal = model.NoopSignal
	input.CDCState.ResyncTableName = ""

	return workflow.NewContinueAsNewError(ctx, DropFlowWorkflow, &DropFlowInput{
		MirrorName: input.MirrorName,
		IsResync:   true,
		Config:     input.CDCInput,
	})
}

// tableResyncSwap performs zero-downtime table resync by creating a _resync shadow table,
// populating it, then atomically swapping it into place.
func tableResyncSwap(ctx workflow.Context, input *TableResyncInput, tableMapping *model.TableMapping) error {
	logger := workflow.GetLogger(ctx)
	// Step 1: Mark table as resyncing
	err := workflow.ExecuteActivity(ctx, activities.UpdateTableSyncStatusActivity, &activities.UpdateTableStatusInput{
		MirrorName: input.MirrorName,
		TableName:  input.TableName,
		Status:     "RESYNCING",
	}).Get(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to update table status: %w", err)
	}

	// Step 2: Create _resync shadow table (drops existing one first)
	logger.Info("creating resync shadow table", slog.String("table", input.TableName))
	err = workflow.ExecuteActivity(ctx, activities.CreateResyncTableActivity, &activities.CreateResyncTableInput{
		MirrorName:      input.MirrorName,
		SourcePeer:      input.SourcePeer,
		DestinationPeer: input.DestinationPeer,
		TableMapping:    *tableMapping,
	}).Get(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to create resync table: %w", err)
	}

	// Step 3: Copy data into _resync table (use modified mapping with _resync suffix)
	resyncMapping := *tableMapping
	resyncMapping.DestinationTable = tableMapping.DestinationTable + "_resync"

	logger.Info("copying data to resync table",
		slog.String("source", tableMapping.FullSourceName()),
		slog.String("dest", resyncMapping.FullDestinationName()))
	err = workflow.ExecuteActivity(ctx, activities.CopyTableActivity, &activities.CopyTableInput{
		MirrorName:      input.MirrorName,
		SourcePeer:      input.SourcePeer,
		DestinationPeer: input.DestinationPeer,
		TableMapping:    resyncMapping,
	}).Get(ctx, nil)
	if err != nil {
		// Cleanup: drop the resync table on failure
		_ = workflow.ExecuteActivity(ctx, activities.DropResyncTableActivity, &activities.DropResyncTableInput{
			MirrorName:      input.MirrorName,
			DestinationPeer: input.DestinationPeer,
			TableMapping:    *tableMapping,
		}).Get(ctx, nil)
		_ = workflow.ExecuteActivity(ctx, activities.UpdateTableSyncStatusActivity, &activities.UpdateTableStatusInput{
			MirrorName:   input.MirrorName,
			TableName:    input.TableName,
			Status:       "ERROR",
			ErrorMessage: err.Error(),
		}).Get(ctx, nil)
		return fmt.Errorf("failed to copy data to resync table: %w", err)
	}

	// Step 5: Create indexes on the _resync table
	logger.Info("creating indexes on resync table", slog.String("table", resyncMapping.FullDestinationName()))
	err = workflow.ExecuteActivity(ctx, activities.CreateTableIndexesActivity, &activities.CreateTableIndexesInput{
		MirrorName:      input.MirrorName,
		SourcePeer:      input.SourcePeer,
		DestinationPeer: input.DestinationPeer,
		TableMapping:    resyncMapping,
		Concurrent:      false, // Not concurrent since the table isn't live yet
	}).Get(ctx, nil)
	if err != nil {
		logger.Warn("failed to create indexes on resync table (continuing with swap)", slog.Any("error", err))
	}

	// Step 6: Atomically swap the tables
	logger.Info("swapping resync table into place", slog.String("table", input.TableName))
	err = workflow.ExecuteActivity(ctx, activities.SwapTablesActivity, &activities.SwapTablesInput{
		MirrorName:      input.MirrorName,
		SourcePeer:      input.SourcePeer,
		DestinationPeer: input.DestinationPeer,
		TableMapping:    *tableMapping,
		RecreateFKs:     true,
	}).Get(ctx, nil)
	if err != nil {
		// Cleanup on swap failure
		_ = workflow.ExecuteActivity(ctx, activities.DropResyncTableActivity, &activities.DropResyncTableInput{
			MirrorName:      input.MirrorName,
			DestinationPeer: input.DestinationPeer,
			TableMapping:    *tableMapping,
		}).Get(ctx, nil)
		_ = workflow.ExecuteActivity(ctx, activities.UpdateTableSyncStatusActivity, &activities.UpdateTableStatusInput{
			MirrorName:   input.MirrorName,
			TableName:    input.TableName,
			Status:       "ERROR",
			ErrorMessage: err.Error(),
		}).Get(ctx, nil)
		return fmt.Errorf("failed to swap tables: %w", err)
	}

	// Step 7: Mark table as synced
	err = workflow.ExecuteActivity(ctx, activities.UpdateTableSyncStatusActivity, &activities.UpdateTableStatusInput{
		MirrorName: input.MirrorName,
		TableName:  input.TableName,
		Status:     "SYNCED",
	}).Get(ctx, nil)
	if err != nil {
		logger.Warn("failed to update table status after swap", slog.Any("error", err))
	}

	// Step 8: Drop replication slot to release the old connection, then restart CDC fresh
	logger.Info("table swap resync completed, dropping slot for clean restart",
		slog.String("mirror", input.MirrorName),
		slog.String("table", input.TableName))

	input.CDCState.ActiveSignal = model.NoopSignal
	input.CDCState.ResyncTableName = ""

	return workflow.NewContinueAsNewError(ctx, DropFlowWorkflow, &DropFlowInput{
		MirrorName: input.MirrorName,
		IsResync:   true,
		Config:     input.CDCInput,
	})
}

// FullSwapResyncInput is the input for the full-mirror swap resync workflow
type FullSwapResyncInput struct {
	MirrorName      string
	SourcePeer      string
	DestinationPeer string
	TableMappings   []model.TableMapping
	CDCInput        *CDCFlowInput
}

// FullSwapResyncWorkflow performs a zero-downtime full mirror resync by creating _resync
// shadow tables for all tables, populating them, then atomically swapping each into place.
func FullSwapResyncWorkflow(ctx workflow.Context, input *FullSwapResyncInput) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("starting full swap resync",
		slog.String("mirror", input.MirrorName),
		slog.Int("tables", len(input.TableMappings)))

	activityOpts := workflow.ActivityOptions{
		StartToCloseTimeout: 24 * time.Hour,
		HeartbeatTimeout:    5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval: 1 * time.Minute,
			MaximumAttempts: 5,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, activityOpts)

	// Phase 1: Create all _resync shadow tables
	for _, tm := range input.TableMappings {
		logger.Info("creating resync table", slog.String("table", tm.FullSourceName()))
		err := workflow.ExecuteActivity(ctx, activities.CreateResyncTableActivity, &activities.CreateResyncTableInput{
			MirrorName:      input.MirrorName,
			SourcePeer:      input.SourcePeer,
			DestinationPeer: input.DestinationPeer,
			TableMapping:    tm,
		}).Get(ctx, nil)
		if err != nil {
			return fmt.Errorf("failed to create resync table for %s: %w", tm.FullSourceName(), err)
		}
	}

	// Phase 2: Copy data into all _resync tables
	for _, tm := range input.TableMappings {
		resyncMapping := tm
		resyncMapping.DestinationTable = tm.DestinationTable + "_resync"

		logger.Info("copying data to resync table", slog.String("table", tm.FullSourceName()))
		err := workflow.ExecuteActivity(ctx, activities.CopyTableActivity, &activities.CopyTableInput{
			MirrorName:      input.MirrorName,
			SourcePeer:      input.SourcePeer,
			DestinationPeer: input.DestinationPeer,
			TableMapping:    resyncMapping,
		}).Get(ctx, nil)
		if err != nil {
			// Cleanup all resync tables on failure
			for _, cleanTm := range input.TableMappings {
				_ = workflow.ExecuteActivity(ctx, activities.DropResyncTableActivity, &activities.DropResyncTableInput{
					MirrorName:      input.MirrorName,
					DestinationPeer: input.DestinationPeer,
					TableMapping:    cleanTm,
				}).Get(ctx, nil)
			}
			return fmt.Errorf("failed to copy data for %s: %w", tm.FullSourceName(), err)
		}
	}

	// Phase 3: Create indexes on all _resync tables
	for _, tm := range input.TableMappings {
		resyncMapping := tm
		resyncMapping.DestinationTable = tm.DestinationTable + "_resync"

		logger.Info("creating indexes on resync table", slog.String("table", resyncMapping.FullDestinationName()))
		err := workflow.ExecuteActivity(ctx, activities.CreateTableIndexesActivity, &activities.CreateTableIndexesInput{
			MirrorName:      input.MirrorName,
			SourcePeer:      input.SourcePeer,
			DestinationPeer: input.DestinationPeer,
			TableMapping:    resyncMapping,
			Concurrent:      false,
		}).Get(ctx, nil)
		if err != nil {
			logger.Warn("failed to create indexes on resync table (continuing)", slog.Any("error", err))
		}
	}

	// Phase 4: Swap all tables atomically (one by one — each swap is atomic per table)
	for _, tm := range input.TableMappings {
		logger.Info("swapping table", slog.String("table", tm.FullDestinationName()))
		err := workflow.ExecuteActivity(ctx, activities.SwapTablesActivity, &activities.SwapTablesInput{
			MirrorName:      input.MirrorName,
			SourcePeer:      input.SourcePeer,
			DestinationPeer: input.DestinationPeer,
			TableMapping:    tm,
			RecreateFKs:     true,
		}).Get(ctx, nil)
		if err != nil {
			return fmt.Errorf("failed to swap table %s: %w", tm.FullDestinationName(), err)
		}
	}

	// Phase 5: Drop replication slot and publication (will be recreated by CDC)
	logger.Info("dropping source replication for fresh restart")
	err := workflow.ExecuteActivity(ctx, activities.DropSourceReplicationActivity, &activities.DropSourceInput{
		MirrorName: input.MirrorName,
	}).Get(ctx, nil)
	if err != nil {
		logger.Warn("failed to drop source replication", slog.Any("error", err))
	}

	// Phase 6: Cleanup catalog (partial — reset LSN so CDC starts fresh)
	err = workflow.ExecuteActivity(ctx, activities.CleanupCatalogActivity, &activities.CleanupCatalogInput{
		MirrorName: input.MirrorName,
		FullClean:  false,
	}).Get(ctx, nil)
	if err != nil {
		logger.Warn("failed to cleanup catalog", slog.Any("error", err))
	}

	// Phase 7: Restart CDC from scratch (fresh state, new slot)
	logger.Info("full swap resync complete, restarting CDC")
	return workflow.NewContinueAsNewError(ctx, CDCFlowWorkflow, input.CDCInput, nil)
}

// DropFlowInput is the input for the drop workflow
type DropFlowInput struct {
	MirrorName             string
	IsResync               bool
	Config                 *CDCFlowInput // For resync, the original config
	DropDestinationTables  bool
}

// DropFlowWorkflow handles mirror cleanup and optional resync
func DropFlowWorkflow(ctx workflow.Context, input *DropFlowInput) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("starting drop flow",
		slog.String("mirror", input.MirrorName),
		slog.Bool("resync", input.IsResync))

	activityOpts := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Minute,
		HeartbeatTimeout:    5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    1 * time.Minute,
			MaximumAttempts:    10,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, activityOpts)

	// Step 1: Drop replication slot and publication on source
	err := workflow.ExecuteActivity(ctx, activities.DropSourceReplicationActivity, &activities.DropSourceInput{
		MirrorName: input.MirrorName,
	}).Get(ctx, nil)
	if err != nil {
		logger.Warn("failed to drop source replication (may not exist)", slog.Any("error", err))
	}

	// Step 2: Optionally drop destination tables (for full drop, not resync)
	if input.DropDestinationTables && !input.IsResync {
		err = workflow.ExecuteActivity(ctx, activities.DropDestinationTablesActivity, &activities.DropDestinationInput{
			MirrorName: input.MirrorName,
		}).Get(ctx, nil)
		if err != nil {
			logger.Warn("failed to drop destination tables", slog.Any("error", err))
		}
	}

	// Step 3: Clean up catalog entries
	err = workflow.ExecuteActivity(ctx, activities.CleanupCatalogActivity, &activities.CleanupCatalogInput{
		MirrorName: input.MirrorName,
		FullClean:  !input.IsResync,
	}).Get(ctx, nil)
	if err != nil {
		logger.Warn("failed to cleanup catalog", slog.Any("error", err))
	}

	// If resync, restart the CDC flow
	if input.IsResync && input.Config != nil {
		logger.Info("drop complete, restarting CDC for resync")
		return workflow.NewContinueAsNewError(ctx, CDCFlowWorkflow, input.Config, nil)
	}

	logger.Info("drop flow completed")
	return nil
}
