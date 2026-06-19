// scenarios/services.js — Load test for RED fleet, RED service, errors, and topology.

import { sleep } from "k6";
import { DEFAULT_THRESHOLDS, activeProfile } from "../config.js";
import { bootstrapAuth } from "../helpers/setup.js";
import { authedGet, checkOk, checkStatus } from "../helpers/http.js";
import { getActiveTimeRange, timeParams } from "../helpers/time.js";

export const options = Object.assign(
  { thresholds: DEFAULT_THRESHOLDS },
  activeProfile(),
);

export function setup() {
  return bootstrapAuth();
}

const PLACEHOLDER_SERVICE  = "api-gateway";
const PLACEHOLDER_GROUP_ID = "1";

export default function (ctx) {
  const qs = timeParams(getActiveTimeRange());

  // ── RED Fleet ──────────────────────────────────────────────────
  const fleetEndpoints = [
    "/api/v1/spans/red/fleet-totals",
    "/api/v1/spans/red/services",
    "/api/v1/spans/red/apdex",
    "/api/v1/spans/red/request-and-error-rate",
    "/api/v1/spans/red/status-timeseries",
    "/api/v1/spans/red/latency-percentiles-timeseries",
    "/api/v1/spans/red/top-endpoints",
  ];

  for (const ep of fleetEndpoints) {
    const tag = `GET ${ep.replace("/api/v1", "")}`;
    const res = authedGet(`${ep}${qs}`, ctx, { tags: { name: tag } });
    checkOk(res, tag);
  }

  // ── RED Service (per-service) ──────────────────────────────────
  const svcEndpoints = [
    `/api/v1/spans/red/services/${PLACEHOLDER_SERVICE}/summary`,
    `/api/v1/spans/red/services/${PLACEHOLDER_SERVICE}/saturation-timeseries`,
  ];

  for (const ep of svcEndpoints) {
    const tag = `GET ${ep.replace("/api/v1", "").replace(PLACEHOLDER_SERVICE, "{svc}")}`;
    const res = authedGet(`${ep}${qs}`, ctx, { tags: { name: tag } });
    checkStatus(res, res.status, tag);
  }

  // ── Operation Baseline ─────────────────────────────────────────
  const baselineRes = authedGet(`/api/v1/spans/red/operation-baseline${qs}`, ctx, {
    tags: { name: "GET /spans/red/operation-baseline" },
  });
  checkOk(baselineRes, "GET /spans/red/operation-baseline");

  // ── Errors ─────────────────────────────────────────────────────
  const errorEndpoints = [
    "/api/v1/errors/service-error-rate",
    "/api/v1/errors/error-volume",
    "/api/v1/errors/groups",
  ];

  for (const ep of errorEndpoints) {
    const tag = `GET ${ep.replace("/api/v1", "")}`;
    const res = authedGet(`${ep}${qs}`, ctx, { tags: { name: tag } });
    checkOk(res, tag);
  }

  // ── Error Group Detail (placeholder) ───────────────────────────
  const groupEndpoints = [
    `/api/v1/errors/groups/${PLACEHOLDER_GROUP_ID}`,
    `/api/v1/errors/groups/${PLACEHOLDER_GROUP_ID}/traces`,
    `/api/v1/errors/groups/${PLACEHOLDER_GROUP_ID}/timeseries`,
    `/api/v1/errors/groups/${PLACEHOLDER_GROUP_ID}/latest-occurrence`,
    `/api/v1/errors/groups/${PLACEHOLDER_GROUP_ID}/facets`,
  ];

  for (const ep of groupEndpoints) {
    const tag = `GET ${ep.replace("/api/v1", "").replace(PLACEHOLDER_GROUP_ID, "{id}")}`;
    const res = authedGet(`${ep}${qs}`, ctx, { tags: { name: tag } });
    checkStatus(res, res.status, tag);
  }

  // ── Error Hotspot ──────────────────────────────────────────────
  const hotspotRes = authedGet(`/api/v1/spans/error-hotspot${qs}`, ctx, {
    tags: { name: "GET /spans/error-hotspot" },
  });
  checkOk(hotspotRes, "GET /spans/error-hotspot");

  // ── Topology ───────────────────────────────────────────────────
  const topoRes = authedGet(`/api/v1/services/topology${qs}`, ctx, {
    tags: { name: "GET /services/topology" },
  });
  checkOk(topoRes, "GET /services/topology");

  sleep(0.5);
}
