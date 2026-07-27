# Optikk query latency remediation

**Date:** 2026-07-26  
**Scope:** the eight slowest APIs from the production query benchmark  
**Status:** diagnosis and rollout plan; no production configuration was changed

## Executive finding

The main p99 problem is admission queueing in the query service, not slow
ClickHouse execution.

Production currently runs:

- **2 query replicas**, with the HPA capped at 2
- **1 expensive-query slot per pod**
- therefore only **2 concurrent telemetry queries for the whole service**

The limiter applies to every logs, traces, metrics, services, infrastructure,
cloud, LLM, and saturation route. A long explorer scan can therefore
head-of-line block a small trace lookup.

During the final high-load interval, the eight slow APIs had HTTP p99 values of
1,628–1,715 ms, while their ClickHouse p99 values were 72–374 ms. Percentiles
cannot be subtracted per request, but the size and consistency of this gap show
that most tail time is outside ClickHouse and is consistent with waiting for
one of the two limiter slots.

## Evidence from the production query log

This table joins the benchmark HTTP p99 with ClickHouse `system.query_log`
statistics from the aligned 32-VU interval, 17:17–17:23 UTC.

| API | HTTP p99 | ClickHouse p99 | Mean DB | Mean rows read | Main issue |
|---|---:|---:|---:|---:|---|
| `logs_query` | 1,715 ms | 264 ms | 32.9 ms | 47,485 | queue + ordering/scan |
| `span_attributes` | 1,668 ms | 104 ms | 29.2 ms | 5,590 | queue |
| `logs_summary` | 1,667 ms | 97 ms | 23.3 ms | 137,858 | queue + raw aggregation |
| `trace_related` | 1,651 ms | 374 ms | 71.6 ms | 34,051 | queue + wrong source/order |
| `trace_span_events` | 1,647 ms | 79 ms | 17.6 ms | 1,592 | queue |
| `trace_errors` | 1,637 ms | 83 ms | 21.1 ms | 1,757 | queue |
| `trace_error_path` | 1,636 ms | 72 ms | 21.0 ms | 1,756 | queue |
| `trace_spans` | 1,628 ms | 84 ms | 24.6 ms | 3,531 | queue |

At 16 VUs the service delivered 80.05 requests/s. At 32 VUs it delivered only
79.02 requests/s while aggregate p99 rose from 647 ms to 1,459 ms. That is the
classic signature of a full admission queue.

The reported HPA CPU value of 176% is relative to the pod's very small 50m CPU
request. It does not mean each pod consumed 1.76 CPU cores. The deployment
allows up to one core per pod.

## Production drift to correct first

The live deployment uses query image `v0.9.1`, and the live HPA has
`maxReplicas: 2`.

The checked-in deployment configuration already has:

- query HPA `maxReplicas: 5`
- query service code at `v0.9.3`
- in-flight coalescing for identical ClickHouse reads in `v0.9.3`

Reconcile production with these checked-in versions before judging further
code changes. The coalescing optimization helps real dashboards when several
clients issue byte-identical reads. It will help less when every request uses
a slightly different `now` value.

## Prioritized fixes

### P0 — restore horizontal capacity

Apply the checked-in HPA maximum of 5, leaving one expensive-query slot per pod
for the first experiment.

This changes total telemetry slots from 2 to as many as 5 without increasing
per-pod ClickHouse pressure. Scale and benchmark in steps at 3, 4, and 5
replicas. The two-slot benchmark ceiling was about 80 requests/s, so a linear
upper estimate at five slots is about 200 requests/s; this is a hypothesis,
not a capacity guarantee.

Do not raise both replica count and per-pod concurrency in the same test. That
would hide which change helped and could oversubscribe the four-core
ClickHouse node.

### P1 — fix the two genuinely expensive query shapes

#### `logs_query`: align the sort with the table key

The logs table is ordered by:

```text
(tenant_id, ts_bucket, timestamp)
```

The API currently orders by:

```text
timestamp DESC, log_id DESC
```

For a seven-day request, ClickHouse reads the full bounded range and performs
extra sorting before returning only 21 rows. Ordering by:

```text
ts_bucket DESC, timestamp DESC, log_id DESC
```

is semantically equivalent because `ts_bucket` is derived from `timestamp`,
but it aligns the read with the table order.

One uncached production comparison returned the identical 21 rows:

| Shape | Duration | Rows read | Bytes read |
|---|---:|---:|---:|
| Current ordering | 267 ms | 428,982 | 18.73 MiB |
| Primary-key ordering | 42 ms | 428,982 | 18.73 MiB |

