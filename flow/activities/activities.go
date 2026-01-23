package activities

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.temporal.io/sdk/activity"

	"github.com/bunnydb/bunnydb/flow/connectors/postgres"
	"github.com/bunnydb/bunnydb/flow/model"
	"github.com/bunnydb/bunnydb/flow/shared"
)

// sanitizeName converts a name to a valid PostgreSQL identifier
// by replacing hyphens and other special characters with underscores
func sanitizeName(name string) string {
	return strings.ReplaceAll(name, "-", "_")
}

// Activity name constants for use in workflows
const (
	SetupMirrorActivity              = "SetupMirror"
	SyncFlowActivity                 = "SyncFlow"
	DropForeignKeysActivity          = "DropForeignKeys"
	RecreateForeignKeysActivity      = "RecreateForeignKeys"
	CreateIndexesActivity            = "CreateIndexes"
	CopyTableActivity                = "CopyTable"
	UpdateTableSyncStatusActivity    = "UpdateTableSyncStatus"
	DropSourceReplicationActivity    = "DropSourceReplication"
	CleanupCatalogActivity           = "CleanupCatalog"
	TruncateTableActivity            = "TruncateTable"
	ExportSnapshotActivity           = "ExportSnapshot"
	DropTableForeignKeysActivity     = "DropTableForeignKeys"
	CreateTableIndexesActivity       = "CreateTableIndexes"
	RecreateTableForeignKeysActivity = "RecreateTableForeignKeys"
	DropDestinationTablesActivity    = "DropDestinationTables"
	GetPartitionInfoActivity         = "GetPartitionInfo"
	CopyPartitionActivity            = "CopyPartition"
	SyncSchemaActivity               = "SyncSchema"
)

// Activities holds the activity implementations
type Activities struct {
	CatalogPool *pgxpool.Pool
	Config      *shared.Config
}

// NewActivities creates a new Activities instance
func NewActivities(catalogPool *pgxpool.Pool, config *shared.Config) *Activities {
	return &Activities{
		CatalogPool: catalogPool,
		Config:      config,
	}
}

// WriteLog writes a log entry to the mirror_logs table
func (a *Activities) WriteLog(ctx context.Context, mirrorName, level, message string, details map[string]interface{}) {
	var detailsJSON []byte
	if details != nil {
		detailsJSON, _ = json.Marshal(details)
	}

	_, err := a.CatalogPool.Exec(ctx, `
		INSERT INTO bunny_stats.mirror_logs (mirror_name, log_level, message, details)
		VALUES ($1, $2, $3, $4)
	`, mirrorName, level, message, detailsJSON)
	if err != nil {
		slog.Error("failed to write mirror log", slog.Any("error", err))
	}
}

// ============================================================================
// Setup Activities
// ============================================================================

// SetupInput is the input for SetupMirror
type SetupInput struct {
	MirrorName           string
	SourcePeer           string
	DestinationPeer      string
	TableMappings        []model.TableMapping
	ReplicateIndexes     bool
	ReplicateForeignKeys bool
}

// SetupOutput is the output of SetupMirror
type SetupOutput struct {
	SlotName              string
	PublicationName       string
	SnapshotName          string
	SrcTableIDNameMapping map[uint32]string
}

// SetupMirror sets up the mirror by creating publication and replication slot
func (a *Activities) SetupMirror(ctx context.Context, input *SetupInput) (*SetupOutput, error) {
	logger := slog.Default().With(slog.String("mirror", input.MirrorName))
	logger.Info("setting up mirror")

	a.WriteLog(ctx, input.MirrorName, "INFO", "Starting mirror setup", map[string]interface{}{
		"source_peer":      input.SourcePeer,
		"destination_peer": input.DestinationPeer,
		"table_count":      len(input.TableMappings),
	})

	// Get source peer config from catalog
	srcConfig, err := a.getPeerConfig(ctx, input.SourcePeer)
	if err != nil {
		a.WriteLog(ctx, input.MirrorName, "ERROR", "Failed to get source peer config", map[string]interface{}{
			"error": err.Error(),
			"peer":  input.SourcePeer,
		})
		return nil, fmt.Errorf("failed to get source peer config: %w", err)
	}

	a.WriteLog(ctx, input.MirrorName, "INFO", "Connecting to source database", map[string]interface{}{
		"host":     srcConfig.Host,
		"port":     srcConfig.Port,
		"database": srcConfig.Database,
	})

	// Connect to source
	srcConn, err := postgres.NewPostgresConnector(ctx, srcConfig)
	if err != nil {
		a.WriteLog(ctx, input.MirrorName, "ERROR", "Failed to connect to source", map[string]interface{}{
			"error": err.Error(),
		})
		return nil, fmt.Errorf("failed to connect to source: %w", err)
	}
	defer srcConn.Close()

	a.WriteLog(ctx, input.MirrorName, "INFO", "Connected to source, setting up replication connection", nil)

	// Set up replication connection
	if err := srcConn.SetupReplConn(ctx); err != nil {
		a.WriteLog(ctx, input.MirrorName, "ERROR", "Failed to setup replication connection", map[string]interface{}{
			"error": err.Error(),
		})
		return nil, fmt.Errorf("failed to setup replication connection: %w", err)
	}

	// Build table list
	var tables []string
	srcTableIDMapping := make(map[uint32]string)
	for _, tm := range input.TableMappings {
		tableName := tm.FullSourceName()
		tables = append(tables, tableName)

		// Get table OID
		oid, err := srcConn.GetTableOID(ctx, tm.SourceSchema, tm.SourceTable)
		if err != nil {
			a.WriteLog(ctx, input.MirrorName, "ERROR", "Failed to get table OID", map[string]interface{}{
				"error": err.Error(),
				"table": tableName,
			})
			return nil, fmt.Errorf("failed to get OID for table %s: %w", tableName, err)
		}
		srcTableIDMapping[oid] = tableName
	}

	// Sanitize mirror name for use in PostgreSQL identifiers (no hyphens allowed)
	safeName := sanitizeName(input.MirrorName)

	a.WriteLog(ctx, input.MirrorName, "INFO", "Creating publication", map[string]interface{}{
		"tables": tables,
	})

	// Create publication
	publicationName := fmt.Sprintf("bunny_pub_%s", safeName)
	if err := srcConn.CreatePublication(ctx, publicationName, tables); err != nil {
		a.WriteLog(ctx, input.MirrorName, "ERROR", "Failed to create publication", map[string]interface{}{
			"error":       err.Error(),
			"publication": publicationName,
		})
		return nil, fmt.Errorf("failed to create publication: %w", err)
	}

	a.WriteLog(ctx, input.MirrorName, "INFO", "Creating replication slot", nil)

	// Create replication slot
	slotName := fmt.Sprintf("bunny_slot_%s", safeName)
	snapshotName, err := srcConn.CreateReplicationSlot(ctx, slotName)
	if err != nil {
		a.WriteLog(ctx, input.MirrorName, "ERROR", "Failed to create replication slot", map[string]interface{}{
			"error": err.Error(),
			"slot":  slotName,
		})
		return nil, fmt.Errorf("failed to create replication slot: %w", err)
	}

	// Store setup info in catalog
	_, err = a.CatalogPool.Exec(ctx, `
		INSERT INTO bunny_internal.mirror_state (mirror_name, slot_name, publication_name, status)
		VALUES ($1, $2, $3, 'SETTING_UP')
		ON CONFLICT (mirror_name) DO UPDATE SET
			slot_name = $2,
			publication_name = $3,
			status = 'SETTING_UP',
			updated_at = NOW()
	`, input.MirrorName, slotName, publicationName)
	if err != nil {
		return nil, fmt.Errorf("failed to store mirror state: %w", err)
	}

	a.WriteLog(ctx, input.MirrorName, "INFO", "Mirror setup complete", map[string]interface{}{
		"slot":        slotName,
		"publication": publicationName,
		"snapshot":    snapshotName,
	})

	logger.Info("mirror setup complete",
		slog.String("slot", slotName),
		slog.String("publication", publicationName))

	return &SetupOutput{
		SlotName:              slotName,
		PublicationName:       publicationName,
		SnapshotName:          snapshotName,
		SrcTableIDNameMapping: srcTableIDMapping,
	}, nil
}

