package postgres

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pglogrepl"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgproto3"
)

// CDCRecord represents a change data capture record
type CDCRecord struct {
	Operation   string            // INSERT, UPDATE, DELETE
	Schema      string
	Table       string
	LSN         int64
	Columns     []string
	OldValues   map[string]interface{} // For UPDATE/DELETE
	NewValues   map[string]interface{} // For INSERT/UPDATE
	CommitTime  time.Time
}

// RelationInfo stores relation metadata received from pgoutput
type RelationInfo struct {
	RelationID uint32
	Schema     string
	Table      string
	Columns    []ColumnInfo
}

// ColumnInfo stores column metadata
type ColumnInfo struct {
	Name     string
	TypeOID  uint32
	Modifier int32
}

// CDCReader reads CDC records from the replication stream
type CDCReader struct {
	conn           *PostgresConnector
	relations      map[uint32]*RelationInfo
	logger         *slog.Logger
	standbyTimeout time.Duration
	lastStatusTime time.Time
	clientXLogPos  pglogrepl.LSN
}

// NewCDCReader creates a new CDC reader
func NewCDCReader(conn *PostgresConnector) *CDCReader {
	return &CDCReader{
		conn:           conn,
		relations:      make(map[uint32]*RelationInfo),
		logger:         slog.Default().With(slog.String("component", "cdc-reader")),
		standbyTimeout: 10 * time.Second,
	}
}

// PullRecords reads records from the replication stream until timeout or maxRecords
func (r *CDCReader) PullRecords(ctx context.Context, maxRecords int, timeout time.Duration) ([]*CDCRecord, int64, error) {
	if r.conn.replConn == nil {
		return nil, 0, fmt.Errorf("replication connection not set up")
	}

	records := make([]*CDCRecord, 0, maxRecords)
	deadline := time.Now().Add(timeout)
	var lastLSN int64

	for len(records) < maxRecords && time.Now().Before(deadline) {
		// Check context
		select {
		case <-ctx.Done():
			return records, lastLSN, ctx.Err()
		default:
		}

		// Send standby status if needed
		if err := r.sendStandbyStatusIfNeeded(ctx); err != nil {
			r.logger.Warn("failed to send standby status", slog.Any("error", err))
		}

		// Calculate receive timeout
		receiveTimeout := time.Until(deadline)
		if receiveTimeout > r.standbyTimeout {
			receiveTimeout = r.standbyTimeout
		}
		if receiveTimeout < 0 {
			break
		}

		// Receive message with timeout
		receiveCtx, cancel := context.WithTimeout(ctx, receiveTimeout)
		rawMsg, err := r.conn.replConn.PgConn().ReceiveMessage(receiveCtx)
		cancel()

		if err != nil {
			if pgconn.Timeout(err) {
				continue // Timeout is normal, keep trying
			}
			if ctx.Err() != nil {
				return records, lastLSN, ctx.Err()
			}
			return records, lastLSN, fmt.Errorf("failed to receive message: %w", err)
		}

		// Process message
		if errMsg, ok := rawMsg.(*pgproto3.ErrorResponse); ok {
			return records, lastLSN, fmt.Errorf("postgres error: %s", errMsg.Message)
		}

		msg, ok := rawMsg.(*pgproto3.CopyData)
		if !ok {
			continue
		}

		switch msg.Data[0] {
		case pglogrepl.PrimaryKeepaliveMessageByteID:
			pkm, err := pglogrepl.ParsePrimaryKeepaliveMessage(msg.Data[1:])
			if err != nil {
				r.logger.Warn("failed to parse keepalive", slog.Any("error", err))
				continue
			}
			if pkm.ReplyRequested {
				r.lastStatusTime = time.Time{} // Force status update
			}

		case pglogrepl.XLogDataByteID:
			xld, err := pglogrepl.ParseXLogData(msg.Data[1:])
			if err != nil {
				r.logger.Warn("failed to parse xlog data", slog.Any("error", err))
				continue
			}

			// Parse the pgoutput message
			rec, err := r.parseXLogData(xld)
			if err != nil {
				r.logger.Warn("failed to parse xlog message", slog.Any("error", err))
				continue
			}

			if rec != nil {
				records = append(records, rec)
				lastLSN = rec.LSN
			}

			// Update position
			r.clientXLogPos = xld.WALStart + pglogrepl.LSN(len(xld.WALData))
		}
	}

	return records, lastLSN, nil
}

