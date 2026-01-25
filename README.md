<p align="center">
  <img src="ui/public/favicon.svg" width="80" alt="BunnyDB" />
</p>

<h1 align="center">BunnyDB</h1>

<p align="center">
  <strong>Fast, focused PostgreSQL-to-PostgreSQL CDC replication.</strong>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/version-v1.0.0-brightgreen" alt="Version 1.0.0" />
  <img src="https://img.shields.io/badge/deployment-self--hosted-blue" alt="Self-Hosted" />
  <img src="https://img.shields.io/badge/cloud-coming%20soon-yellow" alt="Cloud Coming Soon" />
  <img src="https://img.shields.io/badge/license-Apache%202.0-green" alt="License" />
</p>

<p align="center">
  <a href="https://harshil-jani.github.io/bunnyDB/docs">Documentation</a> &middot;
  <a href="https://harshil-jani.github.io/bunnyDB/docs/quickstart">Quickstart</a> &middot;
  <a href="https://harshil-jani.github.io/bunnyDB/docs/self-hosting">Self-Hosting</a> &middot;
  <a href="https://harshil-jani.github.io/bunnyDB/docs/api-reference">API Reference</a>
</p>

---

## What is BunnyDB?

BunnyDB is a self-hosted PostgreSQL-to-PostgreSQL replication tool built on Change Data Capture (CDC). It handles the hard parts of database replication - schema changes, indexes, foreign keys, and table-level resyncs - so you don't have to.

> **Self-Hosted**: Run on your EC2 instances, local servers, or any Docker host. You own your data, your infrastructure, your uptime.
>
> **Cloud Coming Soon**: We're running a pilot to validate demand. If this tool is useful to you, a managed cloud version will follow.

## Key Features

- **CDC Replication** - Real-time streaming via PostgreSQL logical replication
- **Schema Sync** - On-demand DDL sync (columns, types, defaults, constraints)
- **Index Replication** - All index types: B-tree, Hash, GIN, GiST, SP-GiST, BRIN
- **Foreign Key Handling** - Deferred FK strategy for batch consistency
- **Table-Level Resync** - Resync individual tables without mirror restart
- **Zero-Downtime Swap Resync** - Shadow table + atomic rename for production safety
- **Pause / Resume** - Full control over replication lifecycle
- **On-Demand Retry** - Skip Temporal's backoff circuit for immediate retries
- **User Management & RBAC** - Admin and viewer roles with JWT auth
- **Web UI** - Monitor mirrors, manage peers, control replication visually

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         BunnyDB Stack                           │
├─────────────────────────────────────────────────────────────────┤
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌───────────────┐  │
│  │ bunny-ui │  │ bunny-api│  │ temporal │  │ temporal-ui   │  │
│  │  :3000   │  │  :8112   │  │  :7233   │  │  :8085        │  │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  └───────────────┘  │
│       │              │              │                            │
│       └──────────────┼──────────────┘                            │
│                      │                                           │
│  ┌───────────────────┴────────────────────┐                     │
│  │            bunny-worker                │                     │
│  │   (Temporal Worker + CDC Engine)       │                     │
│  └───────────────────┬────────────────────┘                     │
│                      │                                           │
│  ┌───────────────────┴────────────────────┐                     │
│  │              catalog                    │                     │
│  │          (PostgreSQL :5432)             │                     │
│  └─────────────────────────────────────────┘                     │
└─────────────────────────────────────────────────────────────────┘
```

## Installation

### Prerequisites

- Docker & Docker Compose
- A PostgreSQL source database with `wal_level = logical`

### Quick Start (Core)

```bash
git clone https://github.com/Harshil-Jani/bunnyDB.git
cd bunnyDB

# First-time setup: create .env from template
make setup

# Edit .env with your production values (change passwords!)