// ============================================================================
// Sync Activities
// ============================================================================

// SyncInput is the input for SyncFlow
type SyncInput struct {
	MirrorName      string
	SourcePeer      string
	DestinationPeer string
	SlotName        string
	PublicationName string
	LastLSN         int64
	BatchSize       uint32
	IdleTimeout     uint64
	TableMappings   []model.TableMapping
}

// SyncOutput is the output of SyncFlow
type SyncOutput struct {
	LastLSN int64
	BatchID int64
}

// SyncFlow performs CDC synchronization
func (a *Activities) SyncFlow(ctx context.Context, input *SyncInput) (*SyncOutput, error) {
	logger := slog.Default().With(slog.String("mirror", input.MirrorName))
	logger.Info("starting sync flow",
		slog.Int64("lastLSN", input.LastLSN),
		slog.String("slot", input.SlotName),
		slog.String("publication", input.PublicationName))

	// Get peer configs
	srcConfig, err := a.getPeerConfig(ctx, input.SourcePeer)
	if err != nil {
		return nil, fmt.Errorf("failed to get source peer config: %w", err)
	}

	dstConfig, err := a.getPeerConfig(ctx, input.DestinationPeer)
	if err != nil {
		return nil, fmt.Errorf("failed to get destination peer config: %w", err)
	}

	// Connect to source for replication
	srcConn, err := postgres.NewPostgresConnector(ctx, srcConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to source: %w", err)
	}
	defer srcConn.Close()

	// Setup replication connection
	if err := srcConn.SetupReplConn(ctx); err != nil {
		return nil, fmt.Errorf("failed to setup replication connection: %w", err)
	}

	// Get PG version for proper protocol handling
	if _, err := srcConn.GetPGVersion(ctx); err != nil {
		logger.Warn("failed to get PG version", slog.Any("error", err))
	}

	// Start replication
	if err := srcConn.StartReplication(ctx, input.SlotName, input.PublicationName, input.LastLSN); err != nil {
		return nil, fmt.Errorf("failed to start replication: %w", err)
	}

	// Connect to destination
	dstConn, err := postgres.NewPostgresConnector(ctx, dstConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to destination: %w", err)
	}
	defer dstConn.Close()

	// Build table to PK columns mapping
	tablePKs := make(map[string][]string)
	for _, tm := range input.TableMappings {
		schema, err := srcConn.GetTableSchema(ctx, tm.SourceSchema, tm.SourceTable)
		if err != nil {
			logger.Warn("failed to get table schema",
				slog.String("table", tm.FullSourceName()),
				slog.Any("error", err))
			continue
		}
		tablePKs[tm.FullSourceName()] = schema.PrimaryKeyColumns
	}

	// Create CDC reader
	cdcReader := postgres.NewCDCReader(srcConn)

	// Heartbeat and sync configuration
	batchSize := 1000
	if input.BatchSize > 0 {
		batchSize = int(input.BatchSize)
	}

	idleTimeout := 60 * time.Second
	if input.IdleTimeout > 0 {
		idleTimeout = time.Duration(input.IdleTimeout) * time.Second
	}

	lastLSN := input.LastLSN
	lastHeartbeat := time.Now()
	recordsProcessed := int64(0)
	batchID := int64(0)
	tableRowCounts := make(map[string]int64)    // Track total rows per table
	tableInsertCounts := make(map[string]int64) // Track inserts per table
	tableUpdateCounts := make(map[string]int64) // Track updates per table

	logger.Info("CDC sync loop started",
		slog.Int("batchSize", batchSize),
		slog.Duration("idleTimeout", idleTimeout))

	// Update status to RUNNING immediately when CDC starts
	_, err = a.CatalogPool.Exec(ctx, `
		UPDATE bunny_internal.mirror_state
		SET status = 'RUNNING',
		    updated_at = NOW()
		WHERE mirror_name = $1
	`, input.MirrorName)
	if err != nil {
		logger.Warn("failed to update mirror status to RUNNING", slog.Any("error", err))
	}

	// Send initial heartbeat immediately
	activity.RecordHeartbeat(ctx, fmt.Sprintf("starting CDC sync: LSN=%d", lastLSN))

	for {
		select {
		case <-ctx.Done():
			logger.Info("sync flow stopped",
				slog.Int64("lastLSN", lastLSN),
				slog.Int64("recordsProcessed", recordsProcessed))
			return &SyncOutput{LastLSN: lastLSN, BatchID: batchID}, ctx.Err()
		default:
		}

		// Send heartbeat every 10 seconds (more frequent to avoid timeout)
		if time.Since(lastHeartbeat) > 10*time.Second {
			activity.RecordHeartbeat(ctx, fmt.Sprintf("syncing: LSN=%d, records=%d", lastLSN, recordsProcessed))
			lastHeartbeat = time.Now()
		}

		// Pull records from replication stream (with short timeout to allow heartbeats)
		records, newLSN, err := cdcReader.PullRecords(ctx, batchSize, 3*time.Second)
		if err != nil {
			if ctx.Err() != nil {
				return &SyncOutput{LastLSN: lastLSN, BatchID: batchID}, ctx.Err()
			}
			errMsg := err.Error()
			// Connection closed errors are fatal (slot dropped or signal-triggered cancellation)
			if strings.Contains(errMsg, "conn closed") || strings.Contains(errMsg, "connection reset") ||
				strings.Contains(errMsg, "use of closed network connection") {
				logger.Info("replication connection closed, stopping sync",
					slog.Int64("lastLSN", lastLSN),
					slog.Int64("recordsProcessed", recordsProcessed))
				return &SyncOutput{LastLSN: lastLSN, BatchID: batchID}, fmt.Errorf("replication connection lost: %w", err)
			}
			logger.Error("failed to pull records", slog.Any("error", err))
			// Send heartbeat on error to stay alive
			activity.RecordHeartbeat(ctx, fmt.Sprintf("error pulling records, retrying: %v", err))
			lastHeartbeat = time.Now()
			// Context-aware sleep before retrying
			select {
			case <-ctx.Done():
				return &SyncOutput{LastLSN: lastLSN, BatchID: batchID}, ctx.Err()
			case <-time.After(1 * time.Second):
			}
			continue
		}

		if len(records) == 0 {
			// No records - still send heartbeat to stay alive
			if time.Since(lastHeartbeat) > 5*time.Second {
				activity.RecordHeartbeat(ctx, fmt.Sprintf("waiting for changes: LSN=%d", lastLSN))
				lastHeartbeat = time.Now()
			}
			continue
		}

		// Apply records to destination
		for _, rec := range records {
			tableKey := fmt.Sprintf("%s.%s", rec.Schema, rec.Table)
			pkCols := tablePKs[tableKey]

			if err := postgres.ApplyRecord(ctx, dstConn, rec, pkCols); err != nil {
				logger.Error("failed to apply record",
					slog.String("operation", rec.Operation),
					slog.String("table", tableKey),
					slog.Any("error", err))
				// Continue with next record - don't fail entire sync
				continue
			}

			recordsProcessed++
			tableRowCounts[tableKey]++

			// Track inserts vs updates
			switch rec.Operation {
			case "INSERT":
				tableInsertCounts[tableKey]++
			case "UPDATE":
				tableUpdateCounts[tableKey]++
			}

			if recordsProcessed%100 == 0 {
				logger.Debug("CDC progress",
					slog.Int64("records", recordsProcessed),
					slog.Int64("lsn", rec.LSN))
			}
		}

		// Update table sync status periodically (every batch)
		if len(records) > 0 {
			for tableName, rowCount := range tableRowCounts {
				insertCount := tableInsertCounts[tableName]
				updateCount := tableUpdateCounts[tableName]
				_, err := a.CatalogPool.Exec(ctx, `
					INSERT INTO bunny_stats.table_sync_status (mirror_name, table_name, status, rows_synced, rows_inserted, rows_updated, last_synced_at)
					VALUES ($1, $2, 'RUNNING', $3, $4, $5, NOW())
					ON CONFLICT (mirror_name, table_name) DO UPDATE SET
						status = 'RUNNING',
						rows_synced = bunny_stats.table_sync_status.rows_synced + $3,
						rows_inserted = COALESCE(bunny_stats.table_sync_status.rows_inserted, 0) + $4,
						rows_updated = COALESCE(bunny_stats.table_sync_status.rows_updated, 0) + $5,
						last_synced_at = NOW(),
						updated_at = NOW()
				`, input.MirrorName, tableName, rowCount, insertCount, updateCount)
				if err != nil {
					logger.Warn("failed to update table sync status", slog.String("table", tableName), slog.Any("error", err))
				}
			}
			// Reset counts after updating
			tableRowCounts = make(map[string]int64)
			tableInsertCounts = make(map[string]int64)
			tableUpdateCounts = make(map[string]int64)
		}

		// Update LSN and batch ID
		if newLSN > lastLSN {
			lastLSN = newLSN
			batchID++
			srcConn.UpdateLastOffset(lastLSN)

			// Update checkpoint and status in mirror_state
			_, err := a.CatalogPool.Exec(ctx, `
				UPDATE bunny_internal.mirror_state
				SET last_lsn = $2,
				    last_sync_batch_id = $3,
				    status = 'RUNNING',
				    updated_at = NOW()
				WHERE mirror_name = $1
			`, input.MirrorName, lastLSN, batchID)
			if err != nil {
				logger.Warn("failed to update mirror checkpoint", slog.Any("error", err))
			}
		}

		// Log batch completion
		if len(records) > 0 {
			logger.Info("batch processed",
				slog.Int("records", len(records)),
				slog.Int64("lastLSN", lastLSN),
				slog.Int64("batchID", batchID),
				slog.Int64("totalProcessed", recordsProcessed))

			// Write to mirror logs
			a.WriteLog(ctx, input.MirrorName, "DEBUG", "CDC batch processed", map[string]interface{}{
				"records":        len(records),
				"lastLSN":        lastLSN,
				"totalProcessed": recordsProcessed,
			})
		}
	}
}

