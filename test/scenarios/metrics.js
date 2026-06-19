// scenarios/metrics.js — Load test for metrics explorer endpoints.

import { sleep } from "k6";
import { DEFAULT_THRESHOLDS, activeProfile } from "../config.js";
import { bootstrapAuth } from "../helpers/setup.js";
import { authedGet, authedPost, checkOk, checkStatus } from "../helpers/http.js";
import { getActiveTimeRange, timeParams } from "../helpers/time.js";

export const options = Object.assign(
  { thresholds: DEFAULT_THRESHOLDS },
  activeProfile(),
);

export function setup() {
  return bootstrapAuth();
}

export default function (ctx) {
  const range = getActiveTimeRange();
  const qs    = timeParams(range);

  // ── GET /metrics/names ─────────────────────────────────────────
  const namesRes = authedGet(`/api/v1/metrics/names${qs}`, ctx, {
    tags: { name: "GET /metrics/names" },
  });
  checkOk(namesRes, "GET /metrics/names");

  // ── GET /metrics/{metricName}/tags ─────────────────────────────
  const tagsRes = authedGet(`/api/v1/metrics/system.cpu.usage/tags${qs}`, ctx, {
    tags: { name: "GET /metrics/{name}/tags" },
  });
  checkStatus(tagsRes, tagsRes.status, "GET /metrics/{name}/tags");

  // ── POST /metrics/explorer/query ───────────────────────────────
  const queryRes = authedPost("/api/v1/metrics/explorer/query", {
    startTime: range.startTime,
    endTime:   range.endTime,
    step:      "1m",
    queries: [{
      id:          "q1",
      aggregation: "avg",
      metricName:  "system.cpu.usage",
      where:       [],
      groupBy:     [],
    }],
  }, ctx, { tags: { name: "POST /metrics/explorer/query" } });
  checkOk(queryRes, "POST /metrics/explorer/query");

  sleep(0.5);
}