// sendStandbyStatusIfNeeded sends status update to postgres
func (r *CDCReader) sendStandbyStatusIfNeeded(ctx context.Context) error {
	if time.Since(r.lastStatusTime) < r.standbyTimeout {
		return nil
	}

	err := pglogrepl.SendStandbyStatusUpdate(
		ctx,
		r.conn.replConn.PgConn(),
		pglogrepl.StandbyStatusUpdate{
			WALWritePosition: r.clientXLogPos,
			WALFlushPosition: r.clientXLogPos,
			WALApplyPosition: r.clientXLogPos,
			ClientTime:       time.Now(),
			ReplyRequested:   false,
		},
	)
	if err != nil {
		return err
	}

	r.lastStatusTime = time.Now()
	return nil
}

// parseXLogData parses a pgoutput message
func (r *CDCReader) parseXLogData(xld pglogrepl.XLogData) (*CDCRecord, error) {
	if len(xld.WALData) == 0 {
		return nil, nil
	}

	msgType := xld.WALData[0]

	switch msgType {
	case 'R': // Relation
		rel, err := r.parseRelationMessage(xld.WALData[1:])
		if err != nil {
			return nil, err
		}
		r.relations[rel.RelationID] = rel
		return nil, nil // Relation messages don't produce records

	case 'I': // Insert
		return r.parseInsertMessage(xld)

	case 'U': // Update
		return r.parseUpdateMessage(xld)

	case 'D': // Delete
		return r.parseDeleteMessage(xld)

	case 'B': // Begin
		return nil, nil

	case 'C': // Commit
		return nil, nil

	case 'O': // Origin
		return nil, nil

	case 'T': // Truncate
		r.logger.Info("received truncate message")
		return nil, nil

	default:
		// Unknown message type, skip
		return nil, nil
	}
}

// parseRelationMessage parses a relation message
func (r *CDCReader) parseRelationMessage(data []byte) (*RelationInfo, error) {
	// Format: RelationID (4) | Namespace (string) | RelationName (string) | ReplicaIdentity (1) | NumColumns (2) | Columns...
	if len(data) < 4 {
		return nil, fmt.Errorf("relation message too short")
	}

	rel := &RelationInfo{}
	offset := 0

	// RelationID (4 bytes, big endian)
	rel.RelationID = uint32(data[0])<<24 | uint32(data[1])<<16 | uint32(data[2])<<8 | uint32(data[3])
	offset += 4

	// Namespace (null-terminated string)
	end := offset
	for end < len(data) && data[end] != 0 {
		end++
	}
	rel.Schema = string(data[offset:end])
	offset = end + 1

	// RelationName (null-terminated string)
	end = offset
	for end < len(data) && data[end] != 0 {
		end++
	}
	rel.Table = string(data[offset:end])
	offset = end + 1

	// ReplicaIdentity (1 byte)
	offset++

	// NumColumns (2 bytes, big endian)
	if offset+2 > len(data) {
		return rel, nil
	}
	numCols := int(data[offset])<<8 | int(data[offset+1])
	offset += 2

	// Parse columns
	rel.Columns = make([]ColumnInfo, 0, numCols)
	for i := 0; i < numCols && offset < len(data); i++ {
		col := ColumnInfo{}

		// Flags (1 byte)
		offset++

		// Column name (null-terminated string)
		end = offset
		for end < len(data) && data[end] != 0 {
			end++
		}
		col.Name = string(data[offset:end])
		offset = end + 1

		// TypeOID (4 bytes)
		if offset+4 <= len(data) {
			col.TypeOID = uint32(data[offset])<<24 | uint32(data[offset+1])<<16 | uint32(data[offset+2])<<8 | uint32(data[offset+3])
			offset += 4
		}

		// Type modifier (4 bytes)
		if offset+4 <= len(data) {
			col.Modifier = int32(uint32(data[offset])<<24 | uint32(data[offset+1])<<16 | uint32(data[offset+2])<<8 | uint32(data[offset+3]))
			offset += 4
		}

		rel.Columns = append(rel.Columns, col)
	}

	r.logger.Debug("parsed relation",
		slog.Uint64("id", uint64(rel.RelationID)),
		slog.String("schema", rel.Schema),
		slog.String("table", rel.Table),
		slog.Int("columns", len(rel.Columns)))

	return rel, nil
}

