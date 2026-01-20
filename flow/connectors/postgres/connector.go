package postgres

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jackc/pglogrepl"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/bunnydb/bunnydb/flow/shared"
)

// PostgresConnector represents a connection to a PostgreSQL database
type PostgresConnector struct {
	logger            *slog.Logger
	conn              *pgx.Conn
	replConn          *pgx.Conn
	replState         *ReplState
	config            *PostgresConfig
	customTypeMapping map[uint32]shared.CustomDataType
	typeMap           *pgtype.Map
	replLock          sync.Mutex
	metadataSchema    string
	pgVersion         shared.PGVersion
}

// PostgresConfig holds the configuration for a PostgreSQL connection
type PostgresConfig struct {
	Host           string
	Port           int
	User           string
	Password       string
	Database       string
	SSLMode        string
	MetadataSchema string
	RequireTLS     bool
	RootCA         []byte
	TLSHost        string
}

// ReplState holds the replication state
type ReplState struct {
	Slot        string
	Publication string
	Offset      int64
	LastOffset  atomic.Int64
}

// NewPostgresConnector creates a new PostgreSQL connector
func NewPostgresConnector(ctx context.Context, config *PostgresConfig) (*PostgresConnector, error) {
	logger := slog.Default().With(slog.String("component", "postgres-connector"))

	// Debug log the incoming config
	logger.Info("NewPostgresConnector called",
		slog.String("host", config.Host),
		slog.Int("port", config.Port),
		slog.String("user", config.User),
		slog.String("database", config.Database),
		slog.String("sslMode", config.SSLMode))

	// Determine SSL mode
	sslMode := config.SSLMode
	if sslMode == "" {
		if config.RequireTLS {
			sslMode = "require"
		} else {
			sslMode = "disable"
		}
	}

	// Build connection string - only include password if it's set
	var connString string
	if config.Password != "" {
		connString = fmt.Sprintf(
			"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
			config.Host, config.Port, config.User, config.Password, config.Database, sslMode,
		)
	} else {
		connString = fmt.Sprintf(
			"host=%s port=%d user=%s dbname=%s sslmode=%s",
			config.Host, config.Port, config.User, config.Database, sslMode,
		)
	}
	logger.Info("connection string built", slog.String("dbname", config.Database), slog.String("connString", connString))

	connConfig, err := pgx.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse connection string: %w", err)
	}

	connConfig.RuntimeParams["timezone"] = "UTC"
	connConfig.RuntimeParams["DateStyle"] = "ISO, DMY"

	conn, err := pgx.ConnectConfig(ctx, connConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to postgres: %w", err)
	}

	metadataSchema := "_bunny_internal"
	if config.MetadataSchema != "" {
		metadataSchema = config.MetadataSchema
	}

	return &PostgresConnector{
		logger:            logger,
		conn:              conn,
		config:            config,
		metadataSchema:    metadataSchema,
		typeMap:           pgtype.NewMap(),
		customTypeMapping: make(map[uint32]shared.CustomDataType),
	}, nil
}

// Close closes all connections
func (c *PostgresConnector) Close() error {
	var errs []error
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if c.conn != nil {
		if err := c.conn.Close(ctx); err != nil {
			errs = append(errs, fmt.Errorf("failed to close connection: %w", err))
		}
	}

	if c.replConn != nil {
		if err := c.replConn.Close(ctx); err != nil {
			errs = append(errs, fmt.Errorf("failed to close replication connection: %w", err))
		}
	}

	return errors.Join(errs...)
}

// Conn returns the underlying connection
func (c *PostgresConnector) Conn() *pgx.Conn {
	return c.conn
}

// ConnectionActive checks if the connection is active
func (c *PostgresConnector) ConnectionActive(ctx context.Context) error {
	if c.conn == nil {
		return errors.New("connection is nil")
	}
	_, err := c.conn.Exec(ctx, "SELECT 1")
	return err
}

// SetupReplConn creates a replication connection
func (c *PostgresConnector) SetupReplConn(ctx context.Context) error {
	// Determine SSL mode
	sslMode := c.config.SSLMode
	if sslMode == "" {
		if c.config.RequireTLS {
			sslMode = "require"
		} else {
			sslMode = "disable"
		}
	}

	// Build connection string - only include password if it's set
	var connString string
	if c.config.Password != "" {
		connString = fmt.Sprintf(
			"host=%s port=%d user=%s password=%s dbname=%s replication=database sslmode=%s",
			c.config.Host, c.config.Port, c.config.User, c.config.Password, c.config.Database, sslMode,
		)
	} else {
		connString = fmt.Sprintf(
			"host=%s port=%d user=%s dbname=%s replication=database sslmode=%s",
			c.config.Host, c.config.Port, c.config.User, c.config.Database, sslMode,
		)
	}

	connConfig, err := pgx.ParseConfig(connString)
	if err != nil {
		return fmt.Errorf("failed to parse replication connection string: %w", err)
	}

	connConfig.RuntimeParams["timezone"] = "UTC"
	connConfig.RuntimeParams["bytea_output"] = "hex"
	connConfig.RuntimeParams["DateStyle"] = "ISO, DMY"
	connConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol

	conn, err := pgx.ConnectConfig(ctx, connConfig)
	if err != nil {
		return fmt.Errorf("failed to create replication connection: %w", err)
	}

	c.replConn = conn
	return nil
}

