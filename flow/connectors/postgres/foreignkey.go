package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// ForeignKeyDefinition represents a PostgreSQL foreign key constraint
type ForeignKeyDefinition struct {
	Name              string
	SchemaName        string
	SourceTable       string
	SourceColumns     []string
	TargetSchema      string
	TargetTable       string
	TargetColumns     []string
	OnDelete          string // NO ACTION, CASCADE, SET NULL, SET DEFAULT, RESTRICT
	OnUpdate          string
	IsDeferrable      bool
	InitiallyDeferred bool
	Definition        string // Full constraint definition from pg_get_constraintdef
}

// GetForeignKeys returns all foreign keys for a given table
func (c *PostgresConnector) GetForeignKeys(ctx context.Context, schemaName, tableName string) ([]ForeignKeyDefinition, error) {
	query := `
		SELECT
			con.conname AS constraint_name,
			nsp.nspname AS schema_name,
			rel.relname AS table_name,
			pg_get_constraintdef(con.oid) AS constraint_definition,
			con.condeferrable AS is_deferrable,
			con.condeferred AS initially_deferred,
			fnsp.nspname AS target_schema,
			frel.relname AS target_table,
			CASE con.confdeltype
				WHEN 'a' THEN 'NO ACTION'
				WHEN 'r' THEN 'RESTRICT'
				WHEN 'c' THEN 'CASCADE'
				WHEN 'n' THEN 'SET NULL'
				WHEN 'd' THEN 'SET DEFAULT'
			END AS on_delete,
			CASE con.confupdtype
				WHEN 'a' THEN 'NO ACTION'
				WHEN 'r' THEN 'RESTRICT'
				WHEN 'c' THEN 'CASCADE'
				WHEN 'n' THEN 'SET NULL'
				WHEN 'd' THEN 'SET DEFAULT'
			END AS on_update
		FROM pg_constraint con
		JOIN pg_class rel ON rel.oid = con.conrelid
		JOIN pg_namespace nsp ON nsp.oid = rel.relnamespace
		JOIN pg_class frel ON frel.oid = con.confrelid
		JOIN pg_namespace fnsp ON fnsp.oid = frel.relnamespace
		WHERE con.contype = 'f'
			AND nsp.nspname = $1
			AND rel.relname = $2
		ORDER BY con.conname
	`

	rows, err := c.conn.Query(ctx, query, schemaName, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to query foreign keys: %w", err)
	}
	defer rows.Close()

	var fks []ForeignKeyDefinition
	for rows.Next() {
		var fk ForeignKeyDefinition

		if err := rows.Scan(
			&fk.Name,
			&fk.SchemaName,
			&fk.SourceTable,
			&fk.Definition,
			&fk.IsDeferrable,
			&fk.InitiallyDeferred,
			&fk.TargetSchema,
			&fk.TargetTable,
			&fk.OnDelete,
			&fk.OnUpdate,
		); err != nil {
			return nil, fmt.Errorf("failed to scan foreign key: %w", err)
		}

		// Get source columns
		srcCols, err := c.getFKColumns(ctx, schemaName, tableName, fk.Name, true)
		if err != nil {
			return nil, fmt.Errorf("failed to get FK source columns for %s: %w", fk.Name, err)
		}
		fk.SourceColumns = srcCols

		// Get target columns
		tgtCols, err := c.getFKColumns(ctx, schemaName, tableName, fk.Name, false)
		if err != nil {
			return nil, fmt.Errorf("failed to get FK target columns for %s: %w", fk.Name, err)
		}
		fk.TargetColumns = tgtCols

		fks = append(fks, fk)
	}

	return fks, rows.Err()
}

func (c *PostgresConnector) getFKColumns(ctx context.Context, schemaName, tableName, constraintName string, isSource bool) ([]string, error) {
	// Use conkey for source columns, confkey for target columns
	keyColumn := "conkey"
	relColumn := "conrelid"
	if !isSource {
		keyColumn = "confkey"
		relColumn = "confrelid"
	}

	query := fmt.Sprintf(`
		SELECT a.attname
		FROM pg_constraint con
		JOIN pg_class rel ON rel.oid = con.conrelid
		JOIN pg_namespace nsp ON nsp.oid = rel.relnamespace
		JOIN pg_attribute a ON a.attrelid = con.%s AND a.attnum = ANY(con.%s)
		WHERE con.contype = 'f'
			AND nsp.nspname = $1
			AND rel.relname = $2
			AND con.conname = $3
		ORDER BY array_position(con.%s, a.attnum::int2)
	`, relColumn, keyColumn, keyColumn)

	rows, err := c.conn.Query(ctx, query, schemaName, tableName, constraintName)
	if err != nil {
		return nil, err
	}

	return pgx.CollectRows(rows, pgx.RowTo[string])
}