// parseInsertMessage parses an insert message
func (r *CDCReader) parseInsertMessage(xld pglogrepl.XLogData) (*CDCRecord, error) {
	data := xld.WALData[1:] // Skip message type byte
	if len(data) < 5 {
		return nil, fmt.Errorf("insert message too short")
	}

	// RelationID (4 bytes)
	relID := uint32(data[0])<<24 | uint32(data[1])<<16 | uint32(data[2])<<8 | uint32(data[3])
	data = data[4:]

	rel, ok := r.relations[relID]
	if !ok {
		return nil, fmt.Errorf("unknown relation ID: %d", relID)
	}

	// 'N' byte for new tuple
	if len(data) < 1 || data[0] != 'N' {
		return nil, fmt.Errorf("expected 'N' for new tuple, got %c", data[0])
	}
	data = data[1:]

	values, err := r.parseTupleData(data, rel.Columns)
	if err != nil {
		return nil, fmt.Errorf("failed to parse tuple: %w", err)
	}

	return &CDCRecord{
		Operation: "INSERT",
		Schema:    rel.Schema,
		Table:     rel.Table,
		LSN:       int64(xld.WALStart),
		Columns:   getColumnNames(rel.Columns),
		NewValues: values,
	}, nil
}

// parseUpdateMessage parses an update message
func (r *CDCReader) parseUpdateMessage(xld pglogrepl.XLogData) (*CDCRecord, error) {
	data := xld.WALData[1:] // Skip message type byte
	if len(data) < 5 {
		return nil, fmt.Errorf("update message too short")
	}

	// RelationID (4 bytes)
	relID := uint32(data[0])<<24 | uint32(data[1])<<16 | uint32(data[2])<<8 | uint32(data[3])
	data = data[4:]

	rel, ok := r.relations[relID]
	if !ok {
		return nil, fmt.Errorf("unknown relation ID: %d", relID)
	}

	rec := &CDCRecord{
		Operation: "UPDATE",
		Schema:    rel.Schema,
		Table:     rel.Table,
		LSN:       int64(xld.WALStart),
		Columns:   getColumnNames(rel.Columns),
	}

	// Check for old tuple (K or O)
	if len(data) > 0 && (data[0] == 'K' || data[0] == 'O') {
		data = data[1:]
		oldValues, remaining, err := r.parseTupleDataWithRemaining(data, rel.Columns)
		if err != nil {
			return nil, fmt.Errorf("failed to parse old tuple: %w", err)
		}
		rec.OldValues = oldValues
		data = remaining
	}

	// New tuple ('N')
	if len(data) < 1 || data[0] != 'N' {
		return nil, fmt.Errorf("expected 'N' for new tuple in update, got %c", data[0])
	}
	data = data[1:]

	newValues, err := r.parseTupleData(data, rel.Columns)
	if err != nil {
		return nil, fmt.Errorf("failed to parse new tuple: %w", err)
	}
	rec.NewValues = newValues

	return rec, nil
}