// ============================================================================
// Foreign Key Activities
// ============================================================================

// DropFKInput is the input for DropForeignKeys
type DropFKInput struct {
	MirrorName      string
	DestinationPeer string
	TableMappings   []model.TableMapping
}

// DropForeignKeys drops all foreign keys on destination tables
func (a *Activities) DropForeignKeys(ctx context.Context, input *DropFKInput) error {
	logger := slog.Default().With(slog.String("mirror", input.MirrorName))
	logger.Info("dropping foreign keys on destination")

	// Get destination peer config
	dstConfig, err := a.getPeerConfig(ctx, input.DestinationPeer)
	if err != nil {
		return fmt.Errorf("failed to get destination peer config: %w", err)
	}

	// Connect to destination
	dstConn, err := postgres.NewPostgresConnector(ctx, dstConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to destination: %w", err)
	}
	defer dstConn.Close()

	// Drop FKs for each table
	for _, tm := range input.TableMappings {
		fks, err := dstConn.GetForeignKeys(ctx, tm.DestinationSchema, tm.DestinationTable)
		if err != nil {
			logger.Warn("failed to get FKs for table",
				slog.String("table", tm.FullDestinationName()),
				slog.Any("error", err))
			continue
		}

		for _, fk := range fks {
			if err := dstConn.DropForeignKey(ctx, tm.DestinationSchema, tm.DestinationTable, fk.Name); err != nil {
				logger.Warn("failed to drop FK",
					slog.String("fk", fk.Name),
					slog.Any("error", err))
			}

			// Store dropped FK in catalog for later recreation
			_, err = a.CatalogPool.Exec(ctx, `
				INSERT INTO bunny_internal.fk_definitions
				(mirror_name, source_table, constraint_name, constraint_definition,
				 target_table, on_delete, on_update, is_deferrable, initially_deferred, dropped_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())
				ON CONFLICT (mirror_name, source_table, constraint_name) DO UPDATE SET
					dropped_at = NOW()
			`, input.MirrorName, fk.SourceTable, fk.Name, fk.Definition,
				fk.TargetTable, fk.OnDelete, fk.OnUpdate, fk.IsDeferrable, fk.InitiallyDeferred)
			if err != nil {
				logger.Warn("failed to store FK definition", slog.Any("error", err))
			}
		}
	}

	return nil
}

// RecreateFKInput is the input for RecreateForeignKeys
type RecreateFKInput struct {
	MirrorName      string
	SourcePeer      string
	DestinationPeer string
	TableMappings   []model.TableMapping
	MakeDeferrable  bool
	Validate        bool
}

// RecreateForeignKeys recreates foreign keys on destination
func (a *Activities) RecreateForeignKeys(ctx context.Context, input *RecreateFKInput) error {
	logger := slog.Default().With(slog.String("mirror", input.MirrorName))
	logger.Info("recreating foreign keys on destination")

	// Get peer configs
	srcConfig, err := a.getPeerConfig(ctx, input.SourcePeer)
	if err != nil {
		return fmt.Errorf("failed to get source peer config: %w", err)
	}

	dstConfig, err := a.getPeerConfig(ctx, input.DestinationPeer)
	if err != nil {
		return fmt.Errorf("failed to get destination peer config: %w", err)
	}

	// Connect
	srcConn, err := postgres.NewPostgresConnector(ctx, srcConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to source: %w", err)
	}
	defer srcConn.Close()

	dstConn, err := postgres.NewPostgresConnector(ctx, dstConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to destination: %w", err)
	}
	defer dstConn.Close()

	// Create FK replicator
	fkReplicator := postgres.NewFKReplicator(srcConn, dstConn)

	// Get tables to process
	var tables []string
	for _, tm := range input.TableMappings {
		tables = append(tables, tm.FullDestinationName())
	}

	// Replicate FKs from source
	if err := fkReplicator.ReplicateFKsFromSource(ctx, tables, input.MakeDeferrable); err != nil {
		return fmt.Errorf("failed to replicate FKs: %w", err)
	}

	// Update catalog - mark FKs as recreated
	_, err = a.CatalogPool.Exec(ctx, `
		UPDATE bunny_internal.fk_definitions
		SET recreated_at = NOW()
		WHERE mirror_name = $1 AND dropped_at IS NOT NULL AND recreated_at IS NULL
	`, input.MirrorName)
	if err != nil {
		logger.Warn("failed to update FK recreation status", slog.Any("error", err))
	}

	return nil
}

