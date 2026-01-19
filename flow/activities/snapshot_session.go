package activities

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"go.temporal.io/sdk/activity"
)

// SnapshotSession manages a long-lived database connection that holds a snapshot open.
// The snapshot is only valid while this session is active.
type SnapshotSession struct {
	MirrorName   string
	SnapshotName string
	conn         *pgx.Conn
	tx           pgx.Tx
	mu           sync.Mutex
	closed       bool
}

// snapshotSessionRegistry holds active snapshot sessions by mirror name
var (
	snapshotSessions   = make(map[string]*SnapshotSession)
	snapshotSessionsMu sync.RWMutex
)

// StartSnapshotSessionInput is the input for starting a snapshot session
type StartSnapshotSessionInput struct {
	MirrorName string
	SourcePeer string
}

// StartSnapshotSessionOutput is the output of starting a snapshot session
type StartSnapshotSessionOutput struct {
	SnapshotName string
}

// StartSnapshotSession creates a long-lived connection and exports a snapshot.
// The snapshot remains valid until EndSnapshotSession is called.
// This is a LONG-RUNNING activity that holds the connection open.
func (a *Activities) StartSnapshotSession(ctx context.Context, input *StartSnapshotSessionInput) (*StartSnapshotSessionOutput, error) {
	logger := slog.Default().With(slog.String("mirror", input.MirrorName))
	logger.Info("starting snapshot session")

	// Check if session already exists
	snapshotSessionsMu.RLock()
	if existing, ok := snapshotSessions[input.MirrorName]; ok && !existing.closed {
		snapshotSessionsMu.RUnlock()
		logger.Info("reusing existing snapshot session", slog.String("snapshot", existing.SnapshotName))
		return &StartSnapshotSessionOutput{SnapshotName: existing.SnapshotName}, nil
	}
	snapshotSessionsMu.RUnlock()

	// Get source peer config
	srcConfig, err := a.getPeerConfig(ctx, input.SourcePeer)
	if err != nil {
		return nil, fmt.Errorf("failed to get source peer config: %w", err)
	}

	// Create a raw pgx connection (not through our connector, to have direct control)
	connString := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		srcConfig.Host, srcConfig.Port, srcConfig.User, srcConfig.Password, srcConfig.Database, srcConfig.SSLMode,
	)

	conn, err := pgx.Connect(ctx, connString)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to source: %w", err)
	}

	// Start a REPEATABLE READ transaction - this is REQUIRED for snapshot export
	tx, err := conn.Begin(ctx)
	if err != nil {
		conn.Close(ctx)
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Set isolation level FIRST
	_, err = tx.Exec(ctx, "SET TRANSACTION ISOLATION LEVEL REPEATABLE READ")
	if err != nil {
		tx.Rollback(ctx)
		conn.Close(ctx)
		return nil, fmt.Errorf("failed to set isolation level: %w", err)
	}

	// Export the snapshot - this creates a snapshot ID that can be used by other connections
	var snapshotName string
	err = tx.QueryRow(ctx, "SELECT pg_export_snapshot()").Scan(&snapshotName)
	if err != nil {
		tx.Rollback(ctx)
		conn.Close(ctx)
		return nil, fmt.Errorf("failed to export snapshot: %w", err)
	}

	// Create and store the session
	session := &SnapshotSession{
		MirrorName:   input.MirrorName,
		SnapshotName: snapshotName,
		conn:         conn,
		tx:           tx,
		closed:       false,
	}

	snapshotSessionsMu.Lock()
	snapshotSessions[input.MirrorName] = session
	snapshotSessionsMu.Unlock()

	logger.Info("snapshot session started",
		slog.String("snapshot", snapshotName),
		slog.String("mirror", input.MirrorName))

	a.WriteLog(ctx, input.MirrorName, "INFO", "Snapshot session started", map[string]interface{}{
		"snapshot": snapshotName,
	})

	return &StartSnapshotSessionOutput{SnapshotName: snapshotName}, nil
}

// HoldSnapshotSessionInput is the input for holding a snapshot session open
type HoldSnapshotSessionInput struct {
	MirrorName string
}

