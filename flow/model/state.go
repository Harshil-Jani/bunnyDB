package model

import "time"

// MirrorStatus represents the status of a mirror
type MirrorStatus string

const (
	MirrorStatusCreated     MirrorStatus = "CREATED"
	MirrorStatusSettingUp   MirrorStatus = "SETTING_UP"
	MirrorStatusSnapshot    MirrorStatus = "SNAPSHOT"
	MirrorStatusRunning     MirrorStatus = "RUNNING"
	MirrorStatusPaused      MirrorStatus = "PAUSED"
	MirrorStatusPausing     MirrorStatus = "PAUSING"
	MirrorStatusFailed      MirrorStatus = "FAILED"
	MirrorStatusTerminating MirrorStatus = "TERMINATING"
	MirrorStatusTerminated  MirrorStatus = "TERMINATED"
	MirrorStatusResyncing   MirrorStatus = "RESYNCING"
)

// CDCFlowState represents the state of a CDC workflow
type CDCFlowState struct {
	MirrorName string
	Status     MirrorStatus

	// Replication state
	SlotName        string
	PublicationName string
	LastLSN         int64
	LastSyncBatchID int64

	// Sync options
	SyncFlowOptions *SyncFlowOptions

	// Error tracking
	ErrorMessage string
	ErrorCount   int
	LastErrorAt  time.Time

	// Signal handling
	ActiveSignal   MirrorSignal
	SignalPayload  *SignalPayload

	// For resync
	IsResync          bool
	ResyncTableName   string  // For table-level resync

	// Timestamps
	CreatedAt time.Time
	UpdatedAt time.Time
}

// SyncFlowOptions contains options for sync operations
type SyncFlowOptions struct {
	BatchSize          uint32
	IdleTimeoutSeconds uint64

	// Table mappings
	TableMappings         []TableMapping
	SrcTableIDNameMapping map[uint32]string
}

// TableMapping represents a source to destination table mapping
type TableMapping struct {
	SourceSchema      string
	SourceTable       string
	DestinationSchema string
	DestinationTable  string
	PartitionKey      string
	ExcludeColumns    []string
}

// FullSourceName returns the full source table name
func (t *TableMapping) FullSourceName() string {
	return t.SourceSchema + "." + t.SourceTable
}

// FullDestinationName returns the full destination table name
func (t *TableMapping) FullDestinationName() string {
	return t.DestinationSchema + "." + t.DestinationTable
}

// NewCDCFlowState creates a new CDC flow state
func NewCDCFlowState(mirrorName string) *CDCFlowState {
	now := time.Now()
	return &CDCFlowState{
		MirrorName:      mirrorName,
		Status:          MirrorStatusCreated,
		SyncFlowOptions: &SyncFlowOptions{
			BatchSize:          1000,
			IdleTimeoutSeconds: 60,
			TableMappings:      []TableMapping{},
			SrcTableIDNameMapping: make(map[uint32]string),
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// UpdateStatus updates the mirror status
func (s *CDCFlowState) UpdateStatus(status MirrorStatus) {
	s.Status = status
	s.UpdatedAt = time.Now()
}

// RecordError records an error
func (s *CDCFlowState) RecordError(errMsg string) {
	s.ErrorMessage = errMsg
	s.ErrorCount++
	s.LastErrorAt = time.Now()
	s.UpdatedAt = time.Now()
}

// ClearError clears the error state
func (s *CDCFlowState) ClearError() {
	s.ErrorMessage = ""
	s.ErrorCount = 0
	s.UpdatedAt = time.Now()
}

// TableSyncStatus represents the sync status of a single table
type TableSyncStatus struct {
	MirrorName            string
	TableName             string
	Status                string
	RowsSynced            int64
	LastSyncedAt          *time.Time
	LastResyncRequestedAt *time.Time
	ErrorMessage          string
}