# Start BunnyDB
make up
```

This starts the core services: catalog, temporal, API, worker, and UI.

> **Tip**: Use `make help` to see all available commands.

- **UI**: http://localhost:3000
- **API**: http://localhost:8112
- **Temporal UI**: http://localhost:8085

Default login: `admin` / `admin`

### Full Setup (with local docs)

```bash
make docs
```

Adds a local documentation server at http://localhost:3001.

### Development Setup (with test databases)

```bash
make dev
```

Adds source and destination test databases for development/testing.

### All Services

```bash
make all
```

For complete self-hosting instructions including production security, reverse proxy setup, and backups, see the [Self-Hosting Guide](https://harshil-jani.github.io/bunnyDB/docs/self-hosting).

## Quick Usage

```bash
# 1. Login
TOKEN=$(curl -s -X POST http://localhost:8112/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin"}' | jq -r '.token')

# 2. Create source peer
curl -X POST http://localhost:8112/v1/peers \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my_source",
    "host": "host.docker.internal",
    "port": 5432,
    "user": "postgres",
    "password": "your_password",
    "database": "source_db"
  }'

# 3. Create destination peer
curl -X POST http://localhost:8112/v1/peers \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my_dest",
    "host": "host.docker.internal",
    "port": 5433,
    "user": "postgres",
    "password": "your_password",
    "database": "dest_db"
  }'

# 4. Create mirror
curl -X POST http://localhost:8112/v1/mirrors \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my_mirror",
    "source_peer": "my_source",
    "destination_peer": "my_dest",
    "do_initial_snapshot": true,
    "table_mappings": [
      {"source_table": "public.users", "destination_table": "public.users"}
    ]
  }'

# 5. Check status
curl http://localhost:8112/v1/mirrors/my_mirror \
  -H "Authorization: Bearer $TOKEN" | jq '.status'
```

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/v1/auth/login` | Authenticate and get JWT token |
| POST | `/v1/peers` | Create a peer connection |
| GET | `/v1/peers/:name/tables` | List tables on a peer |
| POST | `/v1/mirrors` | Create a new mirror |
| GET | `/v1/mirrors/:name` | Get mirror status |
| POST | `/v1/mirrors/:name/pause` | Pause replication |
| POST | `/v1/mirrors/:name/resume` | Resume replication |
| POST | `/v1/mirrors/:name/resync` | Full mirror resync |
| POST | `/v1/mirrors/:name/resync/:table` | Table-level resync |
| POST | `/v1/mirrors/:name/retry` | Immediate retry (skip backoff) |
| POST | `/v1/mirrors/:name/sync-schema` | Apply schema changes |
| DELETE | `/v1/mirrors/:name` | Drop mirror |

For complete API documentation, see the [API Reference](https://harshil-jani.github.io/bunnyDB/docs/api-reference).

## Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `BUNNY_CATALOG_HOST` | Catalog database host | `catalog` |
| `BUNNY_CATALOG_PORT` | Catalog database port | `5432` |
| `BUNNY_CATALOG_USER` | Catalog database user | `postgres` |
| `BUNNY_CATALOG_PASSWORD` | Catalog database password | `bunnydb` |
| `BUNNY_CATALOG_DATABASE` | Catalog database name | `bunnydb` |
| `TEMPORAL_HOST_PORT` | Temporal server address | `temporal:7233` |
| `BUNNY_JWT_SECRET` | JWT signing secret | `change-me-in-production` |
| `BUNNY_ADMIN_USER` | Default admin username | `admin` |
| `BUNNY_ADMIN_PASSWORD` | Default admin password | `admin` |

For full configuration details, see [Configuration docs](https://harshil-jani.github.io/bunnyDB/docs/configuration).

## Source Database Requirements

Your source PostgreSQL must have:

```sql
-- postgresql.conf
wal_level = logical
max_replication_slots = 4    -- at least 1 per mirror
max_wal_senders = 4          -- at least 1 per mirror
```

The connecting user needs `REPLICATION` privilege or superuser access.

## Project Structure

```
bunnyDB/
├── flow/           # Go backend (API server + Temporal worker)
├── ui/             # Next.js web UI
├── docs/           # Nextra documentation site
├── landing/        # Static landing page (for GitHub Pages)
├── volumes/        # Docker volume configs
└── docker-compose.yml
```

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feat/my-feature`)
3. Run the dev setup: `docker compose --profile dev up -d`
4. Make changes and test
5. Submit a pull request

See [Contributing Guide](https://harshil-jani.github.io/bunnyDB/docs/contributing) for details.

## License

Apache 2.0 - Derived from [PeerDB](https://github.com/PeerDB-io/peerdb) (ELv2)