// parseDeleteMessage parses a delete message
func (r *CDCReader) parseDeleteMessage(xld pglogrepl.XLogData) (*CDCRecord, error) {
	data := xld.WALData[1:] // Skip message type byte
	if len(data) < 5 {
		return nil, fmt.Errorf("delete message too short")
	}

	// RelationID (4 bytes)
	relID := uint32(data[0])<<24 | uint32(data[1])<<16 | uint32(data[2])<<8 | uint32(data[3])
	data = data[4:]

	rel, ok := r.relations[relID]
	if !ok {
		return nil, fmt.Errorf("unknown relation ID: %d", relID)
	}

	rec := &CDCRecord{
		Operation: "DELETE",
		Schema:    rel.Schema,
		Table:     rel.Table,
		LSN:       int64(xld.WALStart),
		Columns:   getColumnNames(rel.Columns),
	}

	// Old tuple ('K' for key or 'O' for old)
	if len(data) < 1 {
		return nil, fmt.Errorf("delete message missing tuple type")
	}
	if data[0] != 'K' && data[0] != 'O' {
		return nil, fmt.Errorf("expected 'K' or 'O' for old tuple in delete, got %c", data[0])
	}
	data = data[1:]

	oldValues, err := r.parseTupleData(data, rel.Columns)
	if err != nil {
		return nil, fmt.Errorf("failed to parse old tuple: %w", err)
	}
	rec.OldValues = oldValues

	return rec, nil
}

// parseTupleData parses tuple data into a map
func (r *CDCReader) parseTupleData(data []byte, columns []ColumnInfo) (map[string]interface{}, error) {
	values, _, err := r.parseTupleDataWithRemaining(data, columns)
	return values, err
}

// parseTupleDataWithRemaining parses tuple data and returns remaining bytes
func (r *CDCReader) parseTupleDataWithRemaining(data []byte, columns []ColumnInfo) (map[string]interface{}, []byte, error) {
	if len(data) < 2 {
		return nil, data, fmt.Errorf("tuple data too short")
	}

	// Number of columns (2 bytes)
	numCols := int(data[0])<<8 | int(data[1])
	data = data[2:]

	values := make(map[string]interface{}, numCols)

	for i := 0; i < numCols && i < len(columns) && len(data) > 0; i++ {
		colType := data[0]
		data = data[1:]

		switch colType {
		case 'n': // NULL
			values[columns[i].Name] = nil

		case 'u': // TOAST unchanged
			// Skip - value not changed
			values[columns[i].Name] = nil

		case 't': // Text value
			if len(data) < 4 {
				return values, data, fmt.Errorf("text value length too short")
			}
			valLen := int(data[0])<<24 | int(data[1])<<16 | int(data[2])<<8 | int(data[3])
			data = data[4:]

			if len(data) < valLen {
				return values, data, fmt.Errorf("text value data too short")
			}
			values[columns[i].Name] = string(data[:valLen])
			data = data[valLen:]

		case 'b': // Binary value
			if len(data) < 4 {
				return values, data, fmt.Errorf("binary value length too short")
			}
			valLen := int(data[0])<<24 | int(data[1])<<16 | int(data[2])<<8 | int(data[3])
			data = data[4:]

			if len(data) < valLen {
				return values, data, fmt.Errorf("binary value data too short")
			}
			values[columns[i].Name] = data[:valLen]
			data = data[valLen:]

		default:
			return values, data, fmt.Errorf("unknown column type: %c", colType)
		}
	}

	return values, data, nil
}

func getColumnNames(cols []ColumnInfo) []string {
	names := make([]string, len(cols))
	for i, col := range cols {
		names[i] = col.Name
	}
	return names
}

// ApplyRecord applies a CDC record to the destination database
func ApplyRecord(ctx context.Context, destConn *PostgresConnector, rec *CDCRecord, pkColumns []string) error {
	switch rec.Operation {
	case "INSERT":
		return applyInsert(ctx, destConn, rec)
	case "UPDATE":
		return applyUpdate(ctx, destConn, rec, pkColumns)
	case "DELETE":
		return applyDelete(ctx, destConn, rec, pkColumns)
	default:
		return fmt.Errorf("unknown operation: %s", rec.Operation)
	}
}

