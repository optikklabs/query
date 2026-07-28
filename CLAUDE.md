# CLAUDE.md — query

Workspace-wide standards live in `../CLAUDE.md` and apply in full here.
This file covers only what is specific to query.

## What This Repo Owns

The HTTP API, business logic, metadata, querying, and authn/authz. Telemetry
writes belong to ingest; UI belongs to web.

## Layout

- `cmd/query/` — entrypoint; wiring in `internal/app/`.
- `internal/modules/<domain>/` — one vertical slice per domain (traces, logs,
  metrics, alerting, dashboards, user, ...), each flowing
  handler → service → repository. Handlers translate HTTP, services own
  business logic, repositories run queries. Nothing skips a layer.
- `internal/shared/<concern>/` — small single-purpose packages reused by
  multiple modules (`errorcode`, `httputil`, `contracts`, ...). A helper used
  by one module lives in that module — `internal/infra/utils/` is a dumping
  ground; do not add to it.
- SQL migrations live in `db/`.

## API Contract

- Error responses go through `internal/shared/errorcode` — typed codes mapped
  to HTTP status in the handler layer. No ad-hoc error shapes.
- DTOs are the contract; never return database models.

## ClickHouse Query Cost

Every endpoint may scan billions of rows. Cost control is architectural, and
it is the one performance concern that is always in scope:

- every query has a LIMIT and a time-range predicate
- no N+1 query loops; batch by design
- prefer direct column predicates (PREWHERE on LowCardinality columns) over
  clever CTEs — fingerprint-CTE "pruning" here has repeatedly pruned nothing
- no repeated identical queries per request

This does not license micro-optimization in Go code (workspace rule 3):
no request coalescing, caches, or pooling around the database client. If a
query is slow, fix the query or the schema.

## Failure Semantics

Dispatch and audit-write failures are counted and logged, not retried —
`AlertingAuditWriteFailures` is the pattern to copy. Alerting is periodic
pull; a failed evaluation cycle is retried by the next tick, not by code.