// HoldSnapshotSession is a LONG-RUNNING activity that keeps the snapshot session alive.
// It should be run in the background and cancelled when the snapshot is no longer needed.
// This activity sends periodic heartbeats to keep both Temporal and the DB connection alive.
func (a *Activities) HoldSnapshotSession(ctx context.Context, input *HoldSnapshotSessionInput) error {
	logger := slog.Default().With(slog.String("mirror", input.MirrorName))
	logger.Info("holding snapshot session open")

	snapshotSessionsMu.RLock()
	session, ok := snapshotSessions[input.MirrorName]
	snapshotSessionsMu.RUnlock()

	if !ok {
		return fmt.Errorf("no snapshot session found for mirror %s", input.MirrorName)
	}

	// Keep the session alive with periodic heartbeats
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Context cancelled - this is the expected way to end the session
			logger.Info("snapshot session ending (context cancelled)")
			return ctx.Err()

		case <-ticker.C:
			// Send heartbeat to Temporal
			activity.RecordHeartbeat(ctx, fmt.Sprintf("snapshot session active: %s", session.SnapshotName))

			// Keep the DB connection alive with a simple query
			session.mu.Lock()
			if !session.closed && session.tx != nil {
				_, err := session.tx.Exec(ctx, "SELECT 1")
				if err != nil {
					session.mu.Unlock()
					logger.Error("snapshot session keepalive failed", slog.Any("error", err))
					return fmt.Errorf("snapshot session keepalive failed: %w", err)
				}
			}
			session.mu.Unlock()
		}
	}
}

// EndSnapshotSessionInput is the input for ending a snapshot session
type EndSnapshotSessionInput struct {
	MirrorName string
}

// EndSnapshotSession closes the snapshot session and releases the snapshot.
// This should be called after all table copies are complete.
func (a *Activities) EndSnapshotSession(ctx context.Context, input *EndSnapshotSessionInput) error {
	logger := slog.Default().With(slog.String("mirror", input.MirrorName))
	logger.Info("ending snapshot session")

	snapshotSessionsMu.Lock()
	session, ok := snapshotSessions[input.MirrorName]
	if !ok {
		snapshotSessionsMu.Unlock()
		logger.Warn("no snapshot session found to end")
		return nil
	}
	delete(snapshotSessions, input.MirrorName)
	snapshotSessionsMu.Unlock()

	session.mu.Lock()
	defer session.mu.Unlock()

	if session.closed {
		return nil
	}
	session.closed = true

	// Commit the transaction (or rollback - doesn't matter for read-only)
	if session.tx != nil {
		if err := session.tx.Commit(ctx); err != nil {
			logger.Warn("failed to commit snapshot transaction", slog.Any("error", err))
			// Try rollback
			session.tx.Rollback(ctx)
		}
	}

	// Close the connection
	if session.conn != nil {
		if err := session.conn.Close(ctx); err != nil {
			logger.Warn("failed to close snapshot connection", slog.Any("error", err))
		}
	}

	logger.Info("snapshot session ended", slog.String("snapshot", session.SnapshotName))
	a.WriteLog(ctx, input.MirrorName, "INFO", "Snapshot session ended", map[string]interface{}{
		"snapshot": session.SnapshotName,
	})

	return nil
}

// GetSnapshotName returns the snapshot name for a mirror's active session
func GetSnapshotName(mirrorName string) (string, bool) {
	snapshotSessionsMu.RLock()
	defer snapshotSessionsMu.RUnlock()

	session, ok := snapshotSessions[mirrorName]
	if !ok || session.closed {
		return "", false
	}
	return session.SnapshotName, true
}

// SnapshotSessionInfo returns information about the snapshot session
type SnapshotSessionInfo struct {
	MirrorName   string
	SnapshotName string
	IsActive     bool
}

// GetSnapshotSessionInfo returns info about an active snapshot session
func (a *Activities) GetSnapshotSessionInfo(ctx context.Context, mirrorName string) (*SnapshotSessionInfo, error) {
	snapshotSessionsMu.RLock()
	defer snapshotSessionsMu.RUnlock()

	session, ok := snapshotSessions[mirrorName]
	if !ok {
		return &SnapshotSessionInfo{
			MirrorName: mirrorName,
			IsActive:   false,
		}, nil
	}

	return &SnapshotSessionInfo{
		MirrorName:   mirrorName,
		SnapshotName: session.SnapshotName,
		IsActive:     !session.closed,
	}, nil
}
