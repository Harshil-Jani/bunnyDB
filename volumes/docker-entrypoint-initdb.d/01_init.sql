-- BunnyDB Catalog Initialization

-- Create schemas
CREATE SCHEMA IF NOT EXISTS bunny_internal;
CREATE SCHEMA IF NOT EXISTS bunny_stats;

-- Peers table: stores source and destination connection configs
CREATE TABLE IF NOT EXISTS bunny_internal.peers (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) UNIQUE NOT NULL,
    peer_type VARCHAR(50) NOT NULL DEFAULT 'POSTGRES',
    config JSONB NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Mirrors table: stores mirror configurations
CREATE TABLE IF NOT EXISTS bunny_internal.mirrors (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) UNIQUE NOT NULL,
    source_peer_id INT NOT NULL REFERENCES bunny_internal.peers(id),
    destination_peer_id INT NOT NULL REFERENCES bunny_internal.peers(id),
    config JSONB NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'CREATED',
    workflow_id VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Table mappings: source to destination table mappings per mirror
CREATE TABLE IF NOT EXISTS bunny_internal.table_mappings (
    id SERIAL PRIMARY KEY,
    mirror_id INT NOT NULL REFERENCES bunny_internal.mirrors(id) ON DELETE CASCADE,
    source_schema VARCHAR(255) NOT NULL,
    source_table VARCHAR(255) NOT NULL,
    destination_schema VARCHAR(255) NOT NULL,
    destination_table VARCHAR(255) NOT NULL,
    partition_key VARCHAR(255),
    exclude_columns TEXT[],
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(mirror_id, source_schema, source_table)
);

-- Table schema mapping: stores replicated table schemas
CREATE TABLE IF NOT EXISTS bunny_internal.table_schema_mapping (
    id SERIAL PRIMARY KEY,
    mirror_name VARCHAR(255) NOT NULL,
    table_name VARCHAR(512) NOT NULL,
    table_schema BYTEA NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(mirror_name, table_name)
);

-- Index definitions: tracks indexes to be replicated
CREATE TABLE IF NOT EXISTS bunny_internal.index_definitions (
    id SERIAL PRIMARY KEY,
    mirror_name VARCHAR(255) NOT NULL,
    table_name VARCHAR(512) NOT NULL,
    index_name VARCHAR(255) NOT NULL,
    index_definition TEXT NOT NULL,
    is_unique BOOLEAN DEFAULT FALSE,
    is_primary BOOLEAN DEFAULT FALSE,
    index_type VARCHAR(50) DEFAULT 'btree',
    replicated BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(mirror_name, table_name, index_name)
);

-- Foreign key definitions: tracks FKs to be replicated (deferred strategy)
CREATE TABLE IF NOT EXISTS bunny_internal.fk_definitions (
    id SERIAL PRIMARY KEY,
    mirror_name VARCHAR(255) NOT NULL,
    source_table VARCHAR(512) NOT NULL,
    constraint_name VARCHAR(255) NOT NULL,
    constraint_definition TEXT NOT NULL,
    target_table VARCHAR(512) NOT NULL,
    on_delete VARCHAR(50),
    on_update VARCHAR(50),
    is_deferrable BOOLEAN DEFAULT FALSE,
    initially_deferred BOOLEAN DEFAULT FALSE,
    dropped_at TIMESTAMP WITH TIME ZONE,  -- When FK was dropped for sync
    recreated_at TIMESTAMP WITH TIME ZONE, -- When FK was recreated after sync
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(mirror_name, source_table, constraint_name)
);