// ============================================================================
// Index Activities
// ============================================================================

// CreateIndexesInput is the input for CreateIndexes
type CreateIndexesInput struct {
	MirrorName      string
	SourcePeer      string
	DestinationPeer string
	TableMappings   []model.TableMapping
	Concurrent      bool
}

// CreateIndexes creates indexes on destination tables
func (a *Activities) CreateIndexes(ctx context.Context, input *CreateIndexesInput) error {
	logger := slog.Default().With(slog.String("mirror", input.MirrorName))
	logger.Info("creating indexes on destination")

	// Get peer configs
	srcConfig, err := a.getPeerConfig(ctx, input.SourcePeer)
	if err != nil {
		return fmt.Errorf("failed to get source peer config: %w", err)
	}

	dstConfig, err := a.getPeerConfig(ctx, input.DestinationPeer)
	if err != nil {
		return fmt.Errorf("failed to get destination peer config: %w", err)
	}

	// Connect
	srcConn, err := postgres.NewPostgresConnector(ctx, srcConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to source: %w", err)
	}
	defer srcConn.Close()

	dstConn, err := postgres.NewPostgresConnector(ctx, dstConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to destination: %w", err)
	}
	defer dstConn.Close()

	// Replicate indexes for each table
	for _, tm := range input.TableMappings {
		if err := srcConn.ReplicateIndexes(ctx, dstConn, tm.DestinationSchema, tm.DestinationTable, input.Concurrent); err != nil {
			logger.Warn("failed to replicate indexes for table",
				slog.String("table", tm.FullDestinationName()),
				slog.Any("error", err))
		}

		activity.RecordHeartbeat(ctx, fmt.Sprintf("created indexes for %s", tm.FullDestinationName()))
	}

	return nil
}

// ============================================================================
// Copy Activities
// ============================================================================

// CopyTableInput is the input for CopyTable
type CopyTableInput struct {
	MirrorName      string
	SourcePeer      string
	DestinationPeer string
	TableMapping    model.TableMapping
	SnapshotName    string
}

// CopyTable copies a table from source to destination
func (a *Activities) CopyTable(ctx context.Context, input *CopyTableInput) error {
	logger := slog.Default().With(
		slog.String("mirror", input.MirrorName),
		slog.String("table", input.TableMapping.FullSourceName()))
	logger.Info("copying table")

	// Get peer configs
	srcConfig, err := a.getPeerConfig(ctx, input.SourcePeer)
	if err != nil {
		return fmt.Errorf("failed to get source peer config: %w", err)
	}

	dstConfig, err := a.getPeerConfig(ctx, input.DestinationPeer)
	if err != nil {
		return fmt.Errorf("failed to get destination peer config: %w", err)
	}

	// Connect - each copy operation uses its own dedicated connections
	srcConn, err := postgres.NewPostgresConnector(ctx, srcConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to source: %w", err)
	}
	defer srcConn.Close()

	dstConn, err := postgres.NewPostgresConnector(ctx, dstConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to destination: %w", err)
	}
	defer dstConn.Close()

	// Ensure destination schema exists
	if err := dstConn.EnsureSchemaExists(ctx, input.TableMapping.DestinationSchema); err != nil {
		return fmt.Errorf("failed to create destination schema: %w", err)
	}

	// Get source table schema and create destination table if needed
	srcSchema, err := srcConn.GetTableSchema(ctx, input.TableMapping.SourceSchema, input.TableMapping.SourceTable)
	if err != nil {
		return fmt.Errorf("failed to get source table schema: %w", err)
	}

	if err := dstConn.CreateTableFromSchema(ctx, srcSchema, input.TableMapping.DestinationSchema, input.TableMapping.DestinationTable); err != nil {
		return fmt.Errorf("failed to create destination table: %w", err)
	}

	// Truncate destination table before copying (in case it already has data)
	truncateSQL := fmt.Sprintf("TRUNCATE TABLE %s.%s CASCADE",
		input.TableMapping.DestinationSchema, input.TableMapping.DestinationTable)
	if _, err := dstConn.Conn().Exec(ctx, truncateSQL); err != nil {
		logger.Warn("failed to truncate destination table (may not exist)", slog.Any("error", err))
	}

	// Start a REPEATABLE READ transaction on source for snapshot consistency
	// This is REQUIRED before SET TRANSACTION SNAPSHOT can be used
	srcTx, err := srcConn.Conn().Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin source transaction: %w", err)
	}
	defer srcTx.Rollback(ctx)

	// Set isolation level FIRST - must be done before any queries
	_, err = srcTx.Exec(ctx, "SET TRANSACTION ISOLATION LEVEL REPEATABLE READ")
	if err != nil {
		return fmt.Errorf("failed to set isolation level: %w", err)
	}

	// If we have a snapshot name, import it into this transaction
	if input.SnapshotName != "" {
		_, err = srcTx.Exec(ctx, fmt.Sprintf("SET TRANSACTION SNAPSHOT '%s'", input.SnapshotName))
		if err != nil {
			return fmt.Errorf("failed to set snapshot: %w", err)
		}
		logger.Info("snapshot imported", slog.String("snapshot", input.SnapshotName))
	}

	// Build query
	srcTable := input.TableMapping.FullSourceName()
	dstTable := input.TableMapping.FullDestinationName()

	// Get columns (excluding any excluded columns)
	columns := "*"
	// In real implementation: filter excluded columns

	query := fmt.Sprintf("SELECT %s FROM %s", columns, srcTable)

	// Copy data using COPY protocol for efficiency
	logger.Info("copying data", slog.String("query", query))

	// Read from source within the snapshot transaction
	rows, err := srcTx.Query(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to query source: %w", err)
	}
	defer rows.Close()

	// Get column descriptions for building insert
	fieldDescs := rows.FieldDescriptions()
	colNames := make([]string, len(fieldDescs))
	for i, fd := range fieldDescs {
		colNames[i] = string(fd.Name)
	}

	// Build insert statement
	placeholders := make([]string, len(colNames))
	for i := range placeholders {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
	}
	insertSQL := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		dstTable,
		strings.Join(colNames, ", "),
		strings.Join(placeholders, ", "))

	// Copy rows in batches
	batchSize := 1000
	batch := make([][]interface{}, 0, batchSize)

	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return fmt.Errorf("failed to get row values: %w", err)
		}
		batch = append(batch, values)

		if len(batch) >= batchSize {
			if err := a.insertBatch(ctx, dstConn.Conn(), insertSQL, batch); err != nil {
				return fmt.Errorf("failed to insert batch: %w", err)
			}
			batch = batch[:0]
			activity.RecordHeartbeat(ctx, "copying data...")
		}
	}

	// Insert remaining rows
	if len(batch) > 0 {
		if err := a.insertBatch(ctx, dstConn.Conn(), insertSQL, batch); err != nil {
			return fmt.Errorf("failed to insert final batch: %w", err)
		}
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("error reading rows: %w", err)
	}

	logger.Info("table copy completed", slog.String("table", srcTable))
	return nil
}

// insertBatch inserts a batch of rows
func (a *Activities) insertBatch(ctx context.Context, conn *pgx.Conn, insertSQL string, batch [][]interface{}) error {
	for _, values := range batch {
		_, err := conn.Exec(ctx, insertSQL, values...)
		if err != nil {
			return err
		}
	}
	return nil
}

