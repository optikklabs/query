// scenarios/logs.js — Load test for all log-related endpoints.

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

export default function (ctx) {
  const range = getActiveTimeRange();

  // ── POST /logs/query ───────────────────────────────────────────
  const queryRes = authedPost("/api/v1/logs/query", {
    startTime: range.startTime,
    endTime:   range.endTime,
    limit:     50,
  }, ctx, { tags: { name: "POST /logs/query" } });
  checkOk(queryRes, "POST /logs/query");

  // ── POST /logs/facets ──────────────────────────────────────────
  const facetsRes = authedPost("/api/v1/logs/facets", {
    startTime: range.startTime,
    endTime:   range.endTime,
  }, ctx, { tags: { name: "POST /logs/facets" } });
  checkOk(facetsRes, "POST /logs/facets");

  // ── POST /logs/summary ─────────────────────────────────────────
  const summaryRes = authedPost("/api/v1/logs/summary", {
    startTime: range.startTime,
    endTime:   range.endTime,
  }, ctx, { tags: { name: "POST /logs/summary" } });
  checkOk(summaryRes, "POST /logs/summary");

  // ── POST /logs/trend ───────────────────────────────────────────
  const trendRes = authedPost("/api/v1/logs/trend", {
    startTime: range.startTime,
    endTime:   range.endTime,
  }, ctx, { tags: { name: "POST /logs/trend" } });
  checkOk(trendRes, "POST /logs/trend");

  // ── GET /logs/{id} ─────────────────────────────────────────────
  // Uses a placeholder ID — validates the handler path, not data.
  const detailRes = authedGet("/api/v1/logs/placeholder-log-id", ctx, {
    tags: { name: "GET /logs/{id}" },
  });
  checkStatus(detailRes, detailRes.status, "GET /logs/{id}");

  // ── GET /logs/trace/{traceID} ──────────────────────────────────
  const traceLogsRes = authedGet(
    "/api/v1/logs/trace/00000000000000000000000000000001",
    ctx,
    { tags: { name: "GET /logs/trace/{traceID}" } },
  );
  checkStatus(traceLogsRes, traceLogsRes.status, "GET /logs/trace/{traceID}");

  sleep(0.5);
}
