# Query API Verification Runbook

## Context

We need to prove the **query service** returns data points that faithfully match what was
ingested — i.e. for a known, controlled load there is no data loss, double-counting, or
bucketing error across time granularities (1m / 5m / 1h) and raw time-window sweeps.

The load source is the existing Java **verification-service**. It is auto-instrumented with the
OpenTelemetry Java agent, so **every HTTP request emits a fixed, deterministic shape of
telemetry** (spans + logs + metrics). That telemetry flows:

```
verification-service (OTel agent) ──OTLP→ otel-collector:4317 ──→ ingest (gRPC) ──→ Kafka ──→ ClickHouse ──→ query API
```

Because the per-request telemetry shape is fixed, the "points sent" ground truth is computable
as `N_requests × per_request_constant`. Step 4's counters (requests, DB calls per store, Kafka
topics, producers/consumers) are exactly the inputs to that calculation.

**Verification principle (atomic-level):** for every API we reconcile three numbers — *exact
input* (what the load sent), *expected* (derived from the per-request constants), and *observed*
(what the query API returned). We never compare aggregate totals alone. We first **calibrate**
the per-request constants with a single request in a quiet window, then scale.

The final deliverable is this runbook saved to `query/Verification.md`.

---

## Topology & ground-truth reference

Per **successful** `POST /api/verify` request (payload not starting with `fail`), the code path
(`verification-service/.../controller/VerificationController.java` +
`kafka/VerificationConsumer.java`) produces a fixed set of DB/Kafka operations:

| Store / system | Operations per request | Source |
|---|---|---|
| MySQL (MariaDB) | 3 — INSERT pending, SELECT byId, UPDATE completed | controller `save` + consumer `findById`/`save` |
| MongoDB | 2 — RECEIVED event, COMPLETED event | controller + consumer `eventRepository.save` |
| Redis | 2 — SET, GET (3 if `rfail*`: + WRONGTYPE) | controller `cacheRecord` |
| Kafka produce | 2 — `optikk.verification.requests`, `optikk.verification.results` | controller + consumer `send` |
| Kafka consume | 2 — requests consumer, results consumer | `VerificationConsumer`, `VerificationResultConsumer` |
| Spans | constant **K_span** (1 SERVER + the DB client spans + Kafka spans above) | OTel auto-instr |
| Logs | constant **K_log** (the `log.info`/`log.error` lines on the path) | OTel log appender |

`fail*` payloads short-circuit in the controller (throw before any DB/Kafka work) → they emit
only the SERVER span marked ERROR and the error log. `rfail*` payloads add one ERRORed Redis span.

**K_span and K_log are not assumed — they are measured in Step 0 (calibration).** Auto-instr may
merge/split spans (e.g. Redis SET+GET), so the single-request calibration is authoritative.

Service endpoints / ports:
- ingest: OTLP gRPC `:4318`, metrics `:18090/metrics`, health `:18090/health`
- query: API `:19090/api/v1`, metrics `:19090/metrics`
- otel-collector: OTLP gRPC `:4317`
- verification-service: `:8081` (`/api/verify`, `/api/verify/stats`)
- ClickHouse `:19000` (native) `:18123` (http) · MariaDB `:13306` · Mongo `:27017` · Redis `:6379` · Kafka `:9092`

Credentials: ClickHouse `default/clickhouse123` db `optikk` · MySQL `root/root123` db `optikk`.
Query API login: `qa-verify@optikk.local` / `Verify123!` (team_id 1, admin) →
`POST /api/v1/auth/login`, token in `data.accessToken`.

> Column note: in `optikk.spans` the service column is **`service`** (not `service_name`);
> in `optikk.logs` it is also **`service`**. `metric_type` lives only in `metrics_series`.

---

## Step 0 — Define success criteria & calibrate (do once)

Goal: lock the per-request constants so all later math is exact.

