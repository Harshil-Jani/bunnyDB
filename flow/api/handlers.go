package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.temporal.io/sdk/client"

	"github.com/bunnydb/bunnydb/flow/model"
	"github.com/bunnydb/bunnydb/flow/shared"
	"github.com/bunnydb/bunnydb/flow/workflows"
)

// Handler holds the API handlers
type Handler struct {
	TemporalClient client.Client
	CatalogPool    *pgxpool.Pool
	Config         *shared.Config
}

// NewHandler creates a new API handler
func NewHandler(temporalClient client.Client, catalogPool *pgxpool.Pool, config *shared.Config) *Handler {
	return &Handler{
		TemporalClient: temporalClient,
		CatalogPool:    catalogPool,
		Config:         config,
	}
}

// ============================================================================
// Request/Response Types
// ============================================================================

// CreateMirrorRequest is the request to create a mirror
type CreateMirrorRequest struct {
	Name            string              `json:"name"`
	SourcePeer      string              `json:"source_peer"`
	DestinationPeer string              `json:"destination_peer"`
	TableMappings   []TableMappingInput `json:"table_mappings"`

	DoInitialSnapshot           bool   `json:"do_initial_snapshot"`
	MaxBatchSize                uint32 `json:"max_batch_size"`
	IdleTimeoutSeconds          uint64 `json:"idle_timeout_seconds"`
	SnapshotNumRowsPerPartition uint32 `json:"snapshot_num_rows_per_partition"`
	SnapshotMaxParallelWorkers  uint32 `json:"snapshot_max_parallel_workers"`
	SnapshotNumTablesInParallel uint32 `json:"snapshot_num_tables_in_parallel"`

	ReplicateIndexes     bool `json:"replicate_indexes"`
	ReplicateForeignKeys bool `json:"replicate_foreign_keys"`
}

// TableMappingInput is the input for table mapping
type TableMappingInput struct {
	SourceSchema      string   `json:"source_schema"`
	SourceTable       string   `json:"source_table"`
	DestinationSchema string   `json:"destination_schema"`
	DestinationTable  string   `json:"destination_table"`
	PartitionKey      string   `json:"partition_key,omitempty"`
	ExcludeColumns    []string `json:"exclude_columns,omitempty"`
}

// MirrorResponse is the response for mirror operations
type MirrorResponse struct {
	Name       string `json:"name"`
	WorkflowID string `json:"workflow_id"`
	Status     string `json:"status"`
	Message    string `json:"message,omitempty"`
}

// MirrorStatusResponse is the response for mirror status
type MirrorStatusResponse struct {
	Name            string                `json:"name"`
	Status          string                `json:"status"`
	SlotName        string                `json:"slot_name"`
	PublicationName string                `json:"publication_name"`
	LastLSN         int64                 `json:"last_lsn"`
	LastSyncBatchID int64                 `json:"last_sync_batch_id"`
	ErrorMessage    string                `json:"error_message,omitempty"`
	ErrorCount      int                   `json:"error_count"`
	Tables          []TableStatusResponse `json:"tables,omitempty"`
}

// TableStatusResponse is the status of a single table
type TableStatusResponse struct {
	TableName    string     `json:"table_name"`
	Status       string     `json:"status"`
	RowsSynced   int64      `json:"rows_synced"`
	LastSyncedAt *time.Time `json:"last_synced_at,omitempty"`
}

// ResyncRequest is the request to resync
type ResyncRequest struct {
	TableName string `json:"table_name,omitempty"` // Optional for table-level resync
}