// ============================================================================
// Table Status Activities
// ============================================================================

// UpdateTableStatusInput is the input for UpdateTableSyncStatus
type UpdateTableStatusInput struct {
	MirrorName   string
	TableName    string
	Status       string
	ErrorMessage string
}

// UpdateTableSyncStatus updates the sync status of a table
func (a *Activities) UpdateTableSyncStatus(ctx context.Context, input *UpdateTableStatusInput) error {
	_, err := a.CatalogPool.Exec(ctx, `
		INSERT INTO bunny_stats.table_sync_status (mirror_name, table_name, status, error_message)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (mirror_name, table_name) DO UPDATE SET
			status = $3,
			error_message = $4,
			updated_at = NOW()
	`, input.MirrorName, input.TableName, input.Status, input.ErrorMessage)

	return err
}

// ============================================================================
// Cleanup Activities
// ============================================================================

// DropSourceInput is the input for DropSourceReplication
type DropSourceInput struct {
	MirrorName string
}

// DropSourceReplication drops the replication slot and publication on source
func (a *Activities) DropSourceReplication(ctx context.Context, input *DropSourceInput) error {
	logger := slog.Default().With(slog.String("mirror", input.MirrorName))
	logger.Info("dropping source replication")

	// Get mirror state and source peer from catalog
	var sourcePeerName, slotName, publicationName string
	err := a.CatalogPool.QueryRow(ctx, `
		SELECT p.name, ms.slot_name, ms.publication_name
		FROM bunny_internal.mirrors m
		JOIN bunny_internal.mirror_state ms ON m.name = ms.mirror_name
		JOIN bunny_internal.peers p ON m.source_peer_id = p.id
		WHERE m.name = $1
	`, input.MirrorName).Scan(&sourcePeerName, &slotName, &publicationName)

	if err != nil {
		logger.Warn("failed to get mirror state from mirrors table, trying fallback", slog.Any("error", err))
		// Fallback: try to get slot/publication from mirror_state alone
		err = a.CatalogPool.QueryRow(ctx, `
			SELECT COALESCE(slot_name, ''), COALESCE(publication_name, '')
			FROM bunny_internal.mirror_state
			WHERE mirror_name = $1
		`, input.MirrorName).Scan(&slotName, &publicationName)
		if err != nil {
			return fmt.Errorf("failed to get mirror state: %w", err)
		}
		// Without source peer, we can't drop - just log and continue
		logger.Warn("no source peer found, skipping replication slot/publication cleanup")
		return nil
	}

	// Get source config and connect
	srcConfig, err := a.getPeerConfig(ctx, sourcePeerName)
	if err != nil {
		return fmt.Errorf("failed to get source config: %w", err)
	}

	srcConn, err := postgres.NewPostgresConnector(ctx, srcConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to source: %w", err)
	}
	defer srcConn.Close()

	// Drop slot
	if slotName != "" {
		if err := srcConn.DropReplicationSlot(ctx, slotName); err != nil {
			logger.Warn("failed to drop slot", slog.Any("error", err))
		} else {
			logger.Info("dropped replication slot", slog.String("slot", slotName))
		}
	}

	// Drop publication
	if publicationName != "" {
		if err := srcConn.DropPublication(ctx, publicationName); err != nil {
			logger.Warn("failed to drop publication", slog.Any("error", err))
		} else {
			logger.Info("dropped publication", slog.String("publication", publicationName))
		}
	}

	return nil
}

// CleanupCatalogInput is the input for CleanupCatalog
type CleanupCatalogInput struct {
	MirrorName string
	FullClean  bool
}

// CleanupCatalog cleans up catalog entries for a mirror
func (a *Activities) CleanupCatalog(ctx context.Context, input *CleanupCatalogInput) error {
	logger := slog.Default().With(slog.String("mirror", input.MirrorName))
	logger.Info("cleaning up catalog", slog.Bool("fullClean", input.FullClean))

	if input.FullClean {
		// Delete all entries
		_, err := a.CatalogPool.Exec(ctx, `
			DELETE FROM bunny_internal.mirror_state WHERE mirror_name = $1
		`, input.MirrorName)
		if err != nil {
			return err
		}

		_, err = a.CatalogPool.Exec(ctx, `
			DELETE FROM bunny_stats.table_sync_status WHERE mirror_name = $1
		`, input.MirrorName)
		if err != nil {
			return err
		}

		_, err = a.CatalogPool.Exec(ctx, `
			DELETE FROM bunny_internal.fk_definitions WHERE mirror_name = $1
		`, input.MirrorName)
		if err != nil {
			return err
		}

		_, err = a.CatalogPool.Exec(ctx, `
			DELETE FROM bunny_internal.index_definitions WHERE mirror_name = $1
		`, input.MirrorName)
		if err != nil {
			return err
		}
	} else {
		// Just reset state for resync
		_, err := a.CatalogPool.Exec(ctx, `
			UPDATE bunny_internal.mirror_state SET
				last_lsn = 0,
				last_sync_batch_id = 0,
				status = 'CREATED',
				error_message = NULL,
				error_count = 0,
				updated_at = NOW()
			WHERE mirror_name = $1
		`, input.MirrorName)
		if err != nil {
			return err
		}
	}

	return nil
}

// ============================================================================
// Helper Functions
// ============================================================================

func (a *Activities) getPeerConfig(ctx context.Context, peerName string) (*postgres.PostgresConfig, error) {
	var host, username, database, sslMode string
	var port int
	var password *string

	err := a.CatalogPool.QueryRow(ctx, `
		SELECT host, port, username, password, database, COALESCE(ssl_mode, 'disable')
		FROM bunny_internal.peers WHERE name = $1
	`, peerName).Scan(&host, &port, &username, &password, &database, &sslMode)

	if err != nil {
		return nil, fmt.Errorf("peer not found: %s: %w", peerName, err)
	}

	config := &postgres.PostgresConfig{
		Host:     host,
		Port:     port,
		User:     username,
		Database: database,
		SSLMode:  sslMode,
	}

	if password != nil {
		config.Password = *password
	}

	slog.Info("loaded peer config",
		slog.String("peer", peerName),
		slog.String("host", config.Host),
		slog.Int("port", config.Port),
		slog.String("user", config.User),
		slog.String("database", config.Database),
		slog.String("sslMode", config.SSLMode))

	return config, nil
}

// getMirrorPeers gets the source and destination peer names for a mirror
func (a *Activities) getMirrorPeers(ctx context.Context, mirrorName string) (sourcePeer, destPeer string, err error) {
	err = a.CatalogPool.QueryRow(ctx, `
		SELECT sp.name, dp.name
		FROM bunny_internal.mirrors m
		JOIN bunny_internal.peers sp ON m.source_peer_id = sp.id
		JOIN bunny_internal.peers dp ON m.destination_peer_id = dp.id
		WHERE m.name = $1
	`, mirrorName).Scan(&sourcePeer, &destPeer)
	if err != nil {
		return "", "", fmt.Errorf("failed to get mirror peers: %w", err)
	}
	return sourcePeer, destPeer, nil
}

// Placeholder types for activities that need more implementation

type TruncateTableInput struct {
	MirrorName      string
	DestinationPeer string
	TableName       string
}

type ExportSnapshotInput struct {
	MirrorName string
	SourcePeer string
}

type DropTableFKInput struct {
	MirrorName      string
	DestinationPeer string
	TableName       string
}

type CreateTableIndexesInput struct {
	MirrorName      string
	SourcePeer      string
	DestinationPeer string
	TableMapping    model.TableMapping
	Concurrent      bool
}