-- Mirror state: tracks CDC state per mirror
CREATE TABLE IF NOT EXISTS bunny_internal.mirror_state (
    id SERIAL PRIMARY KEY,
    mirror_name VARCHAR(255) UNIQUE NOT NULL,
    slot_name VARCHAR(255),
    publication_name VARCHAR(255),
    last_lsn BIGINT DEFAULT 0,
    last_sync_batch_id BIGINT DEFAULT 0,
    last_normalize_batch_id BIGINT DEFAULT 0,
    status VARCHAR(50) DEFAULT 'CREATED',
    error_message TEXT,
    error_count INT DEFAULT 0,
    last_error_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- CDC batches: tracks sync batches
CREATE TABLE IF NOT EXISTS bunny_stats.cdc_batches (
    id SERIAL PRIMARY KEY,
    mirror_name VARCHAR(255) NOT NULL,
    batch_id BIGINT NOT NULL,
    rows_in_batch INT NOT NULL DEFAULT 0,
    batch_start_lsn BIGINT NOT NULL,
    batch_end_lsn BIGINT NOT NULL,
    start_time TIMESTAMP WITH TIME ZONE NOT NULL,
    end_time TIMESTAMP WITH TIME ZONE,
    UNIQUE(mirror_name, batch_id)
);

-- Schema deltas audit log: tracks schema changes
CREATE TABLE IF NOT EXISTS bunny_stats.schema_deltas_audit_log (
    id BIGSERIAL PRIMARY KEY,
    mirror_name VARCHAR(255) NOT NULL,
    table_name VARCHAR(512) NOT NULL,
    delta_type VARCHAR(50) NOT NULL,  -- ADD_COLUMN, DROP_COLUMN, etc.
    delta_info JSONB NOT NULL,
    applied_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Table sync status: tracks per-table sync status for table-level resync
CREATE TABLE IF NOT EXISTS bunny_stats.table_sync_status (
    id SERIAL PRIMARY KEY,
    mirror_name VARCHAR(255) NOT NULL,
    table_name VARCHAR(512) NOT NULL,
    status VARCHAR(50) DEFAULT 'PENDING',  -- PENDING, SYNCING, SYNCED, RESYNCING, ERROR
    rows_synced BIGINT DEFAULT 0,
    last_synced_at TIMESTAMP WITH TIME ZONE,
    last_resync_requested_at TIMESTAMP WITH TIME ZONE,
    error_message TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(mirror_name, table_name)
);

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_table_mappings_mirror ON bunny_internal.table_mappings(mirror_id);
CREATE INDEX IF NOT EXISTS idx_cdc_batches_mirror ON bunny_stats.cdc_batches(mirror_name);
CREATE INDEX IF NOT EXISTS idx_schema_deltas_mirror ON bunny_stats.schema_deltas_audit_log(mirror_name);
CREATE INDEX IF NOT EXISTS idx_table_sync_status_mirror ON bunny_stats.table_sync_status(mirror_name);
CREATE INDEX IF NOT EXISTS idx_index_definitions_mirror ON bunny_internal.index_definitions(mirror_name, table_name);
CREATE INDEX IF NOT EXISTS idx_fk_definitions_mirror ON bunny_internal.fk_definitions(mirror_name, source_table);

-- Functions

-- Update timestamp trigger function
CREATE OR REPLACE FUNCTION bunny_internal.update_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Apply triggers
CREATE TRIGGER tr_peers_updated_at
    BEFORE UPDATE ON bunny_internal.peers
    FOR EACH ROW EXECUTE FUNCTION bunny_internal.update_updated_at();

CREATE TRIGGER tr_mirrors_updated_at
    BEFORE UPDATE ON bunny_internal.mirrors
    FOR EACH ROW EXECUTE FUNCTION bunny_internal.update_updated_at();

CREATE TRIGGER tr_mirror_state_updated_at
    BEFORE UPDATE ON bunny_internal.mirror_state
    FOR EACH ROW EXECUTE FUNCTION bunny_internal.update_updated_at();

CREATE TRIGGER tr_table_sync_status_updated_at
    BEFORE UPDATE ON bunny_stats.table_sync_status
    FOR EACH ROW EXECUTE FUNCTION bunny_internal.update_updated_at();

-- Grant permissions (for non-superuser access if needed)
GRANT ALL ON SCHEMA bunny_internal TO postgres;
GRANT ALL ON SCHEMA bunny_stats TO postgres;
GRANT ALL ON ALL TABLES IN SCHEMA bunny_internal TO postgres;
GRANT ALL ON ALL TABLES IN SCHEMA bunny_stats TO postgres;
GRANT ALL ON ALL SEQUENCES IN SCHEMA bunny_internal TO postgres;
GRANT ALL ON ALL SEQUENCES IN SCHEMA bunny_stats TO postgres;
