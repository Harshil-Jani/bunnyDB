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

	// Step 1: Mark table as resyncing in catalog
	err := workflow.ExecuteActivity(ctx, activities.UpdateTableSyncStatus, &activities.UpdateTableStatusInput{
		MirrorName: input.MirrorName,
		TableName:  input.TableName,
		Status:     "RESYNCING",
	}).Get(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to update table status: %w", err)
	}

	// Step 2: Drop FKs referencing this table on destination
	logger.Info("dropping foreign keys for table", slog.String("table", input.TableName))
	err = workflow.ExecuteActivity(ctx, activities.DropTableForeignKeys, &activities.DropTableFKInput{
		MirrorName:      input.MirrorName,
		DestinationPeer: input.DestinationPeer,
		TableName:       tableMapping.FullDestinationName(),
	}).Get(ctx, nil)
	if err != nil {
		logger.Warn("failed to drop FKs for table (may not exist)", slog.Any("error", err))
	}

	// Step 3: Truncate destination table
	logger.Info("truncating destination table", slog.String("table", tableMapping.FullDestinationName()))
	err = workflow.ExecuteActivity(ctx, activities.TruncateTable, &activities.TruncateTableInput{
		MirrorName:      input.MirrorName,
		DestinationPeer: input.DestinationPeer,
		TableName:       tableMapping.FullDestinationName(),
	}).Get(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to truncate table: %w", err)
	}

	// Step 4: Get current snapshot from source
	var snapshotName string
	err = workflow.ExecuteActivity(ctx, activities.ExportSnapshot, &activities.ExportSnapshotInput{
		MirrorName: input.MirrorName,
		SourcePeer: input.SourcePeer,
	}).Get(ctx, &snapshotName)
	if err != nil {
		// If we can't get a snapshot, copy without one (less consistent but works)
		logger.Warn("failed to get snapshot, copying without snapshot isolation", slog.Any("error", err))
		snapshotName = ""
	}

	// Step 5: Copy table data
	logger.Info("copying table data", slog.String("table", input.TableName))
	err = workflow.ExecuteActivity(ctx, activities.CopyTable, &activities.CopyTableInput{
		MirrorName:      input.MirrorName,
		SourcePeer:      input.SourcePeer,
		DestinationPeer: input.DestinationPeer,
		TableMapping:    *tableMapping,
		SnapshotName:    snapshotName,
	}).Get(ctx, nil)
	if err != nil {
		// Mark as error and continue with CDC
		_ = workflow.ExecuteActivity(ctx, activities.UpdateTableSyncStatus, &activities.UpdateTableStatusInput{
			MirrorName:   input.MirrorName,
			TableName:    input.TableName,
			Status:       "ERROR",
			ErrorMessage: err.Error(),
		}).Get(ctx, nil)
		return fmt.Errorf("failed to copy table: %w", err)
	}

	// Step 6: Recreate indexes for this table
	logger.Info("recreating indexes for table", slog.String("table", input.TableName))
	err = workflow.ExecuteActivity(ctx, activities.CreateTableIndexes, &activities.CreateTableIndexesInput{
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
	err = workflow.ExecuteActivity(ctx, activities.RecreateTableForeignKeys, &activities.RecreateTableFKInput{
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
	err = workflow.ExecuteActivity(ctx, activities.UpdateTableSyncStatus, &activities.UpdateTableStatusInput{
		MirrorName: input.MirrorName,
		TableName:  input.TableName,
		Status:     "SYNCED",
	}).Get(ctx, nil)
	if err != nil {
		logger.Warn("failed to update table status", slog.Any("error", err))
	}

	// Step 9: Resume CDC flow
	logger.Info("table resync completed, resuming CDC",
		slog.String("mirror", input.MirrorName),
		slog.String("table", input.TableName))

	// Clear the resync state and continue as new with CDC
	input.CDCState.ActiveSignal = model.NoopSignal
	input.CDCState.ResyncTableName = ""

	return workflow.NewContinueAsNewError(ctx, CDCFlowWorkflow, input.CDCInput, input.CDCState)
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
	err := workflow.ExecuteActivity(ctx, activities.DropSourceReplication, &activities.DropSourceInput{
		MirrorName: input.MirrorName,
	}).Get(ctx, nil)
	if err != nil {
		logger.Warn("failed to drop source replication (may not exist)", slog.Any("error", err))
	}

	// Step 2: Optionally drop destination tables (for full drop, not resync)
	if input.DropDestinationTables && !input.IsResync {
		err = workflow.ExecuteActivity(ctx, activities.DropDestinationTables, &activities.DropDestinationInput{
			MirrorName: input.MirrorName,
		}).Get(ctx, nil)
		if err != nil {
			logger.Warn("failed to drop destination tables", slog.Any("error", err))
		}
	}

	// Step 3: Clean up catalog entries
	err = workflow.ExecuteActivity(ctx, activities.CleanupCatalog, &activities.CleanupCatalogInput{
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
