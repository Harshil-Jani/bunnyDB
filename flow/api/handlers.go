package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.temporal.io/api/serviceerror"
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
	RowsInserted int64      `json:"rows_inserted,omitempty"`
	RowsUpdated  int64      `json:"rows_updated,omitempty"`
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

// UpdateTablesRequest is the request to update mirror tables
type UpdateTablesRequest struct {
	TableMappings []TableMappingInput `json:"table_mappings"`
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

	// Get peer IDs for storing in mirrors table
	var sourcePeerID, destPeerID int
	err := h.CatalogPool.QueryRow(ctx, `SELECT id FROM bunny_internal.peers WHERE name = $1`, req.SourcePeer).Scan(&sourcePeerID)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("source peer not found: %s", req.SourcePeer))
		return
	}
	err = h.CatalogPool.QueryRow(ctx, `SELECT id FROM bunny_internal.peers WHERE name = $1`, req.DestinationPeer).Scan(&destPeerID)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("destination peer not found: %s", req.DestinationPeer))
		return
	}

	// Store mirror config in mirrors table
	configJSON, _ := json.Marshal(map[string]interface{}{
		"do_initial_snapshot":             req.DoInitialSnapshot,
		"max_batch_size":                  req.MaxBatchSize,
		"idle_timeout_seconds":            req.IdleTimeoutSeconds,
		"snapshot_num_rows_per_partition": req.SnapshotNumRowsPerPartition,
		"snapshot_max_parallel_workers":   req.SnapshotMaxParallelWorkers,
		"snapshot_num_tables_in_parallel": req.SnapshotNumTablesInParallel,
		"replicate_indexes":               req.ReplicateIndexes,
		"replicate_foreign_keys":          req.ReplicateForeignKeys,
	})

	_, err = h.CatalogPool.Exec(ctx, `
		INSERT INTO bunny_internal.mirrors (name, source_peer_id, destination_peer_id, config, status)
		VALUES ($1, $2, $3, $4, 'CREATED')
		ON CONFLICT (name) DO UPDATE SET
			source_peer_id = $2,
			destination_peer_id = $3,
			config = $4,
			status = 'CREATED',
			updated_at = NOW()
	`, req.Name, sourcePeerID, destPeerID, configJSON)
	if err != nil {
		slog.Error("failed to create mirror", slog.Any("error", err))
		writeError(w, http.StatusInternalServerError, "failed to create mirror")
		return
	}

	// Insert mirror into mirror_state immediately so it shows up in the list
	_, err = h.CatalogPool.Exec(ctx, `
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

	// Log mirror creation
	h.WriteMirrorLog(ctx, req.Name, "INFO", "Mirror created", map[string]interface{}{
		"workflow_id":      we.GetID(),
		"source_peer":      req.SourcePeer,
		"destination_peer": req.DestinationPeer,
		"table_count":      len(req.TableMappings),
	})

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

	// First try to get state from catalog (always available)
	var response MirrorStatusResponse
	var errMsg *string
	err := h.CatalogPool.QueryRow(ctx, `
		SELECT mirror_name, status,
			COALESCE(slot_name, ''),
			COALESCE(publication_name, ''),
			COALESCE(last_lsn, 0),
			COALESCE(last_sync_batch_id, 0),
			error_message,
			COALESCE(error_count, 0)
		FROM bunny_internal.mirror_state
		WHERE mirror_name = $1
	`, mirrorName).Scan(
		&response.Name, &response.Status, &response.SlotName, &response.PublicationName,
		&response.LastLSN, &response.LastSyncBatchID, &errMsg, &response.ErrorCount)

	if err != nil {
		writeError(w, http.StatusNotFound, "mirror not found")
		return
	}

	if errMsg != nil {
		response.ErrorMessage = *errMsg
	}

	// Try to get live state from workflow (may not be available if workflow is still starting)
	// Note: We only override status from Temporal, not LSN/BatchID which are updated by the activity directly in the database
	workflowID := fmt.Sprintf("cdc-%s", mirrorName)
	resp, err := h.TemporalClient.QueryWorkflow(ctx, workflowID, "", workflows.QueryFlowState)
	if err == nil {
		var state model.CDCFlowState
		if err := resp.Get(&state); err == nil {
			// Only override status and error info from Temporal
			// Keep LSN and BatchID from database (activity updates them directly)
			response.Status = string(state.Status)
			if state.SlotName != "" {
				response.SlotName = state.SlotName
			}
			if state.PublicationName != "" {
				response.PublicationName = state.PublicationName
			}
			response.ErrorMessage = state.ErrorMessage
			response.ErrorCount = state.ErrorCount
		}
	}

	// Get table statuses from catalog
	rows, err := h.CatalogPool.Query(ctx, `
		SELECT table_name, status, rows_synced, COALESCE(rows_inserted, 0), COALESCE(rows_updated, 0), last_synced_at
		FROM bunny_stats.table_sync_status
		WHERE mirror_name = $1
	`, mirrorName)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var ts TableStatusResponse
			rows.Scan(&ts.TableName, &ts.Status, &ts.RowsSynced, &ts.RowsInserted, &ts.RowsUpdated, &ts.LastSyncedAt)
			response.Tables = append(response.Tables, ts)
		}
	}

	writeJSON(w, http.StatusOK, response)
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
		slog.Error("failed to signal workflow", slog.String("workflowID", workflowID), slog.Any("error", err))

		// Check if workflow is not found or already completed
		var notFoundErr *serviceerror.NotFound
		if strings.Contains(err.Error(), "workflow execution already completed") {
			// Update database status to reflect the workflow is not running
			h.CatalogPool.Exec(ctx, `
				UPDATE bunny_internal.mirror_state
				SET status = 'TERMINATED', error_message = 'Workflow completed unexpectedly', updated_at = NOW()
				WHERE mirror_name = $1 AND status NOT IN ('TERMINATED', 'PAUSED', 'FAILED')
			`, mirrorName)
			writeError(w, http.StatusConflict, "workflow has already completed - mirror status updated. Please restart the mirror.")
			return
		} else if errors.As(err, &notFoundErr) {
			writeError(w, http.StatusNotFound, "workflow not found - mirror may not be running")
			return
		}

		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to pause mirror: %v", err))
		return
	}

	// Update status in database
	h.CatalogPool.Exec(ctx, `
		UPDATE bunny_internal.mirror_state SET status = 'PAUSING', updated_at = NOW()
		WHERE mirror_name = $1
	`, mirrorName)

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
		slog.Error("failed to signal workflow", slog.String("workflowID", workflowID), slog.Any("error", err))
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to resume mirror: %v", err))
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

	// Try to terminate the workflow (may fail if workflow doesn't exist)
	err := h.TemporalClient.SignalWorkflow(ctx, workflowID, "", workflows.SignalTerminate, model.SignalPayload{
		Signal: model.TerminateSignal,
	})
	if err != nil {
		slog.Warn("failed to signal workflow for termination (may not exist)", slog.Any("error", err))
		// Try to cancel the workflow directly
		err = h.TemporalClient.CancelWorkflow(ctx, workflowID, "")
		if err != nil {
			slog.Warn("failed to cancel workflow", slog.Any("error", err))
		}
	}

	// Always clean up catalog entries
	_, err = h.CatalogPool.Exec(ctx, `DELETE FROM bunny_stats.table_sync_status WHERE mirror_name = $1`, mirrorName)
	if err != nil {
		slog.Warn("failed to delete table sync status", slog.Any("error", err))
	}

	_, err = h.CatalogPool.Exec(ctx, `DELETE FROM bunny_internal.fk_definitions WHERE mirror_name = $1`, mirrorName)
	if err != nil {
		slog.Warn("failed to delete fk definitions", slog.Any("error", err))
	}

	_, err = h.CatalogPool.Exec(ctx, `DELETE FROM bunny_internal.index_definitions WHERE mirror_name = $1`, mirrorName)
	if err != nil {
		slog.Warn("failed to delete index definitions", slog.Any("error", err))
	}

	_, err = h.CatalogPool.Exec(ctx, `DELETE FROM bunny_internal.mirror_state WHERE mirror_name = $1`, mirrorName)
	if err != nil {
		slog.Error("failed to delete mirror state", slog.Any("error", err))
		writeError(w, http.StatusInternalServerError, "failed to delete mirror")
		return
	}

	slog.Info("deleted mirror", slog.String("mirror", mirrorName))

	writeJSON(w, http.StatusOK, MirrorResponse{
		Name:    mirrorName,
		Status:  "DELETED",
		Message: "mirror deleted",
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
// If the workflow has completed, it will restart the workflow from scratch
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
		// Check if workflow is completed - if so, restart it
		if strings.Contains(err.Error(), "workflow execution already completed") {
			slog.Info("workflow completed, restarting mirror", slog.String("mirror", mirrorName))
			h.restartMirrorWorkflow(ctx, w, mirrorName)
			return
		}

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

// restartMirrorWorkflow restarts a mirror workflow from the stored configuration
func (h *Handler) restartMirrorWorkflow(ctx context.Context, w http.ResponseWriter, mirrorName string) {
	// Get mirror configuration from database
	var sourcePeerName, destPeerName string
	var configJSON []byte
	err := h.CatalogPool.QueryRow(ctx, `
		SELECT sp.name, dp.name, m.config
		FROM bunny_internal.mirrors m
		JOIN bunny_internal.peers sp ON m.source_peer_id = sp.id
		JOIN bunny_internal.peers dp ON m.destination_peer_id = dp.id
		WHERE m.name = $1
	`, mirrorName).Scan(&sourcePeerName, &destPeerName, &configJSON)
	if err != nil {
		slog.Error("failed to get mirror config", slog.Any("error", err))
		writeError(w, http.StatusNotFound, "mirror configuration not found")
		return
	}

	// Parse config
	var config map[string]interface{}
	if err := json.Unmarshal(configJSON, &config); err != nil {
		slog.Error("failed to parse mirror config", slog.Any("error", err))
		writeError(w, http.StatusInternalServerError, "failed to parse mirror configuration")
		return
	}

	// Get table mappings from config
	var tableMappings []model.TableMapping
	if tm, ok := config["table_mappings"].([]interface{}); ok {
		for _, t := range tm {
			if tmap, ok := t.(map[string]interface{}); ok {
				tableMappings = append(tableMappings, model.TableMapping{
					SourceSchema:      getString(tmap, "source_schema"),
					SourceTable:       getString(tmap, "source_table"),
					DestinationSchema: getString(tmap, "destination_schema"),
					DestinationTable:  getString(tmap, "destination_table"),
					PartitionKey:      getString(tmap, "partition_key"),
				})
			}
		}
	}

	// If no table mappings in config, get them from table_sync_status
	if len(tableMappings) == 0 {
		rows, err := h.CatalogPool.Query(ctx, `
			SELECT table_name FROM bunny_stats.table_sync_status WHERE mirror_name = $1
		`, mirrorName)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var tableName string
				if err := rows.Scan(&tableName); err == nil {
					// Parse schema.table format
					parts := strings.SplitN(tableName, ".", 2)
					schema := "public"
					table := tableName
					if len(parts) == 2 {
						schema = parts[0]
						table = parts[1]
					}
					tableMappings = append(tableMappings, model.TableMapping{
						SourceSchema:      schema,
						SourceTable:       table,
						DestinationSchema: schema,
						DestinationTable:  table,
					})
				}
			}
		}
	}

	if len(tableMappings) == 0 {
		writeError(w, http.StatusBadRequest, "no table mappings found for mirror")
		return
	}

	slog.Info("restarting mirror with table mappings",
		slog.String("mirror", mirrorName),
		slog.Int("tableCount", len(tableMappings)))

	// Get last LSN and batch ID from mirror_state to resume from
	var lastLSN int64
	var lastBatchID int64
	h.CatalogPool.QueryRow(ctx, `
		SELECT COALESCE(last_lsn, 0), COALESCE(last_sync_batch_id, 0)
		FROM bunny_internal.mirror_state WHERE mirror_name = $1
	`, mirrorName).Scan(&lastLSN, &lastBatchID)

	// Start the workflow
	workflowID := fmt.Sprintf("cdc-%s", mirrorName)
	workflowOptions := client.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: h.Config.WorkerTaskQueue,
	}

	input := &workflows.CDCFlowInput{
		MirrorName:         mirrorName,
		SourcePeer:         sourcePeerName,
		DestinationPeer:    destPeerName,
		TableMappings:      tableMappings,
		DoInitialSnapshot:  false, // Don't re-snapshot on restart
		MaxBatchSize:       uint32(getInt(config, "max_batch_size", 1000)),
		IdleTimeoutSeconds: uint64(getInt(config, "idle_timeout_seconds", 60)),
	}

	// Create initial state with last LSN/BatchID to resume CDC
	state := model.NewCDCFlowState(mirrorName)
	state.LastLSN = lastLSN
	state.LastSyncBatchID = lastBatchID
	state.SyncFlowOptions.TableMappings = tableMappings
	state.SyncFlowOptions.BatchSize = input.MaxBatchSize
	state.SyncFlowOptions.IdleTimeoutSeconds = input.IdleTimeoutSeconds

	we, err := h.TemporalClient.ExecuteWorkflow(ctx, workflowOptions, workflows.CDCFlowWorkflow, input, state)
	if err != nil {
		slog.Error("failed to restart workflow", slog.Any("error", err))
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to restart mirror: %v", err))
		return
	}

	// Update mirror state
	h.CatalogPool.Exec(ctx, `
		UPDATE bunny_internal.mirror_state
		SET status = 'RUNNING', error_message = NULL, error_count = 0, updated_at = NOW()
		WHERE mirror_name = $1
	`, mirrorName)

	slog.Info("restarted mirror workflow",
		slog.String("mirror", mirrorName),
		slog.String("workflowID", we.GetID()),
		slog.Int64("lastLSN", lastLSN))

	writeJSON(w, http.StatusOK, MirrorResponse{
		Name:    mirrorName,
		Status:  "RUNNING",
		Message: "mirror workflow restarted",
	})
}

// Helper functions for config parsing
func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getInt(m map[string]interface{}, key string, defaultVal int) int {
	if v, ok := m[key].(float64); ok {
		return int(v)
	}
	return defaultVal
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

// GetMirrorTables returns the table mappings for a mirror
func (h *Handler) GetMirrorTables(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	mirrorName := r.PathValue("name")

	if mirrorName == "" {
		writeError(w, http.StatusBadRequest, "mirror name is required")
		return
	}

	// Get mirror config and publication name from database
	var configJSON []byte
	var publicationName string
	var sourcePeerName string
	err := h.CatalogPool.QueryRow(ctx, `
		SELECT m.config, COALESCE(ms.publication_name, ''), p.name
		FROM bunny_internal.mirrors m
		JOIN bunny_internal.peers p ON m.source_peer_id = p.id
		LEFT JOIN bunny_internal.mirror_state ms ON m.name = ms.mirror_name
		WHERE m.name = $1
	`, mirrorName).Scan(&configJSON, &publicationName, &sourcePeerName)
	if err != nil {
		writeError(w, http.StatusNotFound, "mirror not found")
		return
	}

	// Parse config to get table mappings
	var config map[string]interface{}
	if err := json.Unmarshal(configJSON, &config); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to parse mirror config")
		return
	}

	// Get table sync status
	syncStatusMap := make(map[string]TableStatusResponse)
	rows, err := h.CatalogPool.Query(ctx, `
		SELECT table_name, status, rows_synced, COALESCE(rows_inserted, 0), COALESCE(rows_updated, 0), last_synced_at
		FROM bunny_stats.table_sync_status
		WHERE mirror_name = $1
	`, mirrorName)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var ts TableStatusResponse
			rows.Scan(&ts.TableName, &ts.Status, &ts.RowsSynced, &ts.RowsInserted, &ts.RowsUpdated, &ts.LastSyncedAt)
			syncStatusMap[ts.TableName] = ts
		}
	}

	// Get tables from publication on source database
	var tables []TableStatusResponse
	if publicationName != "" && sourcePeerName != "" {
		pubTables, err := h.getPublicationTables(ctx, sourcePeerName, publicationName)
		if err != nil {
			slog.Warn("failed to get publication tables", slog.Any("error", err))
		} else {
			for _, tableName := range pubTables {
				if status, ok := syncStatusMap[tableName]; ok {
					tables = append(tables, status)
				} else {
					// Table in publication but not yet synced
					tables = append(tables, TableStatusResponse{
						TableName:  tableName,
						Status:     "PENDING",
						RowsSynced: 0,
					})
				}
			}
		}
	}

	// If we couldn't get publication tables, fall back to sync status only
	if len(tables) == 0 {
		for _, ts := range syncStatusMap {
			tables = append(tables, ts)
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"config": config,
		"tables": tables,
	})
}

// getPublicationTables queries the source database for tables in the publication
func (h *Handler) getPublicationTables(ctx context.Context, peerName, publicationName string) ([]string, error) {
	// Get peer connection info
	var host, user, password, database, sslMode string
	var port int
	err := h.CatalogPool.QueryRow(ctx, `
		SELECT host, port, username, password, database, ssl_mode
		FROM bunny_internal.peers WHERE name = $1
	`, peerName).Scan(&host, &port, &user, &password, &database, &sslMode)
	if err != nil {
		return nil, fmt.Errorf("failed to get peer config: %w", err)
	}

	// Connect to source database
	connStr := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		user, password, host, port, database, sslMode)

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to source: %w", err)
	}
	defer pool.Close()

	// Query publication tables
	rows, err := pool.Query(ctx, `
		SELECT schemaname || '.' || tablename as table_name
		FROM pg_publication_tables
		WHERE pubname = $1
		ORDER BY schemaname, tablename
	`, publicationName)
	if err != nil {
		return nil, fmt.Errorf("failed to query publication tables: %w", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err == nil {
			tables = append(tables, tableName)
		}
	}

	return tables, nil
}

// GetAvailableTables returns tables from source database that are NOT already in the mirror
func (h *Handler) GetAvailableTables(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	mirrorName := r.PathValue("name")

	if mirrorName == "" {
		writeError(w, http.StatusBadRequest, "mirror name is required")
		return
	}

	// Get source peer and publication name
	var sourcePeerName string
	var publicationName string
	err := h.CatalogPool.QueryRow(ctx, `
		SELECT p.name, COALESCE(ms.publication_name, '')
		FROM bunny_internal.mirrors m
		JOIN bunny_internal.peers p ON m.source_peer_id = p.id
		LEFT JOIN bunny_internal.mirror_state ms ON m.name = ms.mirror_name
		WHERE m.name = $1
	`, mirrorName).Scan(&sourcePeerName, &publicationName)
	if err != nil {
		writeError(w, http.StatusNotFound, "mirror not found")
		return
	}

	// Get current tables in the mirror (from publication)
	currentTables := make(map[string]bool)
	if publicationName != "" {
		pubTables, err := h.getPublicationTables(ctx, sourcePeerName, publicationName)
		if err == nil {
			for _, t := range pubTables {
				currentTables[t] = true
			}
		}
	}

	// Get peer connection info
	var host, user, password, database, sslMode string
	var port int
	err = h.CatalogPool.QueryRow(ctx, `
		SELECT host, port, username, password, database, ssl_mode
		FROM bunny_internal.peers WHERE name = $1
	`, sourcePeerName).Scan(&host, &port, &user, &password, &database, &sslMode)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get peer config")
		return
	}

	// Connect to source database
	connStr := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		user, password, host, port, database, sslMode)

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to connect to source: %s", err.Error()))
		return
	}
	defer pool.Close()

	// Query for ALL tables in the source database
	rows, err := pool.Query(ctx, `
		SELECT schemaname, tablename
		FROM pg_catalog.pg_tables
		WHERE schemaname NOT IN ('pg_catalog', 'information_schema', 'pg_toast')
		ORDER BY schemaname, tablename
	`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to query tables: %s", err.Error()))
		return
	}
	defer rows.Close()

	var availableTables []TableInfo
	for rows.Next() {
		var schema, tableName string
		if err := rows.Scan(&schema, &tableName); err != nil {
			continue
		}
		fullName := fmt.Sprintf("%s.%s", schema, tableName)
		// Only include tables NOT already in the mirror
		if !currentTables[fullName] {
			availableTables = append(availableTables, TableInfo{
				Schema:    schema,
				TableName: tableName,
			})
		}
	}

	if availableTables == nil {
		availableTables = []TableInfo{}
	}

	writeJSON(w, http.StatusOK, availableTables)
}

// UpdateMirrorTables updates the table mappings for a paused mirror
func (h *Handler) UpdateMirrorTables(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	mirrorName := r.PathValue("name")

	if mirrorName == "" {
		writeError(w, http.StatusBadRequest, "mirror name is required")
		return
	}

	// Check if mirror is paused - first check database, then try Temporal workflow for live state
	var status string
	err := h.CatalogPool.QueryRow(ctx, `
		SELECT status FROM bunny_internal.mirror_state WHERE mirror_name = $1
	`, mirrorName).Scan(&status)
	if err != nil {
		writeError(w, http.StatusNotFound, "mirror not found")
		return
	}

	// Try to get live state from workflow (may override database status)
	workflowID := fmt.Sprintf("cdc-%s", mirrorName)
	resp, queryErr := h.TemporalClient.QueryWorkflow(ctx, workflowID, "", workflows.QueryFlowState)
	if queryErr == nil {
		var state model.CDCFlowState
		if err := resp.Get(&state); err == nil {
			status = string(state.Status)
		}
	}

	if status != "PAUSED" {
		writeError(w, http.StatusBadRequest, "mirror must be paused to update tables")
		return
	}

	// Parse request
	var req UpdateTablesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.TableMappings) == 0 {
		writeError(w, http.StatusBadRequest, "at least one table mapping is required")
		return
	}

	// Get source peer to update publication
	var sourcePeerName string
	err = h.CatalogPool.QueryRow(ctx, `
		SELECT p.name FROM bunny_internal.mirrors m
		JOIN bunny_internal.peers p ON m.source_peer_id = p.id
		WHERE m.name = $1
	`, mirrorName).Scan(&sourcePeerName)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get source peer")
		return
	}

	// Get source peer connection info
	var host, user, password, database, sslMode string
	var port int
	err = h.CatalogPool.QueryRow(ctx, `
		SELECT host, port, username, password, database, ssl_mode
		FROM bunny_internal.peers WHERE name = $1
	`, sourcePeerName).Scan(&host, &port, &user, &password, &database, &sslMode)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get peer config")
		return
	}

	// Get publication name
	var publicationName string
	err = h.CatalogPool.QueryRow(ctx, `
		SELECT publication_name FROM bunny_internal.mirror_state WHERE mirror_name = $1
	`, mirrorName).Scan(&publicationName)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get publication name")
		return
	}

	// Build table list for publication
	var tables []string
	for _, tm := range req.TableMappings {
		tables = append(tables, fmt.Sprintf("%s.%s", tm.SourceSchema, tm.SourceTable))
	}

	// Connect to source and update publication
	connStr := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		user, password, host, port, database, sslMode)

	updateCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	pool, err := pgxpool.New(updateCtx, connStr)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to connect to source: %s", err.Error()))
		return
	}
	defer pool.Close()

	// Update publication to use new tables
	alterSQL := fmt.Sprintf("ALTER PUBLICATION %s SET TABLE %s", publicationName, joinTables(tables))
	_, err = pool.Exec(updateCtx, alterSQL)
	if err != nil {
		slog.Error("failed to update publication", slog.Any("error", err))
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to update publication: %s", err.Error()))
		return
	}

	// Update mirror config in database
	configJSON, _ := json.Marshal(map[string]interface{}{
		"table_mappings": req.TableMappings,
	})

	_, err = h.CatalogPool.Exec(ctx, `
		UPDATE bunny_internal.mirrors SET config = config || $2, updated_at = NOW()
		WHERE name = $1
	`, mirrorName, configJSON)
	if err != nil {
		slog.Warn("failed to update mirror config", slog.Any("error", err))
	}

	// Log the update
	h.WriteMirrorLog(ctx, mirrorName, "INFO", "Tables updated", map[string]interface{}{
		"table_count": len(req.TableMappings),
		"tables":      tables,
	})

	slog.Info("mirror tables updated",
		slog.String("mirror", mirrorName),
		slog.Int("tableCount", len(req.TableMappings)))

	writeJSON(w, http.StatusOK, MirrorResponse{
		Name:    mirrorName,
		Status:  "PAUSED",
		Message: fmt.Sprintf("tables updated (%d tables)", len(req.TableMappings)),
	})
}

func joinTables(tables []string) string {
	if len(tables) == 0 {
		return ""
	}
	result := make([]string, len(tables))
	for i, t := range tables {
		// Tables should already be in schema.table format
		result[i] = t
	}
	return strings.Join(result, ", ")
}

// ListMirrors lists all mirrors
func (h *Handler) ListMirrors(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	rows, err := h.CatalogPool.Query(ctx, `
		SELECT mirror_name, status,
			COALESCE(slot_name, ''),
			COALESCE(publication_name, ''),
			COALESCE(last_lsn, 0),
			COALESCE(last_sync_batch_id, 0),
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
		err := rows.Scan(&m.Name, &m.Status, &m.SlotName, &m.PublicationName, &m.LastLSN, &m.LastSyncBatchID, &errMsg, &m.ErrorCount)
		if err != nil {
			slog.Error("failed to scan mirror row", slog.Any("error", err))
			continue
		}
		if errMsg != nil {
			m.ErrorMessage = *errMsg
		}
		mirrors = append(mirrors, m)
	}

	// Query Temporal to get live status for each mirror
	// Note: We only override status from Temporal, not LSN/BatchID which are updated by the activity directly in the database
	for i := range mirrors {
		workflowID := fmt.Sprintf("cdc-%s", mirrors[i].Name)
		resp, err := h.TemporalClient.QueryWorkflow(ctx, workflowID, "", workflows.QueryFlowState)
		if err == nil {
			var state model.CDCFlowState
			if err := resp.Get(&state); err == nil {
				// Only override status and error info from Temporal
				// Keep LSN and BatchID from database (activity updates them directly)
				mirrors[i].Status = string(state.Status)
				if state.SlotName != "" {
					mirrors[i].SlotName = state.SlotName
				}
				if state.PublicationName != "" {
					mirrors[i].PublicationName = state.PublicationName
				}
				mirrors[i].ErrorMessage = state.ErrorMessage
				mirrors[i].ErrorCount = state.ErrorCount
			}
		}
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
// Mirror Logs
// ============================================================================

// LogEntry represents a log entry
type LogEntry struct {
	ID        int64      `json:"id"`
	Level     string     `json:"level"`
	Message   string     `json:"message"`
	Details   *string    `json:"details,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// GetMirrorLogs returns logs for a specific mirror
func (h *Handler) GetMirrorLogs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	mirrorName := r.PathValue("name")

	if mirrorName == "" {
		writeError(w, http.StatusBadRequest, "mirror name is required")
		return
	}

	// Get limit from query params (default 100)
	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := fmt.Sscanf(l, "%d", &limit); err == nil && parsed > 0 {
			if limit > 500 {
				limit = 500
			}
		}
	}

	rows, err := h.CatalogPool.Query(ctx, `
		SELECT id, log_level, message, details::text, created_at
		FROM bunny_stats.mirror_logs
		WHERE mirror_name = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, mirrorName, limit)
	if err != nil {
		slog.Error("failed to fetch logs", slog.Any("error", err))
		writeError(w, http.StatusInternalServerError, "failed to fetch logs")
		return
	}
	defer rows.Close()

	logs := []LogEntry{}
	for rows.Next() {
		var log LogEntry
		err := rows.Scan(&log.ID, &log.Level, &log.Message, &log.Details, &log.CreatedAt)
		if err != nil {
			slog.Error("failed to scan log row", slog.Any("error", err))
			continue
		}
		logs = append(logs, log)
	}

	writeJSON(w, http.StatusOK, logs)
}

// WriteMirrorLog writes a log entry for a mirror (internal helper)
func (h *Handler) WriteMirrorLog(ctx context.Context, mirrorName, level, message string, details map[string]interface{}) {
	var detailsJSON []byte
	if details != nil {
		detailsJSON, _ = json.Marshal(details)
	}

	_, err := h.CatalogPool.Exec(ctx, `
		INSERT INTO bunny_stats.mirror_logs (mirror_name, log_level, message, details)
		VALUES ($1, $2, $3, $4)
	`, mirrorName, level, message, detailsJSON)
	if err != nil {
		slog.Error("failed to write mirror log", slog.Any("error", err))
	}
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
	mux.HandleFunc("GET /v1/mirrors/{name}/logs", corsMiddleware(h.GetMirrorLogs))
	mux.HandleFunc("GET /v1/mirrors/{name}/tables", corsMiddleware(h.GetMirrorTables))
	mux.HandleFunc("GET /v1/mirrors/{name}/available-tables", corsMiddleware(h.GetAvailableTables))
	mux.HandleFunc("PUT /v1/mirrors/{name}/tables", corsMiddleware(h.UpdateMirrorTables))

	// Health check
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
}
