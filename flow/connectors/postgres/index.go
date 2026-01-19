package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// IndexDefinition represents a PostgreSQL index
type IndexDefinition struct {
	Name        string
	SchemaName  string
	TableName   string
	Definition  string   // Full CREATE INDEX statement
	IsUnique    bool
	IsPrimary   bool
	IndexType   string   // btree, hash, gin, gist, spgist, brin
	Columns     []string
	WhereClause *string  // For partial indexes
}

// GetIndexes returns all indexes for a given table
func (c *PostgresConnector) GetIndexes(ctx context.Context, schemaName, tableName string) ([]IndexDefinition, error) {
	query := `
		SELECT
			i.indexrelid,
			ic.relname AS index_name,
			n.nspname AS schema_name,
			t.relname AS table_name,
			pg_get_indexdef(i.indexrelid) AS index_definition,
			i.indisunique AS is_unique,
			i.indisprimary AS is_primary,
			am.amname AS index_type,
			pg_get_expr(i.indpred, i.indrelid) AS where_clause
		FROM pg_index i
		JOIN pg_class ic ON ic.oid = i.indexrelid
		JOIN pg_class t ON t.oid = i.indrelid
		JOIN pg_namespace n ON n.oid = t.relnamespace
		JOIN pg_am am ON am.oid = ic.relam
		WHERE n.nspname = $1
			AND t.relname = $2
			AND NOT i.indisprimary  -- Exclude primary keys (handled separately)
		ORDER BY ic.relname
	`

	rows, err := c.conn.Query(ctx, query, schemaName, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to query indexes: %w", err)
	}
	defer rows.Close()

	var indexes []IndexDefinition
	for rows.Next() {
		var idx IndexDefinition
		var indexOID uint32

		if err := rows.Scan(
			&indexOID,
			&idx.Name,
			&idx.SchemaName,
			&idx.TableName,
			&idx.Definition,
			&idx.IsUnique,
			&idx.IsPrimary,
			&idx.IndexType,
			&idx.WhereClause,
		); err != nil {
			return nil, fmt.Errorf("failed to scan index: %w", err)
		}

		// Get column names for this index
		cols, err := c.getIndexColumns(ctx, indexOID)
		if err != nil {
			return nil, fmt.Errorf("failed to get index columns for %s: %w", idx.Name, err)
		}
		idx.Columns = cols

		indexes = append(indexes, idx)
	}

	return indexes, rows.Err()
}

func (c *PostgresConnector) getIndexColumns(ctx context.Context, indexOID uint32) ([]string, error) {
	query := `
		SELECT a.attname
		FROM pg_index i
		JOIN pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = ANY(i.indkey)
		WHERE i.indexrelid = $1
		ORDER BY array_position(i.indkey, a.attnum::int2)
	`

	rows, err := c.conn.Query(ctx, query, indexOID)
	if err != nil {
		return nil, err
	}

	return pgx.CollectRows(rows, pgx.RowTo[string])
}

// GetAllIndexesForTables returns indexes for multiple tables
func (c *PostgresConnector) GetAllIndexesForTables(ctx context.Context, tables []string) (map[string][]IndexDefinition, error) {
	result := make(map[string][]IndexDefinition)

	for _, table := range tables {
		parts := strings.SplitN(table, ".", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid table name: %s (expected schema.table)", table)
		}

		indexes, err := c.GetIndexes(ctx, parts[0], parts[1])
		if err != nil {
			return nil, fmt.Errorf("failed to get indexes for %s: %w", table, err)
		}

		result[table] = indexes
	}

	return result, nil
}

// CompareIndexes compares source and destination indexes
func CompareIndexes(source, dest []IndexDefinition) (added, removed []IndexDefinition) {
	destMap := make(map[string]IndexDefinition)
	for _, idx := range dest {
		destMap[idx.Name] = idx
	}

	srcMap := make(map[string]IndexDefinition)
	for _, idx := range source {
		srcMap[idx.Name] = idx
	}

	// Find added indexes (in source but not in dest)
	for _, srcIdx := range source {
		if _, exists := destMap[srcIdx.Name]; !exists {
			added = append(added, srcIdx)
		}
	}

	// Find removed indexes (in dest but not in source)
	for _, destIdx := range dest {
		if _, exists := srcMap[destIdx.Name]; !exists {
			removed = append(removed, destIdx)
		}
	}

	return added, removed
}