// GetAllForeignKeysForTables returns foreign keys for multiple tables
func (c *PostgresConnector) GetAllForeignKeysForTables(ctx context.Context, tables []string) (map[string][]ForeignKeyDefinition, error) {
	result := make(map[string][]ForeignKeyDefinition)

	for _, table := range tables {
		parts := strings.SplitN(table, ".", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid table name: %s (expected schema.table)", table)
		}

		fks, err := c.GetForeignKeys(ctx, parts[0], parts[1])
		if err != nil {
			return nil, fmt.Errorf("failed to get foreign keys for %s: %w", table, err)
		}

		result[table] = fks
	}

	return result, nil
}

// DropForeignKey drops a foreign key constraint
func (c *PostgresConnector) DropForeignKey(ctx context.Context, schemaName, tableName, constraintName string) error {
	query := fmt.Sprintf("ALTER TABLE %s.%s DROP CONSTRAINT IF EXISTS %s",
		quoteIdentifier(schemaName),
		quoteIdentifier(tableName),
		quoteIdentifier(constraintName))

	c.logger.Info("dropping foreign key",
		"table", schemaName+"."+tableName,
		"constraint", constraintName)

	_, err := c.conn.Exec(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to drop FK %s: %w", constraintName, err)
	}

	return nil
}

// CreateForeignKey creates a foreign key constraint
func (c *PostgresConnector) CreateForeignKey(ctx context.Context, fk ForeignKeyDefinition, makeDeferrable bool) error {
	// Build the constraint definition
	srcCols := strings.Join(quoteIdentifiers(fk.SourceColumns), ", ")
	tgtCols := strings.Join(quoteIdentifiers(fk.TargetColumns), ", ")
	tgtTable := fmt.Sprintf("%s.%s", quoteIdentifier(fk.TargetSchema), quoteIdentifier(fk.TargetTable))

	query := fmt.Sprintf(
		"ALTER TABLE %s.%s ADD CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s (%s)",
		quoteIdentifier(fk.SchemaName),
		quoteIdentifier(fk.SourceTable),
		quoteIdentifier(fk.Name),
		srcCols,
		tgtTable,
		tgtCols,
	)

	// Add ON DELETE/UPDATE actions
	if fk.OnDelete != "" && fk.OnDelete != "NO ACTION" {
		query += " ON DELETE " + fk.OnDelete
	}
	if fk.OnUpdate != "" && fk.OnUpdate != "NO ACTION" {
		query += " ON UPDATE " + fk.OnUpdate
	}

	// Add DEFERRABLE clause
	if makeDeferrable || fk.IsDeferrable {
		query += " DEFERRABLE"
		if fk.InitiallyDeferred {
			query += " INITIALLY DEFERRED"
		} else {
			query += " INITIALLY IMMEDIATE"
		}
	}

	c.logger.Info("creating foreign key",
		"table", fk.SchemaName+"."+fk.SourceTable,
		"constraint", fk.Name,
		"deferrable", makeDeferrable || fk.IsDeferrable)

	_, err := c.conn.Exec(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to create FK %s: %w", fk.Name, err)
	}

	return nil
}

// CreateForeignKeyValidated creates a FK and validates existing data
func (c *PostgresConnector) CreateForeignKeyValidated(ctx context.Context, fk ForeignKeyDefinition, makeDeferrable bool) error {
	// First create as NOT VALID for faster creation
	srcCols := strings.Join(quoteIdentifiers(fk.SourceColumns), ", ")
	tgtCols := strings.Join(quoteIdentifiers(fk.TargetColumns), ", ")
	tgtTable := fmt.Sprintf("%s.%s", quoteIdentifier(fk.TargetSchema), quoteIdentifier(fk.TargetTable))

	query := fmt.Sprintf(
		"ALTER TABLE %s.%s ADD CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s (%s)",
		quoteIdentifier(fk.SchemaName),
		quoteIdentifier(fk.SourceTable),
		quoteIdentifier(fk.Name),
		srcCols,
		tgtTable,
		tgtCols,
	)

	if fk.OnDelete != "" && fk.OnDelete != "NO ACTION" {
		query += " ON DELETE " + fk.OnDelete
	}
	if fk.OnUpdate != "" && fk.OnUpdate != "NO ACTION" {
		query += " ON UPDATE " + fk.OnUpdate
	}

	if makeDeferrable || fk.IsDeferrable {
		query += " DEFERRABLE"
		if fk.InitiallyDeferred {
			query += " INITIALLY DEFERRED"
		}
	}

	query += " NOT VALID"

	c.logger.Info("creating foreign key (not valid)",
		"table", fk.SchemaName+"."+fk.SourceTable,
		"constraint", fk.Name)

	if _, err := c.conn.Exec(ctx, query); err != nil {
		return fmt.Errorf("failed to create FK %s: %w", fk.Name, err)
	}

	// Then validate (this can be done concurrently for each FK)
	validateQuery := fmt.Sprintf(
		"ALTER TABLE %s.%s VALIDATE CONSTRAINT %s",
		quoteIdentifier(fk.SchemaName),
		quoteIdentifier(fk.SourceTable),
		quoteIdentifier(fk.Name),
	)

	c.logger.Info("validating foreign key",
		"table", fk.SchemaName+"."+fk.SourceTable,
		"constraint", fk.Name)

	if _, err := c.conn.Exec(ctx, validateQuery); err != nil {
		return fmt.Errorf("failed to validate FK %s: %w", fk.Name, err)
	}

	return nil
}

