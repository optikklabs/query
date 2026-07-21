# optikk query

The DB/query/API layer of the Optikk observability backend. Serves the HTTP
`/api/v1` API (chi), reading ClickHouse (spans/logs/metrics) and MySQL
(users/teams/alerting). Owns the **MySQL** schema; its DDL lives in `db/mysql`
and is applied by hand (no migration-on-boot).

The ingestion path (OTLP gRPC + Kafka) lives in the separate
[`ingest`](https://github.com/optikklabs/ingest) repo, which owns the
**ClickHouse** schema. This service reads those CH tables but does not manage
them — apply ingest's `db/clickhouse` DDL by hand at least once so the tables
exist.

## Schema

The MySQL DDL lives in [`db/mysql`](db/mysql) as numbered `.sql` files and is
**applied by hand** — the service does not migrate on boot. Apply it (in lexical
order) at least once before starting query against a fresh database, or it will
fail against missing tables. The DDL is idempotent, so re-applying is safe:

```bash
for f in db/mysql/*.sql; do mysql < "$f"; done
```

The ClickHouse tables this service reads are owned by `ingest`; apply that
repo's `db/clickhouse` DDL as described in its README.

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
make run                                  # serves :19090 (apply db/mysql first)
```

Configuration is read from `config.yml` (env overrides via the `OPTIKK_` prefix).
`make run` supplies a local-only JWT secret when the environment does not set
`OPTIKK_AUTH_JWT_SECRET`; production must always provide its own secret.
