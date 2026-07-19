# optikk query

The DB/query/API layer of the Optikk observability backend. Serves the HTTP
`/api/v1` API (chi), reading ClickHouse (spans/logs/metrics) and MySQL
(users/teams/alerting). Owns the **MySQL** schema and migrates it on startup.

The ingestion path (OTLP gRPC + Kafka) lives in the separate
[`ingest`](https://github.com/optikklabs/ingest) repo, which owns and migrates
the **ClickHouse** schema. This service reads those CH tables but does not
migrate them — bring up `ingest` (or apply its CH migrations) at least once so
the tables exist.

## Commands

- **Build**: `make build` or `go build ./cmd/query`
- **Run**: `make run`
- **Test**: `go test ./...`
- **Format / Vet**: `make fmt` / `make vet`

## Local development

Infra (ClickHouse + MariaDB) is provided by the `ingest` repo's compose stack,
which is a superset and binds the same host ports — bring that up instead of a
local compose file here:

```bash
(cd ../ingest && docker compose up -d)   # ClickHouse + MariaDB (+ Kafka/Grafana)
make run                                  # applies MySQL migrations, serves :19090
```

Configuration is read from `config.yml` (env overrides via the `OPTIKK_` prefix).
`make run` supplies a local-only JWT secret when the environment does not set
`OPTIKK_AUTH_JWT_SECRET`; production must always provide its own secret.