// FKReplicator handles the deferred FK strategy
type FKReplicator struct {
	srcConn     *PostgresConnector
	dstConn     *PostgresConnector
	droppedFKs  map[string][]ForeignKeyDefinition // table -> dropped FKs
}

// NewFKReplicator creates a new FK replicator
func NewFKReplicator(srcConn, dstConn *PostgresConnector) *FKReplicator {
	return &FKReplicator{
		srcConn:    srcConn,
		dstConn:    dstConn,
		droppedFKs: make(map[string][]ForeignKeyDefinition),
	}
}

// DropFKsForSync drops all FKs on destination tables before sync
func (r *FKReplicator) DropFKsForSync(ctx context.Context, tables []string) error {
	for _, table := range tables {
		parts := strings.SplitN(table, ".", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid table name: %s", table)
		}
		schemaName, tableName := parts[0], parts[1]

		// Get FKs on destination
		fks, err := r.dstConn.GetForeignKeys(ctx, schemaName, tableName)
		if err != nil {
			return fmt.Errorf("failed to get FKs for %s: %w", table, err)
		}

		// Drop each FK and store for later recreation
		for _, fk := range fks {
			if err := r.dstConn.DropForeignKey(ctx, schemaName, tableName, fk.Name); err != nil {
				return fmt.Errorf("failed to drop FK %s: %w", fk.Name, err)
			}
		}

		r.droppedFKs[table] = fks
	}

	return nil
}

// RecreateFKsAfterSync recreates all FKs that were dropped, with validation
func (r *FKReplicator) RecreateFKsAfterSync(ctx context.Context, makeDeferrable bool) error {
	for table, fks := range r.droppedFKs {
		for _, fk := range fks {
			// Use source FK definition if available, otherwise use stored destination FK
			srcFKs, err := r.srcConn.GetForeignKeys(ctx, fk.SchemaName, fk.SourceTable)
			if err != nil {
				return fmt.Errorf("failed to get source FKs for %s: %w", table, err)
			}

			// Find matching FK from source
			var srcFK *ForeignKeyDefinition
			for _, sf := range srcFKs {
				if sf.Name == fk.Name {
					srcFK = &sf
					break
				}
			}

			fkToCreate := fk
			if srcFK != nil {
				fkToCreate = *srcFK
			}

			if err := r.dstConn.CreateForeignKeyValidated(ctx, fkToCreate, makeDeferrable); err != nil {
				return fmt.Errorf("failed to recreate FK %s: %w", fk.Name, err)
			}
		}
	}

	// Clear the dropped FKs tracking
	r.droppedFKs = make(map[string][]ForeignKeyDefinition)

	return nil
}

// ReplicateFKsFromSource replicates all FKs from source to destination
func (r *FKReplicator) ReplicateFKsFromSource(ctx context.Context, tables []string, makeDeferrable bool) error {
	for _, table := range tables {
		parts := strings.SplitN(table, ".", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid table name: %s", table)
		}
		schemaName, tableName := parts[0], parts[1]

		// Get source FKs
		srcFKs, err := r.srcConn.GetForeignKeys(ctx, schemaName, tableName)
		if err != nil {
			return fmt.Errorf("failed to get source FKs for %s: %w", table, err)
		}

		// Get destination FKs
		dstFKs, err := r.dstConn.GetForeignKeys(ctx, schemaName, tableName)
		if err != nil {
			return fmt.Errorf("failed to get destination FKs for %s: %w", table, err)
		}

		// Build map of existing destination FKs
		dstFKMap := make(map[string]ForeignKeyDefinition)
		for _, fk := range dstFKs {
			dstFKMap[fk.Name] = fk
		}

		// Create missing FKs
		for _, srcFK := range srcFKs {
			if _, exists := dstFKMap[srcFK.Name]; !exists {
				if err := r.dstConn.CreateForeignKeyValidated(ctx, srcFK, makeDeferrable); err != nil {
					return fmt.Errorf("failed to create FK %s: %w", srcFK.Name, err)
				}
			}
		}
	}

	return nil
}

// SetSessionReplicationRole sets the session replication role
// This can be used to disable FK checks entirely during batch operations
func (c *PostgresConnector) SetSessionReplicationRole(ctx context.Context, role string) error {
	// Valid roles: 'origin', 'replica', 'local'
	if role != "origin" && role != "replica" && role != "local" {
		return fmt.Errorf("invalid replication role: %s", role)
	}

	_, err := c.conn.Exec(ctx, fmt.Sprintf("SET session_replication_role = '%s'", role))
	return err
}

// SetConstraintsDeferred sets all constraints to deferred mode within a transaction
func (c *PostgresConnector) SetConstraintsDeferred(ctx context.Context) error {
	_, err := c.conn.Exec(ctx, "SET CONSTRAINTS ALL DEFERRED")
	return err
}

func quoteIdentifiers(names []string) []string {
	result := make([]string, len(names))
	for i, name := range names {
		result[i] = quoteIdentifier(name)
	}
	return result
}
