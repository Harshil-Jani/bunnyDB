package model

// Signal types for workflow control
type MirrorSignal int

const (
	NoopSignal MirrorSignal = iota
	PauseSignal
	ResumeSignal
	TerminateSignal
	ResyncSignal
	ResyncTableSignal
	RetryNowSignal
	SyncSchemaSignal
)

// String returns the string representation of the signal
func (s MirrorSignal) String() string {
	switch s {
	case NoopSignal:
		return "NOOP"
	case PauseSignal:
		return "PAUSE"
	case ResumeSignal:
		return "RESUME"
	case TerminateSignal:
		return "TERMINATE"
	case ResyncSignal:
		return "RESYNC"
	case ResyncTableSignal:
		return "RESYNC_TABLE"
	case RetryNowSignal:
		return "RETRY_NOW"
	case SyncSchemaSignal:
		return "SYNC_SCHEMA"
	default:
		return "UNKNOWN"
	}
}

// SignalHandler handles flow signals and returns the new active signal
func SignalHandler(currentSignal MirrorSignal, newSignal MirrorSignal) MirrorSignal {
	switch newSignal {
	case PauseSignal:
		if currentSignal != TerminateSignal && currentSignal != ResyncSignal {
			return PauseSignal
		}
	case ResumeSignal:
		if currentSignal == PauseSignal {
			return NoopSignal
		}
	case TerminateSignal:
		return TerminateSignal
	case ResyncSignal:
		return ResyncSignal
	case ResyncTableSignal:
		return ResyncTableSignal
	case RetryNowSignal:
		return RetryNowSignal
	case SyncSchemaSignal:
		return SyncSchemaSignal
	}
	return currentSignal
}

// SignalPayload contains additional data for signals
type SignalPayload struct {
	Signal    MirrorSignal
	TableName string            // For table-specific operations
	Options   map[string]string // Additional options
}
