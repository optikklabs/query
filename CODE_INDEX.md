# Query Service Code Index

Welcome to the `query` repository index! This service is the HTTP API backend for Optikk, serving the frontend (`web`), reading telemetry data from ClickHouse, and managing metadata (users, alerts, dashboards) in MySQL/MariaDB.

---

## 🚀 Entry Points & Setup
- **Main Entrypoint**: `cmd/query/main.go` (Initializes the server, configures DI, and applies MySQL migrations).
- **Configuration**: `config.yml` (App configuration, env overrides via `OPTIKK_`).
- **Dependencies**: `go.mod` / `go.sum`

---

## 🧠 Business Logic (`internal/modules/`)
This is where the core domain logic and HTTP handlers reside. When adding or modifying API endpoints, look here:

### Telemetry Domains (ClickHouse Reads)
- **`metrics/`**: Endpoints for metric aggregations, exploring series (`metrics/explorer`), and parsing metric filters (`metrics/filter`).
- **`logs/`**: Endpoints for searching and retrieving logs.
- **`traces/`**: Endpoints for fetching traces, spans, and waterfall data.

### Platform & Metadata Domains (MySQL Reads/Writes)
- **`user/`**: Authentication, JWT tokens, tenant management, user signup, and device linking. 
  - Submodules: `auth/`, `device/`, `signup/`, `tenant/`, `users/`, `shared/`
- **`alerting/`**: Alert configurations, rules, and evaluation schedules.
- **`dashboards/`**: User-defined dashboard JSON storage and retrieval.
- **`infrastructure/`**: Maps hosts, containers, and pods to their respective telemetry.
- **`saturation/`**: Calculates and serves subsystem health limits and Kafka/DB saturation metrics.
- **`services/`**: APM service catalog, tracking deployments and service-level KPIs.

---

## 🏗️ Infrastructure & Adapters (`internal/infra/`)
This layer handles external connections, databases, and cross-cutting concerns:
- **`database/`**: Contains adapters for ClickHouse (`clickhouse.go`) and MySQL (`mysql.go`), along with their OpenTelemetry instrumentations (`*_instrument.go`) and migration scripts (`migrate_mysql.go`).
- **`middleware/`**: HTTP middleware for Chi router (e.g., Auth verification, request logging).
- **`cursor/`**: Pagination cursor generation and parsing.
- **`timebucket/`**: Utilities for bucketizing time-series data for aggregations.
- **`token/`**: JWT generation and parsing logic.
- **`utils/`**: General shared Go utilities.

---

## 🎯 Quick Navigation
- **Adding a new endpoint?** Add it in the respective domain under `internal/modules/<domain>/` and wire it up to the Chi router.
- **Modifying database queries?** Check `internal/infra/database/` or the repository layer within your target module.
- **Checking DB migrations?** Look at `internal/infra/database/migrate_mysql.go`. Note: `query` ONLY migrates MySQL. ClickHouse migrations belong to `ingest`.