That single comparison is a strong candidate result, not a replacement for a
repeated A/B benchmark. Preserve the existing `(timestamp, log_id)` cursor
contract and add repository tests proving first-page and cursor-page equality.

#### `trace_related`: query the root-span table

`trace_related` asks only for root spans but currently reads the full
`optikk.spans` table. The existing `optikk.spans_root` materialized table was
created for exactly this root-only access pattern.

One uncached production comparison:

| Source | Duration | Rows read | Bytes read |
|---|---:|---:|---:|
| `optikk.spans` | 149 ms | 2,190,845 | 52.57 MiB |
| `optikk.spans_root` | 47 ms | 1,083,086 | 15.52 MiB |

Switching sources cut this sample's latency by 3.2× and bytes by 3.4×.

The root table is still ordered by tenant and time, not service and operation.
After the source switch, add and benchmark a projection ordered by:

```text
(tenant_id, service, name, timestamp, trace_id)
```

Use `EXPLAIN indexes = 1` and `system.query_log` to verify that the projection
reduces granules and rows read before materializing it across all retained
parts.

### P1 — remove head-of-line blocking

Replace the single route-prefix semaphore with observable workload classes:

- **detail lookup:** trace/span/log identity reads
- **overview:** summaries, trends, RED panels
- **explorer scan:** logs/traces search and facets

Give detail lookups reserved capacity so a seven-day explorer query cannot
block all trace-detail requests. Keep a bounded total across classes and stay
within the per-pod ClickHouse pool.

The current ClickHouse budgets allow 16 threads for every explorer query.
That is too aggressive for several concurrent queries on a four-core
ClickHouse node. When increasing API concurrency, benchmark lower values such
as 2–4 threads for explorer reads and 1–2 for point/detail reads.

Add these metrics before tuning:

- limiter wait-duration histogram by workload class
- queued-request gauge by class
- admitted/in-use gauge by class
- cancellation/rejection counter by class

Use queue wait or queue depth as an HPA custom metric. CPU utilization is an
indirect signal for a service deliberately blocked on an admission semaphore.

### P2 — reduce repeated trace-detail scans

The trace detail page calls several APIs that independently read the same
trace:

- spans
- events
- errors
- error path
- critical path
- service map
- summary

Add a composite trace-detail endpoint, or a short-lived tenant-and-trace keyed
cache of a canonical trace read, then derive these views in Go. This removes
repeated index work, round trips, and limiter acquisitions.

For workloads that retain the separate endpoints, benchmark a ClickHouse
projection ordered by:

```text
(tenant_id, trace_id, span_id, timestamp)
```

The current trace-id bloom filter already keeps these database calls below
104 ms p99, so this projection is lower priority than queue isolation.

### P2 — roll up log summaries

`logs_summary` aggregates raw log rows. Add `log_stats_1m`, `log_stats_5m`, and
`log_stats_1h` materialized rollups for unfiltered and supported
resource-dimension summaries. Use the rollup only when the request's filters
can be answered exactly; retain the raw-table fallback for body, trace, span,
and arbitrary attribute filters.

For summary and trend calls, align cacheable end times to a small fixed bucket.
The existing 60-second ClickHouse query cache and in-flight coalescing cannot
help when every client sends a unique millisecond end time.

## Recommended rollout order

1. Deploy query `v0.9.3` and reconcile the HPA maximum from 2 to 5.
2. Re-run the same 1/4/8/16/32-VU benchmark at each replica level.
3. Implement and A/B test the `logs_query` ordering change.
4. Move `trace_related` to `spans_root`; then test the operation projection.
5. Add limiter metrics and split detail, overview, and explorer capacity.
6. Add the trace lookup projection/composite endpoint only if trace-detail DB
   time is still material after queueing is removed.
7. Add log rollups before log volume makes raw summaries a dominant cost.

## Acceptance criteria

At 32 VUs, with the same 31-API mix and four time windows:

- throughput must continue rising beyond 80 requests/s
- aggregate p99 should remain below 750 ms
- trace-detail API p99 should remain below 500 ms
- `logs_query` and `trace_related` p99 should remain below 750 ms
- failures and ClickHouse query-limit errors must remain zero
- ClickHouse CPU should remain below 70% sustained
- limiter wait p99 should stay below 100 ms for detail requests

Record HTTP latency, limiter wait, ClickHouse duration, rows read, bytes read,
replica count, CPU, memory, and ClickHouse query concurrency in the same time
series so the next saturation point can be attributed rather than inferred.
