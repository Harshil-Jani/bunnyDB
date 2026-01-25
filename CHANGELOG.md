# Changelog

All notable changes to BunnyDB will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2026-01-25

### Added

- **CDC Replication** - Real-time streaming via PostgreSQL logical replication (WAL decoding)
- **Schema Sync** - On-demand DDL synchronization (columns, types, defaults, constraints)
- **Index Replication** - Automatic replication of all PostgreSQL index types (B-tree, Hash, GIN, GiST, SP-GiST, BRIN)
- **Foreign Key Handling** - Deferred FK strategy for batch consistency during CDC
- **Table-Level Resync** - Resync individual tables without stopping the mirror
- **Zero-Downtime Swap Resync** - Shadow table strategy with atomic rename for production safety
- **Pause/Resume** - Full lifecycle control over replication mirrors
- **On-Demand Retry (RetryNow)** - Bypass Temporal's backoff for immediate retry attempts
- **User Management & RBAC** - Admin and readonly roles with JWT authentication
- **Web UI** - Modern dashboard for managing peers, mirrors, and monitoring replication
- **Peer Management** - Register and test PostgreSQL connections
- **Mirror Management** - Create, monitor, and control replication jobs
- **Real-time Monitoring** - Live row counts, LSN tracking, batch IDs, and structured logs
- **Onboarding Tour** - Interactive walkthrough for new users
- **Docker Compose Setup** - One-command deployment with profiles for dev, docs, and production
- **Self-Hosting Guide** - Comprehensive documentation for running BunnyDB on your infrastructure
- **GitHub Pages** - Landing page and documentation at harshil-jani.github.io/bunnyDB

### Technical Stack

- **Backend**: Go with Temporal workflow orchestration
- **Frontend**: Next.js 14 with Tailwind CSS
- **Database**: PostgreSQL for catalog storage
- **Docs**: Nextra 3.3 with static export support

### Infrastructure

- Docker Compose with optional profiles (`dev`, `docs`)
- Makefile for common operations
- Environment-based configuration via `.env`
- Production-ready with configurable JWT secrets and credentials

---

[1.0.0]: https://github.com/Harshil-Jani/bunnyDB/releases/tag/v1.0.0
