# CLAUDE.md — query

Conventions for the query repo, the read API of Optikk. The ingest and web
services are separate repositories; this file covers only query. See
README.md for running it.

## What This Repo Owns

The HTTP `/api/v1` API (chi), reading ClickHouse (owned by ingest) and
MySQL (owned here, DDL in `db/`, applied by hand). Background workers: the
alert evaluator and billing.

## Layout

- `cmd/query` — `main`: config, signals, `app.New(cfg).Start(ctx)`.
- `internal/app` — wiring only: infra, module manifest, router, lifecycle.
- `internal/modules/<area>/<module>` — one package per API surface,
  exposing `NewModule(...)` with `Name()` and `RegisterRoutes(chi.Router)`
  (plus `Start`/`Stop` for background runners). Inside, a request flows
  handler → service → repository. A small module keeps them in one package
  (`handler.go`, `service.go`, `repository.go`, `dto.go`); a large one
  splits them into `handler/`, `service/`, `repository/`, `models/`.
  Modules never import each other's internals; shared code moves to
  `internal/shared`.
- `internal/infra` — clients and cross-cutting middleware (database,
  token, cursor, secretbox, email, metrics).
- `internal/shared` — helpers reused by several modules: `httputil`
  (responses, params), `chargs` (ClickHouse args), `errorcode`,
  `contracts` (response envelope), `filterutil`, `spanstats`.

## API Rules

- Handlers parse and validate with `httputil` (`BindJSON`, `QueryLimit`,
  `QueryEnums`, range parsing), call one service method and answer with
  `httputil.RespondOK` / `RespondServiceError`. No business logic in
  handlers and no SQL in services.
- Services return `errorcode` kinds (`ValidationError`, `NotFoundError`,
  `ConflictError`, ...); `RespondServiceError` maps them to 400/404/409.
  Anything else is a logged 500.
- Every response uses the `contracts.APIResponse` envelope. A list with no
  rows encodes `[]`, never `null`. A value the API cannot compute is a
  JSON `null` (a pointer field), never a fabricated `0` or a placeholder
  string.
- Pagination uses opaque `infra/cursor` cursors, never offsets over
  unbounded data.

## ClickHouse Rules

- Every query goes through `dbutil.SelectCH` / `QueryRowCH` with a budget
  context (`OverviewCtx`, `ExplorerCtx`, `DashboardCtx`) and a stable
  operation name. These enforce limits, metrics and the query comment.
- Bind every value with `clickhouse.Named`. Bind time bounds with
  `chargs.RangeArgs` / `chargs.Millis` (millisecond `DateTime64`) and
  cursors with `chargs.Nanos`. A plain `time.Time` binds at second
  precision and breaks primary-key pruning.
- Identifiers that cannot be bound (map keys, column names) come from an
  allowlist or a validated charset, never straight from a request.
- Filter on `tenant_id` and the time range in `PREWHERE`.

## Go Standards

These follow Effective Go, Go Code Review Comments and the bug-preventing
parts of the Uber Go style guide. `.golangci.yml` enforces most of them;
CI runs `go mod tidy -diff`, `go vet`, golangci-lint, `go test -race` and
govulncheck.

### Tooling

- `make fmt` (gofmt + goimports, imports grouped stdlib / third-party /
  `github.com/optikklabs`), `make lint`, `make test`, `make vulncheck`.
- Fix lint findings; do not suppress them. A `//nolint` must name the
  linter and give the reason (`//nolint:nilnil // a nil cursor is the first
  page`); nolintlint rejects anything vaguer.
- `go.mod` pins the Go patch release CI builds with. Bump it when
  govulncheck reports a standard-library fix.

### Errors

- Return errors; never panic on input. A panic is reserved for a broken
  internal invariant (an unknown constant, a missing budget context).
- Wrap with context using `%w`; compare with `errors.Is` / `errors.As`,
  never `==` or a type switch on a wrapped error.
- No `(nil, nil)` for pointers or interfaces: return a value, a sentinel
  error, or a `(T, bool)` pair. A nil map or slice is a valid empty result.
- Discard an error only when it cannot happen or cannot be acted on
  (`crypto/rand.Read`, hash writes, close-after-failure), and say which.

### Context

- `ctx context.Context` is the first parameter of anything that does I/O,
  and is passed down, never replaced by `context.Background()`.
- Work that must outlive the caller (shutdown drains, background jobs,
  failure bookkeeping) derives from `context.WithoutCancel(ctx)` so it keeps
  the caller's values. Only `main` and long-lived workers start from
  `context.Background()`.
- Log with the `*Context` slog variants and typed attrs:
  `slog.WarnContext(ctx, "msg", slog.String("k", v))`.

### Code shape

- Evaluate before you return: never `return rows, query(&rows)`. Go leaves
  the order of reading `rows` and calling `query` unspecified. Write
  `err := query(&rows); return rows, err`.
- Never append to a slice you do not own and then keep using the original
  (`args := append(base, x)` aliases `base`'s backing array).
- Prefer the standard library: `slices`, `maps`, `strings.Cut`, `min`/`max`,
  `for i := range n`, `errors.Join`, `net.ListenConfig`. Do not hand-roll
  what it provides, and do not shadow builtins (`max`, `new`, `len`).
- Keep functions to five or fewer results; return a struct past that.
- Accept interfaces where a seam is needed and return concrete types.
  Define an interface beside its consumer, not its implementation.
- Names follow Go Code Review Comments: short receivers, `ErrFoo`
  sentinels, `FooError` types, initialisms upper-case (`ID`, `URL`, `HTTP`),
  no `Get` prefix on plain getters, no stutter (`kafka.Client`, not
  `kafka.KafkaClient`).
- A comment explains why, not what. Document an exported symbol when its
  behaviour is not obvious from its name and signature.

### Tests

- Tests are table-driven where cases share a shape. Use the standard
  `testing` package, `t.Context()` and `t.TempDir()`.
- End-to-end behaviour is verified by running the stack and ingesting real
  telemetry, not by mocks of ClickHouse or Kafka.