// applyInsert applies an INSERT record
func applyInsert(ctx context.Context, destConn *PostgresConnector, rec *CDCRecord) error {
	if len(rec.NewValues) == 0 {
		return nil
	}

	columns := make([]string, 0, len(rec.NewValues))
	placeholders := make([]string, 0, len(rec.NewValues))
	values := make([]interface{}, 0, len(rec.NewValues))

	i := 1
	for col, val := range rec.NewValues {
		columns = append(columns, quoteIdentifier(col))
		placeholders = append(placeholders, fmt.Sprintf("$%d", i))
		values = append(values, val)
		i++
	}

	query := fmt.Sprintf(
		"INSERT INTO %s.%s (%s) VALUES (%s) ON CONFLICT DO NOTHING",
		quoteIdentifier(rec.Schema),
		quoteIdentifier(rec.Table),
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "),
	)

	_, err := destConn.conn.Exec(ctx, query, values...)
	return err
}

// applyUpdate applies an UPDATE record
func applyUpdate(ctx context.Context, destConn *PostgresConnector, rec *CDCRecord, pkColumns []string) error {
	if len(rec.NewValues) == 0 {
		return nil
	}

	// Build SET clause
	setClauses := make([]string, 0, len(rec.NewValues))
	values := make([]interface{}, 0, len(rec.NewValues)+len(pkColumns))
	paramIdx := 1

	for col, val := range rec.NewValues {
		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", quoteIdentifier(col), paramIdx))
		values = append(values, val)
		paramIdx++
	}

	// Build WHERE clause using PK columns
	whereClauses := make([]string, 0, len(pkColumns))
	// Use old values if available, otherwise use new values for PK
	sourceValues := rec.OldValues
	if sourceValues == nil {
		sourceValues = rec.NewValues
	}

	for _, pk := range pkColumns {
		if val, ok := sourceValues[pk]; ok {
			whereClauses = append(whereClauses, fmt.Sprintf("%s = $%d", quoteIdentifier(pk), paramIdx))
			values = append(values, val)
			paramIdx++
		}
	}

	if len(whereClauses) == 0 {
		return fmt.Errorf("no primary key columns found for UPDATE")
	}

	query := fmt.Sprintf(
		"UPDATE %s.%s SET %s WHERE %s",
		quoteIdentifier(rec.Schema),
		quoteIdentifier(rec.Table),
		strings.Join(setClauses, ", "),
		strings.Join(whereClauses, " AND "),
	)

	_, err := destConn.conn.Exec(ctx, query, values...)
	return err
}

// applyDelete applies a DELETE record
func applyDelete(ctx context.Context, destConn *PostgresConnector, rec *CDCRecord, pkColumns []string) error {
	if rec.OldValues == nil {
		return fmt.Errorf("DELETE record has no old values")
	}

	// Build WHERE clause using PK columns
	whereClauses := make([]string, 0, len(pkColumns))
	values := make([]interface{}, 0, len(pkColumns))
	paramIdx := 1

	for _, pk := range pkColumns {
		if val, ok := rec.OldValues[pk]; ok {
			whereClauses = append(whereClauses, fmt.Sprintf("%s = $%d", quoteIdentifier(pk), paramIdx))
			values = append(values, val)
			paramIdx++
		}
	}

	if len(whereClauses) == 0 {
		return fmt.Errorf("no primary key columns found for DELETE")
	}

	query := fmt.Sprintf(
		"DELETE FROM %s.%s WHERE %s",
		quoteIdentifier(rec.Schema),
		quoteIdentifier(rec.Table),
		strings.Join(whereClauses, " AND "),
	)

	_, err := destConn.conn.Exec(ctx, query, values...)
	return err
}

// ReplConn returns the replication connection
func (c *PostgresConnector) ReplConn() *pgx.Conn {
	return c.replConn
}

// UpdateLastOffset updates the last processed LSN
func (c *PostgresConnector) UpdateLastOffset(lsn int64) {
	if c.replState != nil {
		c.replState.LastOffset.Store(lsn)
	}
}

// GetLastOffset returns the last processed LSN
func (c *PostgresConnector) GetLastOffset() int64 {
	if c.replState != nil {
		return c.replState.LastOffset.Load()
	}
	return 0
}
