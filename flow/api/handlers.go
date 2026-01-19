package api

import (
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
		SELECT mirror_name, status, slot_name, publication_name, last_lsn, error_message
		FROM bunny_internal.mirror_state
		ORDER BY mirror_name
	`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list mirrors")
		return
	}
	defer rows.Close()

	var mirrors []MirrorStatusResponse
	for rows.Next() {
		var m MirrorStatusResponse
		var errMsg *string
		rows.Scan(&m.Name, &m.Status, &m.SlotName, &m.PublicationName, &m.LastLSN, &errMsg)
		if errMsg != nil {
			m.ErrorMessage = *errMsg
		}
		mirrors = append(mirrors, m)
	}

	writeJSON(w, http.StatusOK, mirrors)
}

// ============================================================================
// Helper Functions
// ============================================================================

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// RegisterRoutes registers all API routes
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// Mirror CRUD
	mux.HandleFunc("POST /v1/mirrors", h.CreateMirror)
	mux.HandleFunc("GET /v1/mirrors", h.ListMirrors)
	mux.HandleFunc("GET /v1/mirrors/{name}", h.GetMirrorStatus)
	mux.HandleFunc("DELETE /v1/mirrors/{name}", h.DeleteMirror)

	// Mirror control
	mux.HandleFunc("POST /v1/mirrors/{name}/pause", h.PauseMirror)
	mux.HandleFunc("POST /v1/mirrors/{name}/resume", h.ResumeMirror)
	mux.HandleFunc("POST /v1/mirrors/{name}/resync", h.ResyncMirror)
	mux.HandleFunc("POST /v1/mirrors/{name}/resync/{table}", h.ResyncMirror) // Table-level resync
	mux.HandleFunc("POST /v1/mirrors/{name}/retry", h.RetryMirror)
	mux.HandleFunc("POST /v1/mirrors/{name}/sync-schema", h.SyncSchema)

	// Health check
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
}
