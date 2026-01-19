package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// TableSchema represents a table's schema
type TableSchema struct {
	SchemaName            string
	TableName             string
	Columns               []ColumnDefinition
	PrimaryKeyColumns     []string
	IsReplicaIdentityFull bool
}

// ColumnDefinition represents a column definition
type ColumnDefinition struct {
	Name         string
	Type         string
	TypeOID      uint32
	TypeModifier int32
	Nullable     bool
	DefaultValue *string
	IsPrimaryKey bool
}

// GetTableSchema returns the schema for a given table
func (c *PostgresConnector) GetTableSchema(ctx context.Context, schemaName, tableName string) (*TableSchema, error) {
	// Get columns
	columns, err := c.getColumns(ctx, schemaName, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}

	// Get primary key columns
	pkCols, err := c.getPrimaryKeyColumns(ctx, schemaName, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to get primary key columns: %w", err)
	}

	// Mark primary key columns
	pkSet := make(map[string]bool)
	for _, col := range pkCols {
		pkSet[col] = true
	}
	for i := range columns {
		if pkSet[columns[i].Name] {
			columns[i].IsPrimaryKey = true
		}
	}

	// Get replica identity
	isFullReplica, err := c.isReplicaIdentityFull(ctx, schemaName, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to get replica identity: %w", err)
	}

	return &TableSchema{
		SchemaName:            schemaName,
		TableName:             tableName,
		Columns:               columns,
		PrimaryKeyColumns:     pkCols,
		IsReplicaIdentityFull: isFullReplica,
	}, nil
}

func (c *PostgresConnector) getColumns(ctx context.Context, schemaName, tableName string) ([]ColumnDefinition, error) {
	query := `
		SELECT
			a.attname AS column_name,
			a.atttypid AS type_oid,
			format_type(a.atttypid, a.atttypmod) AS data_type,
			a.atttypmod AS type_modifier,
			NOT a.attnotnull AS nullable,
			pg_get_expr(d.adbin, d.adrelid) AS default_value
		FROM pg_attribute a
		JOIN pg_class c ON a.attrelid = c.oid
		JOIN pg_namespace n ON c.relnamespace = n.oid
		LEFT JOIN pg_attrdef d ON a.attrelid = d.adrelid AND a.attnum = d.adnum
		WHERE n.nspname = $1
			AND c.relname = $2
			AND a.attnum > 0
			AND NOT a.attisdropped
		ORDER BY a.attnum
	`

	rows, err := c.conn.Query(ctx, query, schemaName, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to query columns: %w", err)
	}
	defer rows.Close()

	var columns []ColumnDefinition
	for rows.Next() {
		var col ColumnDefinition
		var defaultValue *string

		if err := rows.Scan(
			&col.Name,
			&col.TypeOID,
			&col.Type,
			&col.TypeModifier,
			&col.Nullable,
			&defaultValue,
		); err != nil {
			return nil, fmt.Errorf("failed to scan column: %w", err)
		}

		col.DefaultValue = defaultValue
		columns = append(columns, col)
	}

	return columns, rows.Err()
}

func (c *PostgresConnector) getPrimaryKeyColumns(ctx context.Context, schemaName, tableName string) ([]string, error) {
	query := `
		SELECT a.attname
		FROM pg_index i
		JOIN pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = ANY(i.indkey)
		JOIN pg_class c ON c.oid = i.indrelid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = $1
			AND c.relname = $2
			AND i.indisprimary
		ORDER BY array_position(i.indkey, a.attnum::int2)
	`

	rows, err := c.conn.Query(ctx, query, schemaName, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to query primary key: %w", err)
	}

	return pgx.CollectRows(rows, pgx.RowTo[string])
}

func (c *PostgresConnector) isReplicaIdentityFull(ctx context.Context, schemaName, tableName string) (bool, error) {
	var relreplident string
	err := c.conn.QueryRow(ctx, `
		SELECT c.relreplident::text
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = $1 AND c.relname = $2
	`, schemaName, tableName).Scan(&relreplident)
	if err != nil {
		return false, err
	}

	return relreplident == "f", nil
}

// GetAllTables returns all tables in the database
func (c *PostgresConnector) GetAllTables(ctx context.Context) ([]string, error) {
	query := `
		SELECT n.nspname || '.' || c.relname AS schema_table
		FROM pg_class c
		JOIN pg_namespace n ON c.relnamespace = n.oid
		WHERE n.nspname !~ '^pg_'
			AND n.nspname <> 'information_schema'
			AND c.relkind IN ('r', 'p')
		ORDER BY schema_table
	`

	rows, err := c.conn.Query(ctx, query)
	if err != nil {
		return nil, err
	}

	return pgx.CollectRows(rows, pgx.RowTo[string])
}

// GetTablesInSchema returns all tables in a schema
func (c *PostgresConnector) GetTablesInSchema(ctx context.Context, schema string) ([]string, error) {
	query := `
		SELECT c.relname
		FROM pg_class c
		JOIN pg_namespace n ON c.relnamespace = n.oid
		WHERE n.nspname = $1
			AND c.relkind IN ('r', 'p')
		ORDER BY c.relname
	`

	rows, err := c.conn.Query(ctx, query, schema)
	if err != nil {
		return nil, err
	}

	return pgx.CollectRows(rows, pgx.RowTo[string])
}

// GetTableOID returns the OID of a table
func (c *PostgresConnector) GetTableOID(ctx context.Context, schemaName, tableName string) (uint32, error) {
	var oid uint32
	err := c.conn.QueryRow(ctx, `
		SELECT c.oid
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = $1 AND c.relname = $2
	`, schemaName, tableName).Scan(&oid)
	return oid, err
}

