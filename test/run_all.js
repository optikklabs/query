// run_all.js — Orchestrator that runs all scenario functions in a single k6 execution.
// Usage: k6 run query/test/run_all.js --env PROFILE=smoke|load|stress

import { sleep } from "k6";
import { DEFAULT_THRESHOLDS, activeProfile } from "./config.js";
import { bootstrapAuth } from "./helpers/setup.js";

// ── Import scenario default functions ────────────────────────────
import healthRun           from "./scenarios/health.js";
import authRun             from "./scenarios/auth.js";
import logsRun             from "./scenarios/logs.js";
import tracesRun           from "./scenarios/traces.js";
import metricsRun          from "./scenarios/metrics.js";
import servicesRun         from "./scenarios/services.js";
import infrastructureRun   from "./scenarios/infrastructure.js";
import satDbRun            from "./scenarios/saturation_database.js";
import satKafkaRun         from "./scenarios/saturation_kafka.js";
import alertingRun         from "./scenarios/alerting.js";
import userRun             from "./scenarios/user.js";

// ── k6 Options ───────────────────────────────────────────────────
export const options = Object.assign(
  { thresholds: DEFAULT_THRESHOLDS },
  activeProfile(),
);

// ── Setup: bootstrap auth once for all VUs ───────────────────────
export function setup() {
  return bootstrapAuth();
}

// ── Default: run every scenario sequentially per iteration ───────
export default function (ctx) {
  healthRun();          // No auth needed.
  authRun(ctx);
  logsRun(ctx);
  tracesRun(ctx);
  metricsRun(ctx);
  servicesRun(ctx);
  infrastructureRun(ctx);
  satDbRun(ctx);
  satKafkaRun(ctx);
  alertingRun(ctx);
  userRun(ctx);

  sleep(1);
}
