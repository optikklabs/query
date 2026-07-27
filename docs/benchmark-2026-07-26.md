# Optikk query production benchmark

**Date:** 2026-07-26  
**Target:** Optikk production backend APIs  
**Measurement:** backend HTTP response latency  
**Workload:** 31 APIs × 4 lookback windows × 5 concurrency levels

[Open the interactive HTML visualization](benchmark-2026-07-26.html).

## Result

- Practical query knee: **16 VUs**
- Saturation throughput: **approximately 80 requests/s**
- Aggregate p99 at the knee: **647 ms**
- Aggregate p99 at 32 VUs: **1,459 ms**
- Total measured requests: **49,709**
- Failures: **0**

Moving from 16 to 32 VUs did not add throughput: 80.05 requests/s became
79.02 requests/s. Over the same step, mean latency rose from 138 ms to 317 ms
and p99 rose 126%, from 647 ms to 1,459 ms.

## Concurrency curve

| VUs | Requests | Throughput (req/s) | Mean (ms) | p95 (ms) | p99 (ms) | Failures | Query HPA CPU max |
|---:|---:|---:|---:|---:|---:|---:|---:|
| 1 | 1,984 | 13.18 | 43.56 | 73.10 | 106.31 | 0 | 29% |
| 4 | 6,913 | 40.25 | 54.91 | 112.53 | 138.89 | 0 | 97% |
| 8 | 10,478 | 59.81 | 83.08 | 191.01 | 293.08 | 0 | 138% |
| 16 | 14,198 | 80.05 | 138.39 | 391.51 | 646.70 | 0 | 176% |
| 32 | 14,136 | 79.02 | 316.78 | 1,032.03 | 1,459.14 | 0 | 176% |

The query HPA was already at its configured maximum of two replicas. This is
the clearest capacity constraint associated with the latency knee.

## Slowest API/window combinations at 32 VUs

| API | Lookback | p99 |
|---|---:|---:|
| `logs_query` | 24h | 1,715 ms |
| `span_attributes` | 3h | 1,668 ms |
| `logs_summary` | 24h | 1,667 ms |
| `trace_related` | 7d | 1,651 ms |
| `trace_span_events` | 24h | 1,647 ms |
| `trace_errors` | 3h | 1,637 ms |
| `trace_error_path` | 24h | 1,636 ms |
| `trace_spans` | 7d | 1,628 ms |

## Coverage

The lookback windows were 30 minutes, 3 hours, 24 hours, and 7 days. The 31
tested API workloads were:

`ingestion_overview`, `ingestion_services`, `ingestion_summary`,
`ingestion_timeseries_type`, `log_by_id`, `logs_by_trace_id`, `logs_facets`,
`logs_query`, `logs_summary`, `logs_trend`, `red_by_endpoint`,
`red_fleet_overview`, `red_latency_percentiles`, `red_request_error_rate`,
`red_request_rate`, `red_service_summary`, `red_services`,
`red_status_timeseries`, `red_top_endpoints`, `span_attributes`, `trace_by_id`,
`trace_critical_path`, `trace_error_path`, `trace_errors`, `trace_facets`,
`trace_query`, `trace_related`, `trace_service_map`, `trace_span_events`,
`trace_spans`, and `trace_trend`.

## Interpretation

Sixteen concurrent query VUs is the practical knee for this deployment. The
32-VU level is saturated: throughput is flat while queueing latency grows
rapidly. Increasing the query HPA maximum is the first scaling experiment to
run, but it must be validated against ClickHouse, which reached roughly 1.6 CPU
cores during the query workload.

No credentials are stored in this document or its HTML visualization.

## Remediation

The production query log shows that the dominant tail is admission queueing:
the slow APIs had HTTP p99 values of 1,628–1,715 ms while their ClickHouse p99
values were 72–374 ms. Production has only two query replicas and one
expensive-query slot per replica.

See the evidence, route-specific fixes, rollout order, and acceptance criteria
in the [query latency remediation plan](query-latency-remediation-2026-07-26.md).