1. Bring up the full stack (Steps 1–3 below) with **zero other load**.
2. Send exactly **one** successful request:
   ```bash
   curl -s -XPOST localhost:8081/api/verify -H 'Content-Type: application/json' \
     -d '{"payload":"calib-1"}'
   ```
3. Wait for async consumer + ClickHouse flush (~10–15s), then measure observed primitives
   directly in ClickHouse (bypassing the query API) for `service.name='verification-service'`
   over the last 5 minutes:
   ```sql
   -- K_span: spans produced by the one request
   SELECT count() FROM optikk.spans WHERE service='verification-service';
   -- DB-call breakdown (validates the table above)
   SELECT attributes['db.system'] AS sys, count() FROM optikk.spans
     WHERE service='verification-service' AND attributes['db.system']!='' GROUP BY sys;
   -- K_log
   SELECT count() FROM optikk.logs WHERE service='verification-service';
   ```
4. Record observed `K_span`, `K_log`, and per-store DB-call counts. Reconcile against the
   ground-truth table; investigate any mismatch **before** load testing (a wrong constant
   invalidates every later check).

**Success criteria for the whole runbook:** for each API and each interval,
`|observed − expected| / expected ≤ tolerance`, where tolerance = 0 for raw count APIs
(traces/logs/RED request counts) and a small ε (e.g. ≤1 bucket, ≤1%) only for rate/percentile
APIs where boundary bucketing legitimately rounds.

---

## Step 1 — Check infrastructure is running

```bash
cd /Users/ramantayal/Desktop/pro/ingest
docker compose ps                                   # all services Up/healthy
docker compose exec clickhouse clickhouse-client --query "SELECT 1"
docker compose exec mariadb mysql -uroot -proot123 -e "SELECT 1"
docker compose exec mongodb mongosh --quiet --eval "db.adminCommand('ping').ok"
docker compose exec redis redis-cli ping            # PONG
docker compose exec kafka kafka-topics.sh --list --bootstrap-server localhost:9092
```
→ verify: each command returns success. If any is down: `docker compose up -d <svc>`.

---

## Step 2 — Clear the databases and Kafka

Goal: start from a known-empty state so counts are unambiguous.

```bash
cd /Users/ramantayal/Desktop/pro/ingest
# ClickHouse — telemetry tables
docker compose exec clickhouse clickhouse-client --multiquery --query "
  TRUNCATE TABLE optikk.spans; TRUNCATE TABLE optikk.logs; TRUNCATE TABLE optikk.metrics;
  TRUNCATE TABLE optikk.spans_resource; TRUNCATE TABLE optikk.logs_resource;
  TRUNCATE TABLE optikk.metrics_series; TRUNCATE TABLE optikk.trace_index;
  TRUNCATE TABLE optikk.metrics_1m; TRUNCATE TABLE optikk.metrics_5m; TRUNCATE TABLE optikk.metrics_1h;"

# MySQL — verification-service app tables only (keep query-service auth tables)
docker compose exec mariadb mysql -uroot -proot123 optikk -e "
  SET FOREIGN_KEY_CHECKS=0; TRUNCATE verification_record; TRUNCATE verification_event; SET FOREIGN_KEY_CHECKS=1;"

# Mongo + Redis
docker compose exec mongodb mongosh --quiet optikk --eval "db.verification_event.deleteMany({}); db.verification_record.deleteMany({});"
docker compose exec redis redis-cli FLUSHDB

# Kafka — reset offsets to earliest for the ingest + verification groups (do NOT delete ingest topics)
docker compose exec kafka bash -lc 'for g in spans-consumer-group logs-consumer-group metrics-consumer-group metric_series-consumer-group verification-group verification-results-group; do kafka-consumer-groups.sh --bootstrap-server localhost:9092 --group $g --reset-offsets --to-earliest --execute --all-topics || true; done'
```
→ verify: `SELECT count() FROM optikk.spans` = 0 and `... logs` = 0; `verification_record` empty.

> Note: TRUNCATE preserves schema. Only DROP DATABASE if you intend to re-run migrations via
> service restart. We keep the query-service auth/teams/users tables intact so the API token
> used for queries still works.

