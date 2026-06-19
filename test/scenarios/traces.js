// scenarios/traces.js — Load test for all trace-related endpoints.

import { sleep } from "k6";
import { DEFAULT_THRESHOLDS, activeProfile } from "../config.js";
import { bootstrapAuth } from "../helpers/setup.js";
import { authedGet, authedPost, checkOk, checkStatus } from "../helpers/http.js";
import { getActiveTimeRange } from "../helpers/time.js";

export const options = Object.assign(
  { thresholds: DEFAULT_THRESHOLDS },
  activeProfile(),
);

export function setup() {
  return bootstrapAuth();
}

const PLACEHOLDER_TRACE = "00000000000000000000000000000001";
const PLACEHOLDER_SPAN  = "0000000000000001";

export default function (ctx) {
  const range = getActiveTimeRange();

  // ── Explorer POST endpoints ────────────────────────────────────
  const queryRes = authedPost("/api/v1/traces/query", {
    startTime: range.startTime,
    endTime:   range.endTime,
    limit:     20,
  }, ctx, { tags: { name: "POST /traces/query" } });
  checkOk(queryRes, "POST /traces/query");

  const facetsRes = authedPost("/api/v1/traces/facets", {
    startTime: range.startTime,
    endTime:   range.endTime,
  }, ctx, { tags: { name: "POST /traces/facets" } });
  checkOk(facetsRes, "POST /traces/facets");

  const trendRes = authedPost("/api/v1/traces/trend", {
    startTime: range.startTime,
    endTime:   range.endTime,
  }, ctx, { tags: { name: "POST /traces/trend" } });
  checkOk(trendRes, "POST /traces/trend");

  const suggestRes = authedPost("/api/v1/traces/suggest", {
    startTime: range.startTime,
    endTime:   range.endTime,
    field:     "service.name",
    prefix:    "",
    limit:     10,
  }, ctx, { tags: { name: "POST /traces/suggest" } });
  checkOk(suggestRes, "POST /traces/suggest");

  // ── Detail GET endpoints (placeholder IDs) ─────────────────────
  const endpoints = [
    { path: `/api/v1/traces/${PLACEHOLDER_TRACE}`,                                    tag: "GET /traces/{traceId}" },
    { path: `/api/v1/traces/${PLACEHOLDER_TRACE}/spans`,                              tag: "GET /traces/{traceId}/spans" },
    { path: `/api/v1/traces/${PLACEHOLDER_TRACE}/span-events`,                        tag: "GET /traces/{traceId}/span-events" },
    { path: `/api/v1/traces/${PLACEHOLDER_TRACE}/spans/${PLACEHOLDER_SPAN}/attributes`, tag: "GET /traces/{traceId}/spans/{spanId}/attributes" },
    { path: `/api/v1/traces/${PLACEHOLDER_TRACE}/related`,                            tag: "GET /traces/{traceId}/related" },
    { path: `/api/v1/traces/${PLACEHOLDER_TRACE}/critical-path`,                      tag: "GET /traces/{traceId}/critical-path" },
    { path: `/api/v1/traces/${PLACEHOLDER_TRACE}/error-path`,                         tag: "GET /traces/{traceId}/error-path" },
    { path: `/api/v1/traces/${PLACEHOLDER_TRACE}/service-map`,                        tag: "GET /traces/{traceId}/service-map" },
    { path: `/api/v1/traces/${PLACEHOLDER_TRACE}/errors`,                             tag: "GET /traces/{traceId}/errors" },
  ];

  for (const ep of endpoints) {
    const res = authedGet(ep.path, ctx, { tags: { name: ep.tag } });
    checkStatus(res, res.status, ep.tag);
  }

  sleep(0.5);
}