// CreatePeerRequest is the request to create a peer
type CreatePeerRequest struct {
	Name     string `json:"name"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	Database string `json:"database"`
	SSLMode  string `json:"ssl_mode,omitempty"`
}

// PeerResponse is the response for peer operations
type PeerResponse struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Database string `json:"database"`
	SSLMode  string `json:"ssl_mode"`
}

// RetryRequest is the request to retry
type RetryRequest struct {
	SkipBackoff bool `json:"skip_backoff"`
}

// ============================================================================
// HTTP Handlers
// ============================================================================

// CreateMirror creates a new mirror
func (h *Handler) CreateMirror(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req CreateMirrorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate request
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "mirror name is required")
		return
	}

	// Set defaults
	if req.MaxBatchSize == 0 {
		req.MaxBatchSize = 1000
	}
	if req.IdleTimeoutSeconds == 0 {
		req.IdleTimeoutSeconds = 60
	}

	// Convert table mappings
	var tableMappings []model.TableMapping
	for _, tm := range req.TableMappings {
		tableMappings = append(tableMappings, model.TableMapping{
			SourceSchema:      tm.SourceSchema,
			SourceTable:       tm.SourceTable,
			DestinationSchema: tm.DestinationSchema,
			DestinationTable:  tm.DestinationTable,
			PartitionKey:      tm.PartitionKey,
			ExcludeColumns:    tm.ExcludeColumns,
		})
	}

	// Insert mirror into mirror_state immediately so it shows up in the list
	_, err := h.CatalogPool.Exec(ctx, `
		INSERT INTO bunny_internal.mirror_state (mirror_name, status)
		VALUES ($1, 'CREATED')
		ON CONFLICT (mirror_name) DO UPDATE SET
			status = 'CREATED',
			updated_at = NOW()
	`, req.Name)
	if err != nil {
		slog.Error("failed to create mirror state", slog.Any("error", err))
		writeError(w, http.StatusInternalServerError, "failed to create mirror")
		return
	}

	// Start the CDC workflow
	workflowID := fmt.Sprintf("cdc-%s", req.Name)
	workflowOptions := client.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: h.Config.WorkerTaskQueue,
	}

	input := &workflows.CDCFlowInput{
		MirrorName:                    req.Name,
		SourcePeer:                    req.SourcePeer,
		DestinationPeer:               req.DestinationPeer,
		TableMappings:                 tableMappings,
		DoInitialSnapshot:             req.DoInitialSnapshot,
		MaxBatchSize:                  req.MaxBatchSize,
		IdleTimeoutSeconds:            req.IdleTimeoutSeconds,
		SnapshotNumRowsPerPartition:   req.SnapshotNumRowsPerPartition,
		SnapshotMaxParallelWorkers:    req.SnapshotMaxParallelWorkers,
		SnapshotNumTablesInParallel:   req.SnapshotNumTablesInParallel,
		ReplicateIndexes:              req.ReplicateIndexes,
		ReplicateForeignKeys:          req.ReplicateForeignKeys,
	}

	we, err := h.TemporalClient.ExecuteWorkflow(ctx, workflowOptions, workflows.CDCFlowWorkflow, input, nil)
	if err != nil {
		slog.Error("failed to start workflow", slog.Any("error", err))
		// Update status to FAILED if workflow start fails
		h.CatalogPool.Exec(ctx, `
			UPDATE bunny_internal.mirror_state SET status = 'FAILED', error_message = $2, updated_at = NOW()
			WHERE mirror_name = $1
		`, req.Name, err.Error())
		writeError(w, http.StatusInternalServerError, "failed to create mirror")
		return
	}

	slog.Info("created mirror",
		slog.String("mirror", req.Name),
		slog.String("workflowID", we.GetID()))

	writeJSON(w, http.StatusCreated, MirrorResponse{
		Name:       req.Name,
		WorkflowID: we.GetID(),
		Status:     "CREATED",
	})
}

// GetMirrorStatus gets the status of a mirror
func (h *Handler) GetMirrorStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	mirrorName := r.PathValue("name")

	if mirrorName == "" {
		writeError(w, http.StatusBadRequest, "mirror name is required")
		return
	}

	workflowID := fmt.Sprintf("cdc-%s", mirrorName)

	// Query workflow state
	resp, err := h.TemporalClient.QueryWorkflow(ctx, workflowID, "", workflows.QueryFlowState)
	if err != nil {
		slog.Error("failed to query workflow", slog.Any("error", err))
		writeError(w, http.StatusNotFound, "mirror not found")
		return
	}

	var state model.CDCFlowState
	if err := resp.Get(&state); err != nil {
		slog.Error("failed to decode workflow state", slog.Any("error", err))
		writeError(w, http.StatusInternalServerError, "failed to get mirror status")
		return
	}

	// Get table statuses from catalog
	var tables []TableStatusResponse
	rows, err := h.CatalogPool.Query(ctx, `
		SELECT table_name, status, rows_synced, last_synced_at
		FROM bunny_stats.table_sync_status
		WHERE mirror_name = $1
	`, mirrorName)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var ts TableStatusResponse
			rows.Scan(&ts.TableName, &ts.Status, &ts.RowsSynced, &ts.LastSyncedAt)
			tables = append(tables, ts)
		}
	}

	writeJSON(w, http.StatusOK, MirrorStatusResponse{
		Name:            mirrorName,
		Status:          string(state.Status),
		SlotName:        state.SlotName,
		PublicationName: state.PublicationName,
		LastLSN:         state.LastLSN,
		LastSyncBatchID: state.LastSyncBatchID,
		ErrorMessage:    state.ErrorMessage,
		ErrorCount:      state.ErrorCount,
		Tables:          tables,
	})
}

// PauseMirror pauses a mirror
func (h *Handler) PauseMirror(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	mirrorName := r.PathValue("name")

	if mirrorName == "" {
		writeError(w, http.StatusBadRequest, "mirror name is required")
		return
	}

	workflowID := fmt.Sprintf("cdc-%s", mirrorName)

	err := h.TemporalClient.SignalWorkflow(ctx, workflowID, "", workflows.SignalPause, model.SignalPayload{
		Signal: model.PauseSignal,
	})
	if err != nil {
		slog.Error("failed to signal workflow", slog.Any("error", err))
		writeError(w, http.StatusInternalServerError, "failed to pause mirror")
		return
	}

	writeJSON(w, http.StatusOK, MirrorResponse{
		Name:    mirrorName,
		Status:  "PAUSING",
		Message: "pause signal sent",
	})
}

// ResumeMirror resumes a paused mirror
func (h *Handler) ResumeMirror(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	mirrorName := r.PathValue("name")

	if mirrorName == "" {
		writeError(w, http.StatusBadRequest, "mirror name is required")
		return
	}

	workflowID := fmt.Sprintf("cdc-%s", mirrorName)

	err := h.TemporalClient.SignalWorkflow(ctx, workflowID, "", workflows.SignalResume, model.SignalPayload{
		Signal: model.ResumeSignal,
	})
	if err != nil {
		slog.Error("failed to signal workflow", slog.Any("error", err))
		writeError(w, http.StatusInternalServerError, "failed to resume mirror")
		return
	}

	writeJSON(w, http.StatusOK, MirrorResponse{
		Name:    mirrorName,
		Status:  "RESUMING",
		Message: "resume signal sent",
	})
}

// DeleteMirror deletes/terminates a mirror
func (h *Handler) DeleteMirror(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	mirrorName := r.PathValue("name")

	if mirrorName == "" {
		writeError(w, http.StatusBadRequest, "mirror name is required")
		return
	}

	workflowID := fmt.Sprintf("cdc-%s", mirrorName)

	err := h.TemporalClient.SignalWorkflow(ctx, workflowID, "", workflows.SignalTerminate, model.SignalPayload{
		Signal: model.TerminateSignal,
	})
	if err != nil {
		slog.Error("failed to signal workflow", slog.Any("error", err))
		writeError(w, http.StatusInternalServerError, "failed to terminate mirror")
		return
	}

	writeJSON(w, http.StatusOK, MirrorResponse{
		Name:    mirrorName,
		Status:  "TERMINATING",
		Message: "terminate signal sent",
	})
}

// ResyncMirror triggers a full resync or table-level resync
func (h *Handler) ResyncMirror(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	mirrorName := r.PathValue("name")
	tableName := r.PathValue("table") // Optional

	if mirrorName == "" {
		writeError(w, http.StatusBadRequest, "mirror name is required")
		return
	}

	workflowID := fmt.Sprintf("cdc-%s", mirrorName)

	var signalName string
	var payload model.SignalPayload

	if tableName != "" {
		// Table-level resync
		signalName = workflows.SignalResyncTable
		payload = model.SignalPayload{
			Signal:    model.ResyncTableSignal,
			TableName: tableName,
		}
		slog.Info("triggering table-level resync",
			slog.String("mirror", mirrorName),
			slog.String("table", tableName))
	} else {
		// Full resync
		signalName = workflows.SignalResync
		payload = model.SignalPayload{
			Signal: model.ResyncSignal,
		}
		slog.Info("triggering full resync", slog.String("mirror", mirrorName))
	}

	err := h.TemporalClient.SignalWorkflow(ctx, workflowID, "", signalName, payload)
	if err != nil {
		slog.Error("failed to signal workflow", slog.Any("error", err))
		writeError(w, http.StatusInternalServerError, "failed to trigger resync")
		return
	}

	message := "resync signal sent"
	if tableName != "" {
		message = fmt.Sprintf("table resync signal sent for %s", tableName)
	}

	writeJSON(w, http.StatusOK, MirrorResponse{
		Name:    mirrorName,
		Status:  "RESYNCING",
		Message: message,
	})
}

// RetryMirror triggers an immediate retry, bypassing Temporal's backoff
func (h *Handler) RetryMirror(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	mirrorName := r.PathValue("name")

	if mirrorName == "" {
		writeError(w, http.StatusBadRequest, "mirror name is required")
		return
	}

	workflowID := fmt.Sprintf("cdc-%s", mirrorName)

	err := h.TemporalClient.SignalWorkflow(ctx, workflowID, "", workflows.SignalRetryNow, model.SignalPayload{
		Signal: model.RetryNowSignal,
		Options: map[string]string{
			"skip_backoff": "true",
		},
	})
	if err != nil {
		slog.Error("failed to signal workflow", slog.Any("error", err))
		writeError(w, http.StatusInternalServerError, "failed to trigger retry")
		return
	}

	slog.Info("triggered immediate retry", slog.String("mirror", mirrorName))

	writeJSON(w, http.StatusOK, MirrorResponse{
		Name:    mirrorName,
		Status:  "RETRYING",
		Message: "immediate retry signal sent (backoff bypassed)",
	})
}

// SyncSchema triggers a schema sync operation
func (h *Handler) SyncSchema(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	mirrorName := r.PathValue("name")

	if mirrorName == "" {
		writeError(w, http.StatusBadRequest, "mirror name is required")
		return
	}

	workflowID := fmt.Sprintf("cdc-%s", mirrorName)

	err := h.TemporalClient.SignalWorkflow(ctx, workflowID, "", workflows.SignalSyncSchema, model.SignalPayload{
		Signal: model.SyncSchemaSignal,
	})
	if err != nil {
		slog.Error("failed to signal workflow", slog.Any("error", err))
		writeError(w, http.StatusInternalServerError, "failed to trigger schema sync")
		return
	}

	writeJSON(w, http.StatusOK, MirrorResponse{
		Name:    mirrorName,
		Status:  "SYNCING_SCHEMA",
		Message: "schema sync signal sent",
	})
}

// ListMirrors lists all mirrors
func (h *Handler) ListMirrors(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	rows, err := h.CatalogPool.Query(ctx, `
		SELECT mirror_name, status,
			COALESCE(slot_name, ''),
			COALESCE(publication_name, ''),
			COALESCE(last_lsn, 0),
			error_message,
			COALESCE(error_count, 0)
		FROM bunny_internal.mirror_state
		ORDER BY mirror_name
	`)
	if err != nil {
		slog.Error("failed to list mirrors", slog.Any("error", err))
		writeError(w, http.StatusInternalServerError, "failed to list mirrors")
		return
	}
	defer rows.Close()

	mirrors := []MirrorStatusResponse{}
	for rows.Next() {
		var m MirrorStatusResponse
		var errMsg *string
		err := rows.Scan(&m.Name, &m.Status, &m.SlotName, &m.PublicationName, &m.LastLSN, &errMsg, &m.ErrorCount)
		if err != nil {
			slog.Error("failed to scan mirror row", slog.Any("error", err))
			continue
		}
		if errMsg != nil {
			m.ErrorMessage = *errMsg
		}
		mirrors = append(mirrors, m)
	}

	writeJSON(w, http.StatusOK, mirrors)
}

// ============================================================================
// Schema Discovery Handlers
// ============================================================================

// TableInfo represents a table in a database
type TableInfo struct {
	Schema    string `json:"schema"`
	TableName string `json:"table_name"`
	RowCount  int64  `json:"row_count,omitempty"`
}

// GetPeerTables returns all tables from a peer's database
func (h *Handler) GetPeerTables(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	peerName := r.PathValue("name")

	if peerName == "" {
		writeError(w, http.StatusBadRequest, "peer name is required")
		return
	}

	// Get peer connection info
	var host, user, password, database, sslMode string
	var port int
	err := h.CatalogPool.QueryRow(ctx, `
		SELECT host, port, username, password, database, ssl_mode
		FROM bunny_internal.peers
		WHERE name = $1
	`, peerName).Scan(&host, &port, &user, &password, &database, &sslMode)

	if err != nil {
		writeError(w, http.StatusNotFound, "peer not found")
		return
	}

	// Connect to the peer database
	connStr := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		user, password, host, port, database, sslMode)

	testCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	pool, err := pgxpool.New(testCtx, connStr)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to connect: %s", err.Error()))
		return
	}
	defer pool.Close()

	// Query for tables
	rows, err := pool.Query(testCtx, `
		SELECT
			schemaname as schema,
			tablename as table_name
		FROM pg_catalog.pg_tables
		WHERE schemaname NOT IN ('pg_catalog', 'information_schema', 'pg_toast')
		ORDER BY schemaname, tablename
	`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to query tables: %s", err.Error()))
		return
	}
	defer rows.Close()

	var tables []TableInfo
	for rows.Next() {
		var t TableInfo
		if err := rows.Scan(&t.Schema, &t.TableName); err != nil {
			continue
		}
		tables = append(tables, t)
	}

	if tables == nil {
		tables = []TableInfo{}
	}

	writeJSON(w, http.StatusOK, tables)
}

// ============================================================================
// Peer Handlers
// ============================================================================

// CreatePeer creates a new peer connection
func (h *Handler) CreatePeer(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req CreatePeerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" || req.Host == "" || req.Database == "" {
		writeError(w, http.StatusBadRequest, "name, host, and database are required")
		return
	}

	if req.Port == 0 {
		req.Port = 5432
	}
	if req.SSLMode == "" {
		req.SSLMode = "prefer"
	}

	var peerID int64
	err := h.CatalogPool.QueryRow(ctx, `
		INSERT INTO bunny_internal.peers (name, host, port, username, password, database, ssl_mode)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`, req.Name, req.Host, req.Port, req.User, req.Password, req.Database, req.SSLMode).Scan(&peerID)

	if err != nil {
		slog.Error("failed to create peer", slog.Any("error", err))
		writeError(w, http.StatusInternalServerError, "failed to create peer")
		return
	}

	writeJSON(w, http.StatusCreated, PeerResponse{
		ID:       peerID,
		Name:     req.Name,
		Host:     req.Host,
		Port:     req.Port,
		User:     req.User,
		Database: req.Database,
		SSLMode:  req.SSLMode,
	})
}

// ListPeers lists all peer connections
func (h *Handler) ListPeers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	rows, err := h.CatalogPool.Query(ctx, `
		SELECT id, name, host, port, username, database, ssl_mode
		FROM bunny_internal.peers
		ORDER BY name
	`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list peers")
		return
	}
	defer rows.Close()

	var peers []PeerResponse
	for rows.Next() {
		var p PeerResponse
		rows.Scan(&p.ID, &p.Name, &p.Host, &p.Port, &p.User, &p.Database, &p.SSLMode)
		peers = append(peers, p)
	}

	if peers == nil {
		peers = []PeerResponse{}
	}

	writeJSON(w, http.StatusOK, peers)
}

// GetPeer gets a single peer by name
func (h *Handler) GetPeer(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	peerName := r.PathValue("name")

	if peerName == "" {
		writeError(w, http.StatusBadRequest, "peer name is required")
		return
	}

	var p PeerResponse
	err := h.CatalogPool.QueryRow(ctx, `
		SELECT id, name, host, port, username, database, ssl_mode
		FROM bunny_internal.peers
		WHERE name = $1
	`, peerName).Scan(&p.ID, &p.Name, &p.Host, &p.Port, &p.User, &p.Database, &p.SSLMode)

	if err != nil {
		writeError(w, http.StatusNotFound, "peer not found")
		return
	}

	writeJSON(w, http.StatusOK, p)
}

// UpdatePeer updates a peer connection
func (h *Handler) UpdatePeer(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	peerName := r.PathValue("name")

	if peerName == "" {
		writeError(w, http.StatusBadRequest, "peer name is required")
		return
	}

	var req CreatePeerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Port == 0 {
		req.Port = 5432
	}
	if req.SSLMode == "" {
		req.SSLMode = "prefer"
	}

	result, err := h.CatalogPool.Exec(ctx, `
		UPDATE bunny_internal.peers
		SET host = $1, port = $2, username = $3, password = $4, database = $5, ssl_mode = $6
		WHERE name = $7
	`, req.Host, req.Port, req.User, req.Password, req.Database, req.SSLMode, peerName)

	if err != nil {
		slog.Error("failed to update peer", slog.Any("error", err))
		writeError(w, http.StatusInternalServerError, "failed to update peer")
		return
	}

	if result.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "peer not found")
		return
	}

	// Fetch updated peer to return
	var p PeerResponse
	err = h.CatalogPool.QueryRow(ctx, `
		SELECT id, name, host, port, username, database, ssl_mode
		FROM bunny_internal.peers
		WHERE name = $1
	`, peerName).Scan(&p.ID, &p.Name, &p.Host, &p.Port, &p.User, &p.Database, &p.SSLMode)

	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch updated peer")
		return
	}

	writeJSON(w, http.StatusOK, p)
}

// DeletePeer deletes a peer connection
func (h *Handler) DeletePeer(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	peerName := r.PathValue("name")

	if peerName == "" {
		writeError(w, http.StatusBadRequest, "peer name is required")
		return
	}

	result, err := h.CatalogPool.Exec(ctx, `
		DELETE FROM bunny_internal.peers WHERE name = $1
	`, peerName)

	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete peer")
		return
	}

	if result.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "peer not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// TestPeer tests connectivity to a peer
func (h *Handler) TestPeer(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	peerName := r.PathValue("name")

	if peerName == "" {
		writeError(w, http.StatusBadRequest, "peer name is required")
		return
	}

	var host, user, password, database, sslMode string
	var port int
	err := h.CatalogPool.QueryRow(ctx, `
		SELECT host, port, username, password, database, ssl_mode
		FROM bunny_internal.peers
		WHERE name = $1
	`, peerName).Scan(&host, &port, &user, &password, &database, &sslMode)

	if err != nil {
		writeError(w, http.StatusNotFound, "peer not found")
		return
	}

	connStr := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		user, password, host, port, database, sslMode)

	testCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(testCtx, connStr)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}
	defer pool.Close()

	var version string
	err = pool.QueryRow(ctx, "SELECT version()").Scan(&version)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"version": version,
	})
}

// ============================================================================
// Helper Functions
// ============================================================================

func setCORSHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	setCORSHeaders(w)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// corsMiddleware wraps a handler to add CORS support
func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next(w, r)
	}
}

// RegisterRoutes registers all API routes
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// CORS preflight handler for all routes
	mux.HandleFunc("OPTIONS /", func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w)
		w.WriteHeader(http.StatusOK)
	})

	// Peer CRUD
	mux.HandleFunc("POST /v1/peers", corsMiddleware(h.CreatePeer))
	mux.HandleFunc("GET /v1/peers", corsMiddleware(h.ListPeers))
	mux.HandleFunc("GET /v1/peers/{name}", corsMiddleware(h.GetPeer))
	mux.HandleFunc("PUT /v1/peers/{name}", corsMiddleware(h.UpdatePeer))
	mux.HandleFunc("DELETE /v1/peers/{name}", corsMiddleware(h.DeletePeer))
	mux.HandleFunc("POST /v1/peers/{name}/test", corsMiddleware(h.TestPeer))
	mux.HandleFunc("GET /v1/peers/{name}/tables", corsMiddleware(h.GetPeerTables))

	// Mirror CRUD
	mux.HandleFunc("POST /v1/mirrors", corsMiddleware(h.CreateMirror))
	mux.HandleFunc("GET /v1/mirrors", corsMiddleware(h.ListMirrors))
	mux.HandleFunc("GET /v1/mirrors/{name}", corsMiddleware(h.GetMirrorStatus))
	mux.HandleFunc("DELETE /v1/mirrors/{name}", corsMiddleware(h.DeleteMirror))

	// Mirror control
	mux.HandleFunc("POST /v1/mirrors/{name}/pause", corsMiddleware(h.PauseMirror))
	mux.HandleFunc("POST /v1/mirrors/{name}/resume", corsMiddleware(h.ResumeMirror))
	mux.HandleFunc("POST /v1/mirrors/{name}/resync", corsMiddleware(h.ResyncMirror))
	mux.HandleFunc("POST /v1/mirrors/{name}/resync/{table}", corsMiddleware(h.ResyncMirror))
	mux.HandleFunc("POST /v1/mirrors/{name}/retry", corsMiddleware(h.RetryMirror))
	mux.HandleFunc("POST /v1/mirrors/{name}/sync-schema", corsMiddleware(h.SyncSchema))

	// Health check
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
}
