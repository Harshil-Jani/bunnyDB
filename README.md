# BunnyDB

**Fast, focused PostgreSQL-to-PostgreSQL replication with enhanced schema, index, and foreign key handling.**

BunnyDB is derived from [PeerDB](https://github.com/PeerDB-io/peerdb) with a specialized focus on Postgres-to-Postgres replication, adding features like:

- **Schema Replication**: User-configurable DDL sync (columns, types, defaults)
- **Index Replication**: Automatically replicate all index types
- **Foreign Key Handling**: Deferred FK strategy for consistent replication
- **Table-Level Resync**: Resync individual tables without full mirror restart
- **On-Demand Retry**: Bypass Temporal's backoff circuit for immediate retries

## Quick Start

```bash
# Clone and start
git clone <repo>
cd bunnyDB

# Start all services
docker-compose up -d

# Check status
docker-compose ps
```

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        BunnyDB Stack                            │
├─────────────────────────────────────────────────────────────────┤
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────────────┐ │
│  │ bunny-ui │  │ bunny-api│  │ temporal │  │ temporal-ui      │ │
│  │  :3000   │  │  :8112   │  │  :7233   │  │  :8085           │ │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  └──────────────────┘ │
│       │             │             │                              │
│       └─────────────┼─────────────┘                              │
│                     │                                            │
│  ┌──────────────────┴───────────────────┐                       │
│  │           bunny-worker               │                       │
│  │  (Temporal Worker + CDC Engine)      │                       │
│  └──────────────────┬───────────────────┘                       │
│                     │                                            │
│  ┌──────────────────┴───────────────────┐                       │
│  │             catalog                   │                       │
│  │         (PostgreSQL :5432)            │                       │
│  └───────────────────────────────────────┘                       │
└─────────────────────────────────────────────────────────────────┘
```

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/v1/mirrors` | Create a new mirror |
| GET | `/v1/mirrors/:name` | Get mirror status |
| POST | `/v1/mirrors/:name/pause` | Pause mirror |
| POST | `/v1/mirrors/:name/resume` | Resume mirror |
| DELETE | `/v1/mirrors/:name` | Drop mirror |
| POST | `/v1/mirrors/:name/resync` | Full resync |
| POST | `/v1/mirrors/:name/resync/:table` | Table-level resync |
| POST | `/v1/mirrors/:name/retry` | On-demand retry (skip backoff) |
| GET | `/v1/mirrors/:name/schema-diff` | Show pending schema changes |
| POST | `/v1/mirrors/:name/sync-schema` | Apply schema/index/FK changes |

## Key Features

### 1. Index Replication
Automatically replicates all indexes from source to destination:
- B-tree, Hash, GIN, GiST, SP-GiST, BRIN
- Unique constraints
- Partial indexes
- Expression indexes

### 2. Foreign Key Handling (Deferred Strategy)
1. **Initial Sync**: FKs dropped before sync, recreated after
2. **CDC**: Uses `DEFERRABLE INITIALLY DEFERRED` for batch consistency
3. **Validation**: FKs validated when recreated

### 3. Table-Level Resync
Resync individual tables without disrupting the entire mirror:
```bash
curl -X POST http://localhost:8112/v1/mirrors/my_mirror/resync/public.users
```

### 4. On-Demand Retry
Skip Temporal's exponential backoff for immediate retry:
```bash
curl -X POST http://localhost:8112/v1/mirrors/my_mirror/retry
```

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `BUNNY_CATALOG_HOST` | Catalog database host | `catalog` |
| `BUNNY_CATALOG_PORT` | Catalog database port | `5432` |
| `BUNNY_CATALOG_USER` | Catalog database user | `postgres` |
| `BUNNY_CATALOG_PASSWORD` | Catalog database password | `bunnydb` |
| `TEMPORAL_HOST_PORT` | Temporal server address | `temporal:7233` |

## Development

```bash
# Generate protobuf
./scripts/generate-protos.sh

# Build locally
cd flow && go build -o bunny-worker ./cmd/worker
cd flow && go build -o bunny-api ./cmd/api

# Run tests
cd flow && go test ./...
```

## License

Apache 2.0 - Derived from PeerDB (ELv2)