type RecreateTableFKInput struct {
	MirrorName      string
	SourcePeer      string
	DestinationPeer string
	TableName       string
	MakeDeferrable  bool
	Validate        bool
}

type DropDestinationInput struct {
	MirrorName string
}

type GetPartitionInfoInput struct {
	MirrorName          string
	SourcePeer          string
	TableMapping        model.TableMapping
	NumRowsPerPartition uint32
}

type PartitionInfo struct {
	PartitionKey  string
	NumPartitions uint32
	MinValue      interface{}
	MaxValue      interface{}
}

type CopyPartitionInput struct {
	MirrorName      string
	SourcePeer      string
	DestinationPeer string
	TableMapping    model.TableMapping
	SnapshotName    string
	PartitionKey    string
	PartitionNum    uint32
	TotalPartitions uint32
	MinValue        interface{}
	MaxValue        interface{}
}

// TruncateTable truncates a table on the destination
func (a *Activities) TruncateTable(ctx context.Context, input *TruncateTableInput) error {
	logger := slog.Default().With(
		slog.String("mirror", input.MirrorName),
		slog.String("table", input.TableName))
	logger.Info("truncating table")

	// Get destination peer config
	dstConfig, err := a.getPeerConfig(ctx, input.DestinationPeer)
	if err != nil {
		return fmt.Errorf("failed to get destination peer config: %w", err)
	}

	// Connect to destination
	dstConn, err := postgres.NewPostgresConnector(ctx, dstConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to destination: %w", err)
	}
	defer dstConn.Close()

	// Truncate the table with CASCADE to handle FK dependencies
	_, err = dstConn.Conn().Exec(ctx, fmt.Sprintf("TRUNCATE TABLE %s CASCADE", input.TableName))
	if err != nil {
		return fmt.Errorf("failed to truncate table %s: %w", input.TableName, err)
	}

	logger.Info("table truncated successfully")
	return nil
}

// ExportSnapshot exports a snapshot for consistent reads
func (a *Activities) ExportSnapshot(ctx context.Context, input *ExportSnapshotInput) (string, error) {
	logger := slog.Default().With(slog.String("mirror", input.MirrorName))
	logger.Info("exporting snapshot")

	// Get source peer config
	srcConfig, err := a.getPeerConfig(ctx, input.SourcePeer)
	if err != nil {
		return "", fmt.Errorf("failed to get source peer config: %w", err)
	}

	// Connect to source
	srcConn, err := postgres.NewPostgresConnector(ctx, srcConfig)
	if err != nil {
		return "", fmt.Errorf("failed to connect to source: %w", err)
	}
	defer srcConn.Close()

	// Start a transaction with REPEATABLE READ isolation and export snapshot
	tx, err := srcConn.Conn().Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Set isolation level
	_, err = tx.Exec(ctx, "SET TRANSACTION ISOLATION LEVEL REPEATABLE READ")
	if err != nil {
		return "", fmt.Errorf("failed to set isolation level: %w", err)
	}

	// Export snapshot
	var snapshotName string
	err = tx.QueryRow(ctx, "SELECT pg_export_snapshot()").Scan(&snapshotName)
	if err != nil {
		return "", fmt.Errorf("failed to export snapshot: %w", err)
	}

	logger.Info("snapshot exported", slog.String("snapshot", snapshotName))
	return snapshotName, nil
}

// DropTableForeignKeys drops foreign keys that reference the given table
func (a *Activities) DropTableForeignKeys(ctx context.Context, input *DropTableFKInput) error {
	logger := slog.Default().With(
		slog.String("mirror", input.MirrorName),
		slog.String("table", input.TableName))
	logger.Info("dropping foreign keys for table")

	// Get destination peer config
	dstConfig, err := a.getPeerConfig(ctx, input.DestinationPeer)
	if err != nil {
		return fmt.Errorf("failed to get destination peer config: %w", err)
	}

	// Connect to destination
	dstConn, err := postgres.NewPostgresConnector(ctx, dstConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to destination: %w", err)
	}
	defer dstConn.Close()

	// Find FKs referencing this table or owned by this table
	rows, err := dstConn.Conn().Query(ctx, `
		SELECT
			tc.table_schema,
			tc.table_name,
			tc.constraint_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.constraint_column_usage ccu
			ON tc.constraint_name = ccu.constraint_name
			AND tc.constraint_schema = ccu.constraint_schema
		WHERE tc.constraint_type = 'FOREIGN KEY'
			AND (ccu.table_schema || '.' || ccu.table_name = $1
				OR tc.table_schema || '.' || tc.table_name = $1)
	`, input.TableName)
	if err != nil {
		return fmt.Errorf("failed to query foreign keys: %w", err)
	}
	defer rows.Close()

	var fksToDrops []struct{ schema, table, constraint string }
	for rows.Next() {
		var fk struct{ schema, table, constraint string }
		if err := rows.Scan(&fk.schema, &fk.table, &fk.constraint); err != nil {
			continue
		}
		fksToDrops = append(fksToDrops, fk)
	}

	// Drop each FK
	for _, fk := range fksToDrops {
		query := fmt.Sprintf("ALTER TABLE %s.%s DROP CONSTRAINT IF EXISTS %s",
			fk.schema, fk.table, fk.constraint)
		_, err = dstConn.Conn().Exec(ctx, query)
		if err != nil {
			logger.Warn("failed to drop FK", slog.String("constraint", fk.constraint), slog.Any("error", err))
		} else {
			logger.Info("dropped FK", slog.String("constraint", fk.constraint))
		}
	}

	return nil
}

// CreateTableIndexes creates indexes for a specific table on the destination
func (a *Activities) CreateTableIndexes(ctx context.Context, input *CreateTableIndexesInput) error {
	logger := slog.Default().With(
		slog.String("mirror", input.MirrorName),
		slog.String("table", input.TableMapping.FullSourceName()))
	logger.Info("creating indexes for table")

	// Get peer configs
	srcConfig, err := a.getPeerConfig(ctx, input.SourcePeer)
	if err != nil {
		return fmt.Errorf("failed to get source peer config: %w", err)
	}

	dstConfig, err := a.getPeerConfig(ctx, input.DestinationPeer)
	if err != nil {
		return fmt.Errorf("failed to get destination peer config: %w", err)
	}

	// Connect
	srcConn, err := postgres.NewPostgresConnector(ctx, srcConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to source: %w", err)
	}
	defer srcConn.Close()

	dstConn, err := postgres.NewPostgresConnector(ctx, dstConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to destination: %w", err)
	}
	defer dstConn.Close()

	// Replicate indexes for the table
	if err := srcConn.ReplicateIndexes(ctx, dstConn,
		input.TableMapping.DestinationSchema,
		input.TableMapping.DestinationTable,
		input.Concurrent); err != nil {
		return fmt.Errorf("failed to replicate indexes: %w", err)
	}

	activity.RecordHeartbeat(ctx, fmt.Sprintf("created indexes for %s", input.TableMapping.FullDestinationName()))
	return nil
}