// CompareSchemas compares source and destination schemas and returns the differences
func CompareSchemas(source, dest *TableSchema) *SchemaDelta {
	delta := &SchemaDelta{
		SourceTable:      source.SchemaName + "." + source.TableName,
		DestinationTable: dest.SchemaName + "." + dest.TableName,
	}

	// Build maps for comparison
	destCols := make(map[string]ColumnDefinition)
	for _, col := range dest.Columns {
		destCols[col.Name] = col
	}

	srcCols := make(map[string]ColumnDefinition)
	for _, col := range source.Columns {
		srcCols[col.Name] = col
	}

	// Find added and modified columns
	for _, srcCol := range source.Columns {
		if destCol, exists := destCols[srcCol.Name]; !exists {
			delta.AddedColumns = append(delta.AddedColumns, srcCol)
		} else if srcCol.Type != destCol.Type {
			delta.TypeChanges = append(delta.TypeChanges, ColumnTypeChange{
				ColumnName: srcCol.Name,
				OldType:    destCol.Type,
				NewType:    srcCol.Type,
			})
		}
	}

	// Find dropped columns
	for _, destCol := range dest.Columns {
		if _, exists := srcCols[destCol.Name]; !exists {
			delta.DroppedColumns = append(delta.DroppedColumns, destCol.Name)
		}
	}

	return delta
}

// SchemaDelta represents the differences between two schemas
type SchemaDelta struct {
	SourceTable      string
	DestinationTable string

	AddedColumns   []ColumnDefinition
	DroppedColumns []string
	TypeChanges    []ColumnTypeChange

	AddedIndexes   []IndexDefinition
	DroppedIndexes []string

	AddedFKs   []ForeignKeyDefinition
	DroppedFKs []string
}

// ColumnTypeChange represents a column type change
type ColumnTypeChange struct {
	ColumnName string
	OldType    string
	NewType    string
}

// HasChanges returns true if there are any schema changes
func (d *SchemaDelta) HasChanges() bool {
	return len(d.AddedColumns) > 0 ||
		len(d.DroppedColumns) > 0 ||
		len(d.TypeChanges) > 0 ||
		len(d.AddedIndexes) > 0 ||
		len(d.DroppedIndexes) > 0 ||
		len(d.AddedFKs) > 0 ||
		len(d.DroppedFKs) > 0
}

// ApplySchemaDelta applies schema changes to the destination
func (c *PostgresConnector) ApplySchemaDelta(ctx context.Context, delta *SchemaDelta) error {
	// Apply column additions
	for _, col := range delta.AddedColumns {
		query := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s",
			delta.DestinationTable,
			quoteIdentifier(col.Name),
			col.Type,
		)

		if !col.Nullable {
			query += " NOT NULL"
		}
		if col.DefaultValue != nil {
			query += " DEFAULT " + *col.DefaultValue
		}

		if _, err := c.conn.Exec(ctx, query); err != nil {
			return fmt.Errorf("failed to add column %s: %w", col.Name, err)
		}
		c.logger.Info("added column", "table", delta.DestinationTable, "column", col.Name)
	}

	// Apply index additions
	for _, idx := range delta.AddedIndexes {
		if _, err := c.conn.Exec(ctx, idx.Definition); err != nil {
			return fmt.Errorf("failed to create index %s: %w", idx.Name, err)
		}
		c.logger.Info("created index", "name", idx.Name)
	}

	// Apply FK additions (deferred)
	for _, fk := range delta.AddedFKs {
		if _, err := c.conn.Exec(ctx, fk.Definition); err != nil {
			return fmt.Errorf("failed to create FK %s: %w", fk.Name, err)
		}
		c.logger.Info("created foreign key", "name", fk.Name)
	}

	return nil
}

func quoteIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// CreateTableFromSchema creates a table in the destination database based on source schema
func (c *PostgresConnector) CreateTableFromSchema(ctx context.Context, schema *TableSchema, destSchema, destTable string) error {
	// Build column definitions
	var columnDefs []string
	for _, col := range schema.Columns {
		colDef := fmt.Sprintf("%s %s", quoteIdentifier(col.Name), col.Type)

		if !col.Nullable {
			colDef += " NOT NULL"
		}
		if col.DefaultValue != nil && !strings.Contains(*col.DefaultValue, "nextval(") {
			// Skip auto-increment defaults, they need the sequence
			colDef += " DEFAULT " + *col.DefaultValue
		}

		columnDefs = append(columnDefs, colDef)
	}

	// Add primary key constraint if present
	if len(schema.PrimaryKeyColumns) > 0 {
		pkCols := make([]string, len(schema.PrimaryKeyColumns))
		for i, col := range schema.PrimaryKeyColumns {
			pkCols[i] = quoteIdentifier(col)
		}
		columnDefs = append(columnDefs, fmt.Sprintf("PRIMARY KEY (%s)", strings.Join(pkCols, ", ")))
	}

	// Build CREATE TABLE statement
	query := fmt.Sprintf(
		"CREATE TABLE IF NOT EXISTS %s.%s (\n  %s\n)",
		quoteIdentifier(destSchema),
		quoteIdentifier(destTable),
		strings.Join(columnDefs, ",\n  "),
	)

	c.logger.Info("creating table", "schema", destSchema, "table", destTable)

	if _, err := c.conn.Exec(ctx, query); err != nil {
		return fmt.Errorf("failed to create table %s.%s: %w", destSchema, destTable, err)
	}

	c.logger.Info("table created successfully", "schema", destSchema, "table", destTable)
	return nil
}

// EnsureSchemaExists creates the schema if it doesn't exist
func (c *PostgresConnector) EnsureSchemaExists(ctx context.Context, schemaName string) error {
	query := fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", quoteIdentifier(schemaName))
	_, err := c.conn.Exec(ctx, query)
	return err
}
