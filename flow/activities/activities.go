package activities

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.temporal.io/sdk/activity"

	"github.com/bunnydb/bunnydb/flow/connectors/postgres"
	"github.com/bunnydb/bunnydb/flow/model"
	"github.com/bunnydb/bunnydb/flow/shared"
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

	// Get source peer config from catalog
	srcConfig, err := a.getPeerConfig(ctx, input.SourcePeer)
	if err != nil {
		return nil, fmt.Errorf("failed to get source peer config: %w", err)
	}

	// Connect to source
	srcConn, err := postgres.NewPostgresConnector(ctx, srcConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to source: %w", err)
	}
	defer srcConn.Close()

	// Set up replication connection
	if err := srcConn.SetupReplConn(ctx); err != nil {
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
			return nil, fmt.Errorf("failed to get OID for table %s: %w", tableName, err)
		}
		srcTableIDMapping[oid] = tableName
	}

	// Create publication
	publicationName := fmt.Sprintf("bunny_pub_%s", input.MirrorName)
	if err := srcConn.CreatePublication(ctx, publicationName, tables); err != nil {
		return nil, fmt.Errorf("failed to create publication: %w", err)
	}

	// Create replication slot
	slotName := fmt.Sprintf("bunny_slot_%s", input.MirrorName)
	snapshotName, err := srcConn.CreateReplicationSlot(ctx, slotName)
	if err != nil {
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
	logger.Info("starting sync flow", slog.Int64("lastLSN", input.LastLSN))

	// This is a placeholder for the actual CDC sync implementation
	// In a real implementation, this would:
	// 1. Connect to source replication stream
	// 2. Read WAL changes
	// 3. Transform and apply to destination
	// 4. Update checkpoint

	// Heartbeat to keep activity alive
	heartbeat := func(msg string) {
		activity.RecordHeartbeat(ctx, msg)
	}

	// Simulate sync loop
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			heartbeat("syncing...")
			// In real implementation: pull records, sync, update LSN
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

	// If we have a snapshot name, set it
	if input.SnapshotName != "" {
		_, err = srcConn.Conn().Exec(ctx, fmt.Sprintf("SET TRANSACTION SNAPSHOT '%s'", input.SnapshotName))
		if err != nil {
			logger.Warn("failed to set snapshot", slog.Any("error", err))
		}
	}

	// Build query
	srcTable := input.TableMapping.FullSourceName()
	_ = input.TableMapping.FullDestinationName() // dstTable used in COPY TO destination

	// Get columns (excluding any excluded columns)
	columns := "*"
	// In real implementation: filter excluded columns

	query := fmt.Sprintf("SELECT %s FROM %s", columns, srcTable)

	// Copy data using COPY protocol for efficiency
	// This is a simplified version - real implementation would use COPY TO/FROM
	logger.Info("copying data", slog.String("query", query))

	// Placeholder for actual COPY implementation
	activity.RecordHeartbeat(ctx, "copying data...")

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

	// Get mirror state from catalog
	var sourcePeer, slotName, publicationName string
	err := a.CatalogPool.QueryRow(ctx, `
		SELECT m.source_peer_id, ms.slot_name, ms.publication_name
		FROM bunny_internal.mirrors m
		JOIN bunny_internal.mirror_state ms ON m.name = ms.mirror_name
		JOIN bunny_internal.peers p ON m.source_peer_id = p.id
		WHERE m.name = $1
	`, input.MirrorName).Scan(&sourcePeer, &slotName, &publicationName)

	if err != nil {
		return fmt.Errorf("failed to get mirror state: %w", err)
	}

	// Get source config and connect
	srcConfig, err := a.getPeerConfig(ctx, sourcePeer)
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
		}
	}

	// Drop publication
	if publicationName != "" {
		if err := srcConn.DropPublication(ctx, publicationName); err != nil {
			logger.Warn("failed to drop publication", slog.Any("error", err))
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
	var config postgres.PostgresConfig
	var configJSON []byte

	err := a.CatalogPool.QueryRow(ctx, `
		SELECT config FROM bunny_internal.peers WHERE name = $1
	`, peerName).Scan(&configJSON)

	if err != nil {
		return nil, fmt.Errorf("peer not found: %s", peerName)
	}

	// Parse JSON config
	// In real implementation: use json.Unmarshal
	// For now, assume config is stored with direct fields

	return &config, nil
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

// Placeholder implementations
func (a *Activities) TruncateTable(ctx context.Context, input *TruncateTableInput) error {
	return nil
}

func (a *Activities) ExportSnapshot(ctx context.Context, input *ExportSnapshotInput) (string, error) {
	return "", nil
}

func (a *Activities) DropTableForeignKeys(ctx context.Context, input *DropTableFKInput) error {
	return nil
}

func (a *Activities) CreateTableIndexes(ctx context.Context, input *CreateTableIndexesInput) error {
	return nil
}

func (a *Activities) RecreateTableForeignKeys(ctx context.Context, input *RecreateTableFKInput) error {
	return nil
}

func (a *Activities) DropDestinationTables(ctx context.Context, input *DropDestinationInput) error {
	return nil
}

func (a *Activities) GetPartitionInfo(ctx context.Context, input *GetPartitionInfoInput) (*PartitionInfo, error) {
	return nil, nil
}

func (a *Activities) CopyPartition(ctx context.Context, input *CopyPartitionInput) error {
	return nil
}
