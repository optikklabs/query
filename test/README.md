# Query Service — Load Tests

k6-based load test suite targeting every API endpoint in the query service.

## Prerequisites

```bash
# macOS
brew install k6

# or download from https://grafana.com/docs/k6/latest/get-started/installation/
```

## Quick Start

Make sure the query service is running locally on `:19090` (or set `BASE_URL`):

```bash
cd query && make run
```

### Run a single scenario

```bash
# Health checks (no auth)
k6 run query/test/scenarios/health.js

# Logs endpoints
k6 run query/test/scenarios/logs.js

# Any scenario — they all self-bootstrap auth
k6 run query/test/scenarios/traces.js
```

### Run the full suite

```bash
# Smoke test (1 VU, 30s)
k6 run query/test/run_all.js --env PROFILE=smoke

# Load test (10→50 VUs, 5 min)
k6 run query/test/run_all.js --env PROFILE=load

# Stress test (50→200 VUs, 10 min)
k6 run query/test/run_all.js --env PROFILE=stress
```

## Environment Variables

| Variable      | Default                     | Description                      |
|---------------|-----------------------------|----------------------------------|
| `BASE_URL`    | `http://localhost:19090`    | Query service base URL           |
| `PROFILE`     | `smoke`                     | Load profile: smoke, load, stress|
| `TIME_RANGE`  | `1h`                        | Time range: 15m, 1h, 6h, 24h     |
| `TEAM_NAME`   | `loadtest-team`             | Team name for bootstrap          |
| `ORG_NAME`    | `loadtest-org`              | Org name for bootstrap           |
| `USER_EMAIL`  | `loadtest@optikk.dev`       | User email for bootstrap         |
| `USER_NAME`   | `Load Tester`               | User name for bootstrap          |
| `USER_PASS`   | `LoadTest@2026!`            | User password for bootstrap      |

## Architecture

```
query/test/
├── config.js                    # Shared config, thresholds, profiles
├── helpers/
│   ├── setup.js                 # Bootstrap: team → user → login
│   ├── http.js                  # Auth-injecting HTTP wrappers (DRY)
│   └── time.js                  # Time-range generators
├── scenarios/
│   ├── health.js                # /health, /health/live, /health/ready
│   ├── auth.js                  # /auth/login, /auth/me, /auth/logout
│   ├── logs.js                  # /logs/query, facets, summary, trend, detail
│   ├── traces.js                # /traces/query, facets, trend, suggest, detail
│   ├── metrics.js               # /metrics/names, tags, explorer/query
│   ├── services.js              # RED fleet, RED service, errors, topology
│   ├── infrastructure.js        # CPU, memory, fleet, hosts, nodes
│   ├── saturation_database.js   # DB systems, latency, slow-queries, volume
│   ├── saturation_kafka.js      # Kafka producer, consumer, explorer
│   ├── alerting.js              # Monitors CRUD, notifications
│   └── user.js                  # Profile, preferences
└── run_all.js                   # Orchestrator (runs all scenarios)
```

## Bootstrap Flow

The `setup()` phase (runs **once** before VUs start) executes:

1. **Create team** → `POST /api/v1/teams` (public, no auth)
2. **Create user** → `POST /api/v1/users` (public, no auth)
3. **Login**       → `POST /api/v1/auth/login` → extracts `accessToken`

The returned `{ accessToken, teamId, userId }` is shared across all VUs.  
If team/user already exist, the setup gracefully skips to login.

## Endpoints Covered

| Domain                | # Endpoints | Scenario File            |
|-----------------------|:-----------:|--------------------------|
| Health                |      3      | `health.js`              |
| Auth                  |      3      | `auth.js`                |
| Logs                  |      6      | `logs.js`                |
| Traces                |     13      | `traces.js`              |
| Metrics               |      3      | `metrics.js`             |
| Services (RED/Errors) |     20      | `services.js`            |
| Infrastructure        |      9      | `infrastructure.js`      |
| Saturation – Database |      4      | `saturation_database.js` |
| Saturation – Kafka    |     12      | `saturation_kafka.js`    |
| Alerting              |     14      | `alerting.js`            |
| User                  |      3      | `user.js`                |
| **Total**             |   **90**    |                          |

## Thresholds

Default thresholds (configurable in `config.js`):

- **p95 latency** < 500ms
- **Error rate** < 1%