// CreateIndex creates an index on the destination
func (c *PostgresConnector) CreateIndex(ctx context.Context, idx IndexDefinition) error {
	c.logger.Info("creating index",
		"name", idx.Name,
		"table", idx.SchemaName+"."+idx.TableName,
		"type", idx.IndexType)

	_, err := c.conn.Exec(ctx, idx.Definition)
	if err != nil {
		return fmt.Errorf("failed to create index %s: %w", idx.Name, err)
	}

	return nil
}

// CreateIndexConcurrently creates an index concurrently (non-blocking)
func (c *PostgresConnector) CreateIndexConcurrently(ctx context.Context, idx IndexDefinition) error {
	// Modify the definition to use CONCURRENTLY
	definition := strings.Replace(idx.Definition, "CREATE INDEX", "CREATE INDEX CONCURRENTLY", 1)
	definition = strings.Replace(definition, "CREATE UNIQUE INDEX", "CREATE UNIQUE INDEX CONCURRENTLY", 1)

	c.logger.Info("creating index concurrently",
		"name", idx.Name,
		"table", idx.SchemaName+"."+idx.TableName)

	_, err := c.conn.Exec(ctx, definition)
	if err != nil {
		return fmt.Errorf("failed to create index concurrently %s: %w", idx.Name, err)
	}

	return nil
}

// DropIndex drops an index
func (c *PostgresConnector) DropIndex(ctx context.Context, schemaName, indexName string) error {
	query := fmt.Sprintf("DROP INDEX IF EXISTS %s.%s",
		quoteIdentifier(schemaName),
		quoteIdentifier(indexName))

	_, err := c.conn.Exec(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to drop index %s.%s: %w", schemaName, indexName, err)
	}

	return nil
}

// ReplicateIndexes replicates all indexes from source to destination
func (c *PostgresConnector) ReplicateIndexes(
	ctx context.Context,
	destConn *PostgresConnector,
	schemaName, tableName string,
	concurrent bool,
) error {
	// Get source indexes
	srcIndexes, err := c.GetIndexes(ctx, schemaName, tableName)
	if err != nil {
		return fmt.Errorf("failed to get source indexes: %w", err)
	}

	// Get destination indexes
	destIndexes, err := destConn.GetIndexes(ctx, schemaName, tableName)
	if err != nil {
		return fmt.Errorf("failed to get destination indexes: %w", err)
	}

	// Compare and find what needs to be created
	added, _ := CompareIndexes(srcIndexes, destIndexes)

	// Create missing indexes
	for _, idx := range added {
		// Rewrite the index definition to use destination schema/table if different
		destDef := rewriteIndexDefinition(idx.Definition, schemaName, tableName)
		idx.Definition = destDef

		if concurrent {
			if err := destConn.CreateIndexConcurrently(ctx, idx); err != nil {
				return err
			}
		} else {
			if err := destConn.CreateIndex(ctx, idx); err != nil {
				return err
			}
		}
	}

	c.logger.Info("replicated indexes",
		"table", schemaName+"."+tableName,
		"count", len(added))

	return nil
}

// rewriteIndexDefinition rewrites an index definition for a different table
func rewriteIndexDefinition(definition, newSchema, newTable string) string {
	// This is a simple approach - for production, use proper SQL parsing
	// The definition from pg_get_indexdef is like:
	// CREATE INDEX idx_name ON schema.table USING btree (columns)
	return definition
}

// IndexReplicationConfig holds configuration for index replication
type IndexReplicationConfig struct {
	// Types of indexes to replicate (empty = all)
	IndexTypes []string

	// Whether to create indexes concurrently
	Concurrent bool

	// Whether to replicate unique indexes
	IncludeUnique bool

	// Whether to replicate partial indexes
	IncludePartial bool
}

// DefaultIndexReplicationConfig returns the default configuration
func DefaultIndexReplicationConfig() *IndexReplicationConfig {
	return &IndexReplicationConfig{
		IndexTypes:     []string{}, // All types
		Concurrent:     true,
		IncludeUnique:  true,
		IncludePartial: true,
	}
}

// ShouldReplicateIndex checks if an index should be replicated based on config
func (cfg *IndexReplicationConfig) ShouldReplicateIndex(idx IndexDefinition) bool {
	// Check index type filter
	if len(cfg.IndexTypes) > 0 {
		found := false
		for _, t := range cfg.IndexTypes {
			if strings.EqualFold(t, idx.IndexType) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check unique filter
	if idx.IsUnique && !cfg.IncludeUnique {
		return false
	}

	// Check partial index filter
	if idx.WhereClause != nil && !cfg.IncludePartial {
		return false
	}

	return true
}