// RecreateTableForeignKeys recreates foreign keys for a specific table
func (a *Activities) RecreateTableForeignKeys(ctx context.Context, input *RecreateTableFKInput) error {
	logger := slog.Default().With(
		slog.String("mirror", input.MirrorName),
		slog.String("table", input.TableName))
	logger.Info("recreating foreign keys for table")

	// Get peer configs
	srcConfig, err := a.getPeerConfig(ctx, input.SourcePeer)
	if err != nil {
		return fmt.Errorf("failed to get source peer config: %w", err)
	}

	dstConfig, err := a.getPeerConfig(ctx, input.DestinationPeer)
	if err != nil {
		return fmt.Errorf("failed to get destination peer config: %w", err)
	}

	// Connect
	srcConn, err := postgres.NewPostgresConnector(ctx, srcConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to source: %w", err)
	}
	defer srcConn.Close()

	dstConn, err := postgres.NewPostgresConnector(ctx, dstConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to destination: %w", err)
	}
	defer dstConn.Close()

	// Create FK replicator and replicate FKs for this table
	fkReplicator := postgres.NewFKReplicator(srcConn, dstConn)
	if err := fkReplicator.ReplicateFKsFromSource(ctx, []string{input.TableName}, input.MakeDeferrable); err != nil {
		return fmt.Errorf("failed to replicate FKs: %w", err)
	}

	return nil
}

// DropDestinationTables drops all destination tables for a mirror
func (a *Activities) DropDestinationTables(ctx context.Context, input *DropDestinationInput) error {
	logger := slog.Default().With(slog.String("mirror", input.MirrorName))
	logger.Info("dropping destination tables")

	// Get destination peer from mirrors table
	_, destPeer, err := a.getMirrorPeers(ctx, input.MirrorName)
	if err != nil {
		logger.Warn("failed to get destination peer", slog.Any("error", err))
		return nil // Don't fail the whole operation
	}

	dstConfig, err := a.getPeerConfig(ctx, destPeer)
	if err != nil {
		return fmt.Errorf("failed to get destination peer config: %w", err)
	}

	dstConn, err := postgres.NewPostgresConnector(ctx, dstConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to destination: %w", err)
	}
	defer dstConn.Close()

	// Get table mappings from catalog
	rows, err := a.CatalogPool.Query(ctx, `
		SELECT destination_schema, destination_table
		FROM bunny_internal.table_mappings tm
		JOIN bunny_internal.mirrors m ON tm.mirror_id = m.id
		WHERE m.name = $1
	`, input.MirrorName)
	if err != nil {
		logger.Warn("failed to get table mappings", slog.Any("error", err))
		return nil
	}
	defer rows.Close()

	for rows.Next() {
		var schema, table string
		if err := rows.Scan(&schema, &table); err != nil {
			continue
		}
		tableName := fmt.Sprintf("%s.%s", schema, table)
		_, err = dstConn.Conn().Exec(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE", tableName))
		if err != nil {
			logger.Warn("failed to drop table", slog.String("table", tableName), slog.Any("error", err))
		} else {
			logger.Info("dropped table", slog.String("table", tableName))
		}
	}

	return nil
}

// GetPartitionInfo gets partition information for a table
func (a *Activities) GetPartitionInfo(ctx context.Context, input *GetPartitionInfoInput) (*PartitionInfo, error) {
	logger := slog.Default().With(
		slog.String("mirror", input.MirrorName),
		slog.String("table", input.TableMapping.FullSourceName()))
	logger.Info("getting partition info")

	srcConfig, err := a.getPeerConfig(ctx, input.SourcePeer)
	if err != nil {
		return nil, fmt.Errorf("failed to get source peer config: %w", err)
	}

	srcConn, err := postgres.NewPostgresConnector(ctx, srcConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to source: %w", err)
	}
	defer srcConn.Close()

	// Get row count for the table
	var rowCount int64
	tableName := input.TableMapping.FullSourceName()
	err = srcConn.Conn().QueryRow(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s", tableName)).Scan(&rowCount)
	if err != nil {
		return nil, fmt.Errorf("failed to count rows: %w", err)
	}

	// Calculate number of partitions based on rows per partition
	numPartitions := uint32(1)
	if input.NumRowsPerPartition > 0 && rowCount > int64(input.NumRowsPerPartition) {
		numPartitions = uint32((rowCount + int64(input.NumRowsPerPartition) - 1) / int64(input.NumRowsPerPartition))
	}

	return &PartitionInfo{
		PartitionKey:  input.TableMapping.PartitionKey,
		NumPartitions: numPartitions,
	}, nil
}

// CopyPartition copies a partition of a table
func (a *Activities) CopyPartition(ctx context.Context, input *CopyPartitionInput) error {
	logger := slog.Default().With(
		slog.String("mirror", input.MirrorName),
		slog.String("table", input.TableMapping.FullSourceName()),
		slog.Uint64("partition", uint64(input.PartitionNum)))
	logger.Info("copying partition")

	// Get peer configs
	srcConfig, err := a.getPeerConfig(ctx, input.SourcePeer)
	if err != nil {
		return fmt.Errorf("failed to get source peer config: %w", err)
	}

	dstConfig, err := a.getPeerConfig(ctx, input.DestinationPeer)
	if err != nil {
		return fmt.Errorf("failed to get destination peer config: %w", err)
	}

	// Connect - each partition copy uses its own dedicated connections
	srcConn, err := postgres.NewPostgresConnector(ctx, srcConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to source: %w", err)
	}
	defer srcConn.Close()

	dstConn, err := postgres.NewPostgresConnector(ctx, dstConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to destination: %w", err)
	}
	defer dstConn.Close()

	// For first partition, ensure schema and table exist
	if input.PartitionNum == 0 {
		if err := dstConn.EnsureSchemaExists(ctx, input.TableMapping.DestinationSchema); err != nil {
			return fmt.Errorf("failed to create destination schema: %w", err)
		}

		srcSchema, err := srcConn.GetTableSchema(ctx, input.TableMapping.SourceSchema, input.TableMapping.SourceTable)
		if err != nil {
			return fmt.Errorf("failed to get source table schema: %w", err)
		}

		if err := dstConn.CreateTableFromSchema(ctx, srcSchema, input.TableMapping.DestinationSchema, input.TableMapping.DestinationTable); err != nil {
			return fmt.Errorf("failed to create destination table: %w", err)
		}

		// Truncate destination table before copying (in case it already has data)
		truncateSQL := fmt.Sprintf("TRUNCATE TABLE %s.%s CASCADE",
			input.TableMapping.DestinationSchema, input.TableMapping.DestinationTable)
		if _, err := dstConn.Conn().Exec(ctx, truncateSQL); err != nil {
			logger.Warn("failed to truncate destination table (may not exist)", slog.Any("error", err))
		}
	}

	// Start a REPEATABLE READ transaction on source for snapshot consistency
	// This is REQUIRED before SET TRANSACTION SNAPSHOT can be used
	srcTx, err := srcConn.Conn().Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin source transaction: %w", err)
	}
	defer srcTx.Rollback(ctx)

	// Set isolation level FIRST - must be done before any queries
	_, err = srcTx.Exec(ctx, "SET TRANSACTION ISOLATION LEVEL REPEATABLE READ")
	if err != nil {
		return fmt.Errorf("failed to set isolation level: %w", err)
	}

	// If we have a snapshot name, import it into this transaction
	if input.SnapshotName != "" {
		_, err = srcTx.Exec(ctx, fmt.Sprintf("SET TRANSACTION SNAPSHOT '%s'", input.SnapshotName))
		if err != nil {
			return fmt.Errorf("failed to set snapshot: %w", err)
		}
		logger.Info("snapshot imported", slog.String("snapshot", input.SnapshotName))
	}

	// Build query for this partition
	srcTable := input.TableMapping.FullSourceName()
	dstTable := input.TableMapping.FullDestinationName()

	query := fmt.Sprintf("SELECT * FROM %s", srcTable)
	if input.PartitionKey != "" && input.TotalPartitions > 1 {
		// Use modulo-based partitioning
		query = fmt.Sprintf("SELECT * FROM %s WHERE MOD(HASHTEXT(%s::text), %d) = %d",
			srcTable, input.PartitionKey, input.TotalPartitions, input.PartitionNum)
	}

	// Read from source within the snapshot transaction
	rows, err := srcTx.Query(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to query source: %w", err)
	}
	defer rows.Close()

	// Get column descriptions for building insert
	fieldDescs := rows.FieldDescriptions()
	colNames := make([]string, len(fieldDescs))
	for i, fd := range fieldDescs {
		colNames[i] = string(fd.Name)
	}

	// Build insert statement
	placeholders := make([]string, len(colNames))
	for i := range placeholders {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
	}
	insertSQL := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		dstTable,
		strings.Join(colNames, ", "),
		strings.Join(placeholders, ", "))

	// Copy rows in batches
	batchSize := 1000
	batch := make([][]interface{}, 0, batchSize)
	rowCount := 0

	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return fmt.Errorf("failed to get row values: %w", err)
		}
		batch = append(batch, values)
		rowCount++

		if len(batch) >= batchSize {
			if err := a.insertBatch(ctx, dstConn.Conn(), insertSQL, batch); err != nil {
				return fmt.Errorf("failed to insert batch: %w", err)
			}
			batch = batch[:0]
			activity.RecordHeartbeat(ctx, fmt.Sprintf("partition %d/%d: copied %d rows",
				input.PartitionNum+1, input.TotalPartitions, rowCount))
		}
	}

	// Insert remaining rows
	if len(batch) > 0 {
		if err := a.insertBatch(ctx, dstConn.Conn(), insertSQL, batch); err != nil {
			return fmt.Errorf("failed to insert final batch: %w", err)
		}
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("error reading rows: %w", err)
	}

	logger.Info("partition copy completed",
		slog.Int("partition", int(input.PartitionNum)+1),
		slog.Int("totalPartitions", int(input.TotalPartitions)),
		slog.Int("rowCount", rowCount))
	return nil
}

