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
		err := workflow.ExecuteActivity(dropFKCtx, activities.DropForeignKeysActivity, &activities.DropFKInput{
			MirrorName:      input.MirrorName,
			DestinationPeer: input.DestinationPeer,
			TableMappings:   input.TableMappings,
		}).Get(dropFKCtx, nil)

		if err != nil {
			return fmt.Errorf("failed to drop foreign keys: %w", err)
		}
	}

	// Step 2: Clone tables in parallel using Temporal's workflow primitives
	logger.Info("starting parallel table cloning",
		slog.Int("parallelism", int(input.NumTablesInParallel)))

	// Execute child workflows for all tables and collect futures
	var childFutures []workflow.ChildWorkflowFuture
	for _, tm := range input.TableMappings {
		mapping := tm // Capture for closure
		tableName := mapping.FullSourceName()

		childOpts := workflow.ChildWorkflowOptions{
			WorkflowID: fmt.Sprintf("clone-%s-%s", input.MirrorName, tableName),
			RetryPolicy: &temporal.RetryPolicy{
				MaximumAttempts: 3,
			},
		}
		childCtx := workflow.WithChildOptions(ctx, childOpts)

		logger.Info("starting table clone", slog.String("table", tableName))
		future := workflow.ExecuteChildWorkflow(childCtx, CloneTableWorkflow, &CloneTableInput{
			MirrorName:          input.MirrorName,
			SourcePeer:          input.SourcePeer,
			DestinationPeer:     input.DestinationPeer,
			TableMapping:        mapping,
			SnapshotName:        input.SnapshotName,
			NumRowsPerPartition: input.NumRowsPerPartition,
			MaxParallelWorkers:  input.MaxParallelWorkers,
		})
		childFutures = append(childFutures, future)
	}

	// Wait for all child workflows to complete
	var firstErr error
	for i, future := range childFutures {
		tableName := input.TableMappings[i].FullSourceName()
		err := future.Get(ctx, nil)
		if err != nil {
			logger.Error("table clone error", slog.String("table", tableName), slog.Any("error", err))
			if firstErr == nil {
				firstErr = fmt.Errorf("failed to clone table %s: %w", tableName, err)
			}
		} else {
			logger.Info("table clone completed", slog.String("table", tableName))
		}
	}

	if firstErr != nil {
		return firstErr
	}

	// Step 3: Create indexes on destination
	if input.ReplicateIndexes {
		logger.Info("creating indexes on destination")

		createIdxCtx := workflow.WithActivityOptions(ctx, activityOpts)
		err := workflow.ExecuteActivity(createIdxCtx, activities.CreateIndexesActivity, &activities.CreateIndexesInput{
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
		err := workflow.ExecuteActivity(createFKCtx, activities.RecreateForeignKeysActivity, &activities.RecreateFKInput{
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
	err := workflow.ExecuteActivity(ctx, activities.GetPartitionInfoActivity, &activities.GetPartitionInfoInput{
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

		err := workflow.ExecuteActivity(ctx, activities.CopyTableActivity, &activities.CopyTableInput{
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
		// Copy partitions in parallel using Temporal futures
		logger.Info("copying table with partitioning",
			slog.String("table", tableName),
			slog.Int("partitions", int(partitions.NumPartitions)),
			slog.Int("workers", int(input.MaxParallelWorkers)))

		// Execute all partition copies and collect futures
		var partitionFutures []workflow.Future
		for i := uint32(0); i < partitions.NumPartitions; i++ {
			future := workflow.ExecuteActivity(ctx, activities.CopyPartitionActivity, &activities.CopyPartitionInput{
				MirrorName:      input.MirrorName,
				SourcePeer:      input.SourcePeer,
				DestinationPeer: input.DestinationPeer,
				TableMapping:    input.TableMapping,
				SnapshotName:    input.SnapshotName,
				PartitionKey:    partitions.PartitionKey,
				PartitionNum:    i,
				TotalPartitions: partitions.NumPartitions,
				MinValue:        partitions.MinValue,
				MaxValue:        partitions.MaxValue,
			})
			partitionFutures = append(partitionFutures, future)
		}

		// Wait for all partitions to complete
		for i, future := range partitionFutures {
			if err := future.Get(ctx, nil); err != nil {
				return fmt.Errorf("failed to copy partition %d: %w", i, err)
			}
		}
	}

	logger.Info("table clone completed", slog.String("table", tableName))
	return nil
}