---

## Step 3 — Run ingest and query services

```bash
cd /Users/ramantayal/Desktop/pro/ingest && make run    # creates/migrates ClickHouse, OTLP gRPC :4318
cd /Users/ramantayal/Desktop/pro/query  && make run    # migrates MySQL, API :19090
```
→ verify: `curl -s localhost:18090/health` and `curl -s localhost:19090/health` both OK;
ingest log shows ClickHouse schema ready and Kafka consumers joined groups.

Obtain a query API bearer token (auth tables survive the truncate; if empty, bootstrap once):
```bash
# bootstrap (only if no user exists) — create team, user, then login
# POST /api/v1/teams, POST /api/v1/users  (see docs/api_curls.md)
TOKEN=$(curl -s -XPOST localhost:19090/api/v1/auth/login -H 'Content-Type: application/json' \
  -d '{"email":"user@example.com","password":"123"}' | jq -r .accessToken)
```

---

## Step 4 — Run controlled load with full counting

Goal: drive a **known** number of requests and capture every counter that feeds the ground truth.

1. **Record load parameters.** Decide `N` (e.g. 1,000) and concurrency. Use a wrapper that counts
   client-side outcomes so we know exact inputs (not just what the server logged):
   ```bash
   cd /Users/ramantayal/Desktop/pro/verification-service
   python3 load_generator.py        # adjust TOTAL/concurrency; logs success/failed/throughput
   ```
   Capture: `N_total`, `N_success` (2xx/202), `N_fail` (payload `fail*` → 5xx expected),
   `N_rfail`, start_ts, end_ts (use a clean minute boundary so 1m buckets align).

2. **Snapshot Kafka topology** (topics created, producers/consumers):
   ```bash
   docker compose exec kafka kafka-topics.sh --describe --bootstrap-server localhost:9092 \
     | grep -E 'optikk.(ingest|dlq|verification)'        # topic + partition count
   docker compose exec kafka kafka-consumer-groups.sh --bootstrap-server localhost:9092 --list
   ```
   Expected ingest topics: `optikk.ingest.{spans,logs,metrics,metric_series}` (+ `optikk.dlq.*`),
   plus `optikk.verification.{requests,results}`. Expected groups: the 4 ingest groups +
   `verification-group`, `verification-results-group`.

3. **Snapshot DB-call counters** from ingest Prometheus metrics, before vs after load:
   ```bash
   curl -s localhost:18090/metrics | grep -E 'optikk_db_queries_total|optikk_ingest_records_total|optikk_kafka_produce_duration_seconds_count'
   ```
   These give server-side records ingested per signal and ClickHouse insert counts — an
   independent cross-check on the OTLP→ClickHouse path (no loss between Kafka and ClickHouse).

4. **Compute expected ground truth:**
   ```
   expected_spans            = N_success × K_span_success + N_fail × 1 + N_rfail × ΔK_rfail
   expected_logs             = N_success × K_log_success + N_fail × K_log_fail
   expected_request_count    = N_total            (RED request count for POST /api/verify)
   expected_error_count      = N_fail (+ rfail Redis errors for the Redis error API)
   expected_mysql_calls      = N_success × 3   (1 INSERT + 1 SELECT + 1 UPDATE)
   expected_mongo_calls      = N_success × 2   (2 inserts: RECEIVED, COMPLETED)
   expected_redis_calls      = N_success × 2 + N_rfail × 1   (SET + GET [+ rfail LPUSH])
   ```
   **Reads vs saves (writes)** — derived from the controller + consumer code path:
   ```
   expected_mysql_saves  = N_success × 2   (INSERT pending + UPDATE completed)
   expected_mysql_reads  = N_success × 1   (SELECT findById)
   expected_mongo_saves  = N_success × 2   (both event inserts)
   expected_redis_saves  = N_success × 1   (SET)   redis_reads = N_success × 1 (GET)
   expected_total_saves  = N_success × 5   (2 mysql + 2 mongo + 1 redis)
   ```
   → verify: also wait for `optikk_kafka_consumer_lag_records` for all ingest groups to reach 0
   (all produced messages consumed into ClickHouse) before running Step 5.