// ============================================================================
// Schema Sync Activity
// ============================================================================

// SyncSchemaInput is the input for the SyncSchema activity
type SyncSchemaInput struct {
	MirrorName      string
	SourcePeer      string
	DestinationPeer string
	TableMappings   []model.TableMapping
	ReplicateIndexes bool
}

// SyncSchemaOutput is the output of the SyncSchema activity
type SyncSchemaOutput struct {
	TablesModified int
	ColumnsAdded   int
	IndexesAdded   int
}

// SyncSchema compares source and destination schemas and applies differences
func (a *Activities) SyncSchema(ctx context.Context, input *SyncSchemaInput) (*SyncSchemaOutput, error) {
	logger := slog.Default().With(slog.String("mirror", input.MirrorName))
	logger.Info("starting schema sync", slog.Int("tables", len(input.TableMappings)))

	a.WriteLog(ctx, input.MirrorName, "INFO", "Starting schema sync", map[string]interface{}{
		"table_count": len(input.TableMappings),
	})

	srcConfig, err := a.getPeerConfig(ctx, input.SourcePeer)
	if err != nil {
		return nil, fmt.Errorf("failed to get source peer config: %w", err)
	}

	dstConfig, err := a.getPeerConfig(ctx, input.DestinationPeer)
	if err != nil {
		return nil, fmt.Errorf("failed to get destination peer config: %w", err)
	}

	srcConn, err := postgres.NewPostgresConnector(ctx, srcConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to source: %w", err)
	}
	defer srcConn.Close()

	dstConn, err := postgres.NewPostgresConnector(ctx, dstConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to destination: %w", err)
	}
	defer dstConn.Close()

	output := &SyncSchemaOutput{}

	for _, tm := range input.TableMappings {
		activity.RecordHeartbeat(ctx, fmt.Sprintf("syncing schema for %s", tm.FullSourceName()))

		// Get source schema
		srcSchema, err := srcConn.GetTableSchema(ctx, tm.SourceSchema, tm.SourceTable)
		if err != nil {
			a.WriteLog(ctx, input.MirrorName, "ERROR", "Failed to get source schema", map[string]interface{}{
				"table": tm.FullSourceName(),
				"error": err.Error(),
			})
			return nil, fmt.Errorf("failed to get source schema for %s: %w", tm.FullSourceName(), err)
		}

		// Get destination schema
		dstSchema, err := dstConn.GetTableSchema(ctx, tm.DestinationSchema, tm.DestinationTable)
		if err != nil {
			a.WriteLog(ctx, input.MirrorName, "ERROR", "Failed to get destination schema", map[string]interface{}{
				"table": tm.FullDestinationName(),
				"error": err.Error(),
			})
			return nil, fmt.Errorf("failed to get destination schema for %s: %w", tm.FullDestinationName(), err)
		}

		// Compare schemas
		delta := postgres.CompareSchemas(srcSchema, dstSchema)

		// Compare indexes if enabled
		if input.ReplicateIndexes {
			srcIndexes, err := srcConn.GetIndexes(ctx, tm.SourceSchema, tm.SourceTable)
			if err != nil {
				logger.Warn("failed to get source indexes", slog.Any("error", err), slog.String("table", tm.FullSourceName()))
			} else {
				dstIndexes, err := dstConn.GetIndexes(ctx, tm.DestinationSchema, tm.DestinationTable)
				if err != nil {
					logger.Warn("failed to get destination indexes", slog.Any("error", err), slog.String("table", tm.FullDestinationName()))
				} else {
					added, _ := postgres.CompareIndexes(srcIndexes, dstIndexes)
					for _, idx := range added {
						delta.AddedIndexes = append(delta.AddedIndexes, postgres.IndexDefinition{
							Name:       idx.Name,
							Definition: idx.Definition,
						})
					}
				}
			}
		}

		if !delta.HasChanges() {
			logger.Info("no schema changes for table", slog.String("table", tm.FullSourceName()))
			continue
		}

		// Apply changes
		output.TablesModified++
		output.ColumnsAdded += len(delta.AddedColumns)
		output.IndexesAdded += len(delta.AddedIndexes)

		a.WriteLog(ctx, input.MirrorName, "INFO", "Applying schema changes", map[string]interface{}{
			"table":           tm.FullSourceName(),
			"columns_added":   len(delta.AddedColumns),
			"columns_dropped": len(delta.DroppedColumns),
			"indexes_added":   len(delta.AddedIndexes),
			"type_changes":    len(delta.TypeChanges),
		})

		if err := dstConn.ApplySchemaDelta(ctx, delta); err != nil {
			a.WriteLog(ctx, input.MirrorName, "ERROR", "Failed to apply schema changes", map[string]interface{}{
				"table": tm.FullSourceName(),
				"error": err.Error(),
			})
			return nil, fmt.Errorf("failed to apply schema delta for %s: %w", tm.FullSourceName(), err)
		}

		logger.Info("schema sync applied",
			slog.String("table", tm.FullSourceName()),
			slog.Int("columnsAdded", len(delta.AddedColumns)),
			slog.Int("indexesAdded", len(delta.AddedIndexes)))
	}

	a.WriteLog(ctx, input.MirrorName, "INFO", "Schema sync completed", map[string]interface{}{
		"tables_modified": output.TablesModified,
		"columns_added":   output.ColumnsAdded,
		"indexes_added":   output.IndexesAdded,
	})

	return output, nil
}