// StartReplication starts logical replication
func (c *PostgresConnector) StartReplication(
	ctx context.Context,
	slotName string,
	publicationName string,
	lastOffset int64,
) error {
	if c.replConn == nil {
		return errors.New("replication connection not set up")
	}

	pluginArgs := []string{
		"proto_version '1'",
		fmt.Sprintf("publication_names '%s'", publicationName),
	}

	// Add messages support for PG14+
	if c.pgVersion >= shared.POSTGRES_14 {
		pluginArgs = append(pluginArgs, "messages 'true'")
	}

	opts := pglogrepl.StartReplicationOptions{
		PluginArgs: pluginArgs,
	}

	var startLSN pglogrepl.LSN
	if lastOffset > 0 {
		startLSN = pglogrepl.LSN(lastOffset + 1)
	}

	c.replLock.Lock()
	defer c.replLock.Unlock()

	if err := pglogrepl.StartReplication(
		ctx, c.replConn.PgConn(), slotName, startLSN, opts,
	); err != nil {
		return fmt.Errorf("failed to start replication: %w", err)
	}

	c.replState = &ReplState{
		Slot:        slotName,
		Publication: publicationName,
		Offset:      lastOffset,
	}
	c.replState.LastOffset.Store(lastOffset)

	c.logger.Info("started replication",
		slog.String("slot", slotName),
		slog.Int64("startLSN", int64(startLSN)))

	return nil
}

// GetPGVersion returns the PostgreSQL version
func (c *PostgresConnector) GetPGVersion(ctx context.Context) (shared.PGVersion, error) {
	if c.pgVersion > 0 {
		return c.pgVersion, nil
	}

	var version int
	err := c.conn.QueryRow(ctx, "SHOW server_version_num").Scan(&version)
	if err != nil {
		return 0, fmt.Errorf("failed to get postgres version: %w", err)
	}

	c.pgVersion = shared.PGVersion(version)
	return c.pgVersion, nil
}

// CreatePublication creates a publication for the given tables
func (c *PostgresConnector) CreatePublication(
	ctx context.Context,
	publicationName string,
	tables []string,
) error {
	// Check if publication exists
	var exists bool
	err := c.conn.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM pg_publication WHERE pubname = $1)",
		publicationName,
	).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check publication existence: %w", err)
	}

	if exists {
		c.logger.Info("publication already exists", slog.String("name", publicationName))
		return nil
	}

	// Build table list
	tableList := ""
	for i, t := range tables {
		if i > 0 {
			tableList += ", "
		}
		tableList += t
	}

	query := fmt.Sprintf("CREATE PUBLICATION \"%s\" FOR TABLE %s", publicationName, tableList)
	_, err = c.conn.Exec(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to create publication: %w", err)
	}

	c.logger.Info("created publication", slog.String("name", publicationName))
	return nil
}

// CreateReplicationSlot creates a replication slot
func (c *PostgresConnector) CreateReplicationSlot(
	ctx context.Context,
	slotName string,
) (string, error) {
	if c.replConn == nil {
		return "", errors.New("replication connection not set up")
	}

	// Check if slot exists
	var exists bool
	err := c.conn.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM pg_replication_slots WHERE slot_name = $1)",
		slotName,
	).Scan(&exists)
	if err != nil {
		return "", fmt.Errorf("failed to check slot existence: %w", err)
	}

	if exists {
		c.logger.Info("replication slot already exists", slog.String("name", slotName))
		// Get snapshot name from existing slot (won't have one, return empty)
		return "", nil
	}

	c.replLock.Lock()
	defer c.replLock.Unlock()

	result, err := pglogrepl.CreateReplicationSlot(
		ctx,
		c.replConn.PgConn(),
		slotName,
		"pgoutput",
		pglogrepl.CreateReplicationSlotOptions{
			Temporary:      false,
			SnapshotAction: "EXPORT_SNAPSHOT",
		},
	)
	if err != nil {
		return "", fmt.Errorf("failed to create replication slot: %w", err)
	}

	c.logger.Info("created replication slot",
		slog.String("name", slotName),
		slog.String("snapshot", result.SnapshotName))

	return result.SnapshotName, nil
}

// DropReplicationSlot drops a replication slot, terminating any active connection first
func (c *PostgresConnector) DropReplicationSlot(ctx context.Context, slotName string) error {
	// First, terminate any active connection using this slot
	_, err := c.conn.Exec(ctx, `
		SELECT pg_terminate_backend(active_pid)
		FROM pg_replication_slots
		WHERE slot_name = $1 AND active_pid IS NOT NULL
	`, slotName)
	if err != nil {
		c.logger.Warn("failed to terminate active slot connection", slog.String("slot", slotName), slog.Any("error", err))
		// Continue anyway - the slot might not be active
	}

	// Small delay to allow connection to fully terminate
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(500 * time.Millisecond):
	}

	// Now drop the slot
	_, err = c.conn.Exec(ctx,
		"SELECT pg_drop_replication_slot($1)",
		slotName,
	)
	if err != nil {
		return fmt.Errorf("failed to drop replication slot: %w", err)
	}
	return nil
}

// DropPublication drops a publication
func (c *PostgresConnector) DropPublication(ctx context.Context, publicationName string) error {
	_, err := c.conn.Exec(ctx,
		fmt.Sprintf("DROP PUBLICATION IF EXISTS \"%s\"", publicationName),
	)
	if err != nil {
		return fmt.Errorf("failed to drop publication: %w", err)
	}
	return nil
}