---

## Step 5 — Per-API verification (detailed)

For every API below: pick the time window `[start, end]` covering the load, call the query API,
extract the count/series, and reconcile **input → expected → observed**. Repeat per interval
(see the matrix in Step 6). Cross-check observed against a direct ClickHouse `count()` for the
same window so a query-API bug is distinguishable from an ingestion bug.

### 5A. Traces — `POST /api/v1/traces/query`, `/facets`, `/trend`
1. `traces/query` filtered to `services:["verification-service"]`, window = load window.
   - Page through using `pageInfo.cursor` until `hasMore=false`; sum `results` length.
   - Expected = number of root traces = `N_total` (one trace per HTTP request).
   - Cross-check: `SELECT uniqExact(trace_id) FROM optikk.spans WHERE service='verification-service' AND is_root=1 AND <window>`.
   - **Cross-check must use the same `<window>`** — a stray out-of-window log/span otherwise
     inflates the expected count (see the 2026-06-23 run note below).
2. `traces/facets` → `topServices`/`topOperations` counts: the `POST /api/verify` operation count
   should equal `N_total`; `topStatuses` error count should equal `N_fail`.
3. `traces/trend` over the window with each interval: sum of per-bucket counts must equal `N_total`
   (verifies bucketing doesn't drop/duplicate across boundaries).
4. Sanity: total spans (sum of `spanCount` across traces) = `expected_spans` from Step 4.

### 5B. Logs — `POST /api/v1/logs/query`, `/facets`
1. `logs/query` filtered `services:["verification-service"]`, window = load window; paginate fully.
   - Expected = `expected_logs`. Cross-check `SELECT count() FROM optikk.logs WHERE service='verification-service' AND <window>`.
2. `logs/facets` severity breakdown: ERROR count must equal the error-path log lines
   (`N_fail` + `N_rfail` simulated-failure logs); INFO count = success-path lines.

### 5C. Metrics explorer — `GET /metrics/names`, `/{m}/tags`, `POST /metrics/explorer/query`
1. `metrics/names` over the window: verify the verification-service JVM/OTel metric names appear.
2. `metrics/explorer/query` for a **monotonic counter** the load drives (e.g. an HTTP server
   request counter or `kafka producer` counter) with `aggregation:"sum"`, `step:"1m"`:
   - Sum the returned series `values` across all `timestamps` → must equal `N_total`
     (counter rate×interval reconstructs the total; use `aggregation:"count"`/`sum` per metric type).
   - Re-run with `step:"5m"` and `step:"1h"`: the **windowed total is invariant to step**. This is
     the core interval check — `metrics_1m`, `metrics_5m`, `metrics_1h` must agree.
3. Boundary test: choose a window that splits a 1m bucket; confirm no double-count at edges.

### 5D. Services / RED — `GET /api/v1/spans/red/*`
1. `red/services` → must list `verification-service`.
2. `red/services/verification-service/summary` over the window: `requestCount` = `N_total`,
   `errorCount` = `N_fail`, `errorRate` = `N_fail/N_total`.
3. `red/request-and-error-rate` and `red/status-timeseries` at each interval: sum of per-bucket
   request counts = `N_total`; sum of error buckets = `N_fail`.
4. `red/top-endpoints` / `red/red-by-endpoint`: the `POST /api/verify` endpoint row count = `N_total`.
5. `red/top-db-queries`: rows for mysql/mongo/redis should reflect the per-store call totals.

### 5E. Database saturation — `GET /api/v1/saturation/database/latency/by-system`
1. Call over the window. Expected `db.system` groups: `mysql`, `mongodb`, `redis`.
2. Per-system call counts must equal `expected_mysql_calls` / `expected_mongo_calls`
   / `expected_redis_calls` from Step 4. `attributes` is a JSON column (db keys nest under `db`),
   but the **span name** is the most reliable deterministic signal for the call/op breakdown:
   ```sql
   -- Total calls per store (db spans carry a db.system attribute)
   SELECT attributes.db.system::String sys, count() FROM optikk.spans
     WHERE service='verification-service' AND attributes.db.system::String != '' AND <window>
   GROUP BY sys;   -- expect mysql=N×3, mongodb=N×2, redis=N×2(+rfail)
   -- Saves vs reads via operation in the span name
   SELECT multiIf(name LIKE 'INSERT %' OR name LIKE 'insert %' OR name LIKE 'UPDATE %'
                   OR name = 'SET', 'save',
                  name LIKE 'SELECT %' OR name = 'GET', 'read', 'other') AS kind,
          count()
   FROM optikk.spans WHERE service='verification-service' AND <window> GROUP BY kind;
   ```
   → verify: `save` count = `expected_total_saves` (N×5); per store matches the Step-4 split.
   Reconcile three ways — input → ClickHouse → and (once added, see "Recommended next
   additions") the verification-service's own `verification_db_calls_total` counter.

### 5F. Kafka saturation — `GET /api/v1/saturation/kafka/*`
1. `kafka/topics/backlog` and `consume-rate-by-topic` for `optikk.verification.requests` and
   `optikk.verification.results`: produced ≈ consumed (backlog → 0 after load).
2. `kafka/groups/health`, `consumer-lag-by-group`: `verification-group` / `verification-results-group`
   lag = 0; cross-check vs `kafka-consumer-groups.sh --describe`.
3. Topic/partition counts from the API match the Step-4 Kafka snapshot.

---

## Step 6 — Interval matrix

Run Step 5 across these dimensions and tabulate. The invariant being tested: **windowed totals
are identical regardless of granularity**; only per-bucket shape changes.

| Interval / step | Applies to | What must hold |
|---|---|---|
| `step=1m` | 5C metrics, 5D/5F timeseries | sum of buckets = total; ≤7d window (raw `metrics_1m` TTL) |
| `step=5m` | 5C metrics, timeseries | same windowed total as 1m (`metrics_5m`) |
| `step=1h` | 5C metrics, timeseries | same windowed total as 1m/5m (`metrics_1h`) |
| Raw window sweep 5m / 1h / 24h | 5A traces, 5B logs (no bucketing) | count grows monotonically; never exceeds `N_total`/`expected_logs`; boundary windows don't double-count |

For each cell record a row: `api | interval | exact_input | expected | observed | clickhouse_xcheck | pass/fail`.

---

## Step 7 — Report

Produce a reconciliation table (one row per API × interval from Step 6). Flag any row where
observed ≠ expected and note whether the ClickHouse cross-check also diverges:
- observed ≠ expected **and** ClickHouse = expected → **query-service bug** (bucketing/pagination/filter).
- observed ≠ expected **and** ClickHouse ≠ expected → **ingestion bug** (loss before query); use
  `optikk_db_queries_total{result=err}` and DLQ topic depth to localize.

---

## Verification run — 2026-06-23 (findings #1–#5 fixes)

Re-ran the runbook against the existing controlled load in ClickHouse (516 root traces, 3766
in-window logs) with the fixed query service (`go run ./cmd/query`, API `:19090`). Each row
reconciles **expected** (derived from the load) against **observed** (query API) and a direct
ClickHouse **cross-check** over the *same* window `[1782186400000, 1782186450000]` ms.

| Finding | API | exact_input | expected | observed | ch_xcheck | pass |
|---|---|---|---|---|---|---|
| #1a trace pagination | `traces/query` @100, 6 pages | 516 roots | 516 | 516 | 516 | ✅ |
| #2 facet operation | `traces/facets` | `POST /api/verify` | 500 | 500 | 500 | ✅ |
| #2 facet status ERROR | `traces/facets` | error roots | 50 | 50 | 50 | ✅ |
| #2 facet http_status 202 | `traces/facets` | 202 roots | 450 | 450 | 450 | ✅ |
| #2 facet http_status 500 | `traces/facets` | 500 roots | 50 | 50 | 50 | ✅ |
| #1b log pagination | `logs/query` @200, 19 pages | in-window logs | 3766 | 3766 | 3766 | ✅ |
| #3 service filter keeps ERROR | `logs/query` | framework errors | 100 | 100 | 100 | ✅ |
| #3 service filter keeps SEVERE | `logs/query` | severe logs | 50 | 50 | 50 | ✅ |
| #5 histogram count 1m/5m/1h | `metrics/explorer/query` | observations | 516 | 516 | 516 | ✅ |
| #4 exceptions sum 1m/5m/1h | `metrics/explorer/query` | exceptions | 50 | 50 | 50 | ✅ |

**14/14 reconciliations pass.**

### Bug found during verification (and fixed)

The first run **failed #1a and #1b**: trace pagination returned only 200/516 rows and log
pagination 2400/3766 — both terminating early after a few pages. Root cause was *not* the cursor
shape (the `(timestamp, span_id)` / `(timestamp, log_id)` keyset and ns-precision cursor were
correct) but the **parameter binding**: a `time.Time` passed via `clickhouse.Named` is rendered by
clickhouse-go at **whole-second** precision. The keyset boundary `curStart` was floored to the
second (e.g. `…406.000` instead of `…406.775102`), so once a page's boundary fell inside a
sub-second burst, the next page's `(timestamp, span_id) < (curStart, …)` matched **zero** rows.
This silently re-introduced the very truncation finding #1 set out to fix — just one layer lower.

Fix: bind the cursor timestamp with explicit nanosecond scale via
`clickhouse.DateNamed(name, t, clickhouse.NanoSeconds)` in both
[traces/explorer/repository.go](query/internal/modules/traces/explorer/repository.go) and
[logs/explorer/repository.go](query/internal/modules/logs/explorer/repository.go). After the fix,
both paginate completely (516 and 3766). Unit cursor round-trip tests cannot catch this — it is a
driver-level concern only an integration sweep surfaces.

> The initial #1b "off-by-one" (3767 vs 3766) was a **cross-check error, not a bug**: one stray
> log existed ~9 min past the load window; the un-windowed `count()` over-counted by one. The API
> correctly excluded it. Lesson encoded above: always window the cross-check.

---

## Known data-source gaps — Kafka panels (broker-scraped vs client-side)

Several Kafka panels render empty because they query **broker-scraped** metrics
(`kafka.partition.*`, `kafka.consumer_group.*`) that the custom MQ broker
(`ghcr.io/ramantayal12/mq`) never emits. But the Kafka **client** (the verification-service's
producer/consumer, auto-instrumented by the OTel Java agent) already emits rich
`kafka.producer.*` / `kafka.consumer.*` JMX metrics — confirmed flowing into `metrics_series`,
each tagged with `topic` and `messaging.consumer.group.name`. **Most "broker" panels can be
re-sourced to these client metrics instead of waiting on broker emission.**

### Re-sourceable today (client metric already flowing — change the query, no broker needed)

| Panel / handler | Currently queries (broker) | Re-source to (client) |
|---|---|---|
| `produce-rate-by-topic` | `kafka.partition.current_offset` Δ | `kafka.producer.record_send_total` (rate), by `topic` |
| `consume-rate-by-topic` | `kafka.consumer_group.offset` Δ | `kafka.consumer.records_consumed_total` (rate), by `topic` |
| `consumer-lag-by-group` | `kafka.consumer_group.lag` | `kafka.consumer.records_lag_max`, by group + `topic` |
| `topics/lag` | `kafka.consumer_group.lag` (+lead) | `kafka.consumer.records_lag_max` |
| `groups/partitions` | `kafka.consumer_group.lag`/`members` | `kafka.consumer.assigned_partitions`; members = distinct `client-id` |
| `topics/consumers` | `kafka.consumer_group.*` | distinct `client-id` / group on `kafka.consumer.*` |
| `groups/commits` / `groups/fetches` | — | already client: `kafka.consumer.commit_rate` / `fetch_rate` ✅ |
| `topics/throughput` | — | already client: `kafka.consumer.bytes_consumed_rate` ✅ |

Client lag (`records_lag_max`) is the consumer's own `log-end-offset − current-position`, so it is
a faithful substitute for broker `consumer_group.lag` for the active-consumer view — the single
most valuable currently-empty panel.

### Genuinely broker-only (no client equivalent — still require broker emission)

- **`cluster/health`** — `kafka.brokers`, `kafka.controller.active.count` (cluster membership).
- **`topics/backlog` log-size half** — `kafka.partition.oldest_offset` / `topic.partitions`
  (retention/log size). Note backlog ≈ `records_lag_max` *is* client-derivable; only the absolute
  log size and partition count are not.
- **Replication health** — `kafka.partition.replicas` / `replicas_in_sync` (ISR).

Recommendation: re-source the first table in `query/internal/modules/saturation/kafka/` to the
client metrics (a query-only change), and document the second table as a true broker gap.

---

## Recommended next additions to verification-service

Ranked by verification value. The theme: make every number the query API reports **independently
assertable** from a client-reported ground truth, so a mismatch localizes to query vs ingest vs DB.

1. **Deterministic DB-call & save counters (highest value).** The load path already emits
   `db.system` spans, but the only client-side ground truth today is `N_requests`. Add Micrometer
   counters in the controller/consumer, exported via the existing `/actuator/prometheus`:
   - `verification_db_calls_total{system,operation}` — incremented around each repository call.
   - `verification_db_saves_total{system}` — incremented on INSERT/UPDATE/SET only.
   This gives a **third reconciliation leg** (input → DB counter → telemetry) for Step 5E, turning
   "saves = N×5" from a code-derived expectation into a measured invariant. Without it, a dropped
   write is indistinguishable from a dropped *span* for that write.

2. **A DELETE / cleanup path.** The service does INSERT/SELECT/UPDATE but never DELETE, so the
   `db.operation` facet and slow-query/ops-by-operation panels are never exercised for deletes.
   A periodic `repository.deleteById` on completed records closes the CRUD matrix cheaply.

3. **Bounded latency variation.** `Thread.sleep(50)` is constant, so duration histograms collapse
   to one bucket and p50≈p95≈p99 — the percentile panels (#5 Tier B) can't be meaningfully
   verified. Inject a deterministic spread (e.g. `sleep(base + hash(id)%spread)`) so p50/p95/p99
   are distinct and predictable.

4. **A second downstream hop.** All spans share `service=verification-service`, so service-map /
   dependency / RED-by-service panels have a trivial single-node topology. A thin second service
   (or a self-call to a different endpoint) would exercise multi-service edges.

DB-calls and saves (items 1) are the priority — they directly back Step 4's
`expected_*_saves` and Step 5E.

## Files referenced
- Load: [verification-service/load_generator.py](verification-service/load_generator.py),
  [controller/VerificationController.java](verification-service/src/main/java/com/optikk/verification/controller/VerificationController.java),
  [kafka/VerificationConsumer.java](verification-service/src/main/java/com/optikk/verification/kafka/VerificationConsumer.java)
- Infra: [ingest/docker-compose.yml](ingest/docker-compose.yml), [ingest/config.yml](ingest/config.yml)
- Query routes: [query/internal/app/routes.go](query/internal/app/routes.go) and modules under
  [query/internal/modules/](query/internal/modules/)
- Metrics defs: [ingest/internal/infra/metrics/](ingest/internal/infra/metrics/)
- Schemas: [ingest/db/clickhouse/](ingest/db/clickhouse/)

## Deliverable
This runbook lives at **`query/Verification.md`** (moved from `docs/query-api-verification.md`),
co-located with the query service it verifies.
