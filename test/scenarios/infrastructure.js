// scenarios/infrastructure.js — Load test for infrastructure endpoints.

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

const PLACEHOLDER_HOST = "node-01";

export default function (ctx) {
  const qs = timeParams(getActiveTimeRange());

  // ── CPU ────────────────────────────────────────────────────────
  const cpuEndpoints = [
    "/api/v1/infrastructure/cpu/avg",
    "/api/v1/infrastructure/cpu/by-instance",
  ];

  for (const ep of cpuEndpoints) {
    const tag = `GET ${ep.replace("/api/v1", "")}`;
    const res = authedGet(`${ep}${qs}`, ctx, { tags: { name: tag } });
    checkOk(res, tag);
  }

  // ── Memory ─────────────────────────────────────────────────────
  const memEndpoints = [
    "/api/v1/infrastructure/memory/avg",
    "/api/v1/infrastructure/memory/by-instance",
  ];

  for (const ep of memEndpoints) {
    const tag = `GET ${ep.replace("/api/v1", "")}`;
    const res = authedGet(`${ep}${qs}`, ctx, { tags: { name: tag } });
    checkOk(res, tag);
  }

  // ── Fleet Pods ─────────────────────────────────────────────────
  const podsRes = authedGet(`/api/v1/infrastructure/fleet/pods${qs}`, ctx, {
    tags: { name: "GET /infrastructure/fleet/pods" },
  });
  checkOk(podsRes, "GET /infrastructure/fleet/pods");

  // ── Hosts ──────────────────────────────────────────────────────
  const hostsRes = authedGet(`/api/v1/infrastructure/hosts${qs}`, ctx, {
    tags: { name: "GET /infrastructure/hosts" },
  });
  checkOk(hostsRes, "GET /infrastructure/hosts");

  // ── Nodes ──────────────────────────────────────────────────────
  const nodeEndpoints = [
    "/api/v1/infrastructure/nodes",
    "/api/v1/infrastructure/nodes/summary",
  ];

  for (const ep of nodeEndpoints) {
    const tag = `GET ${ep.replace("/api/v1", "")}`;
    const res = authedGet(`${ep}${qs}`, ctx, { tags: { name: tag } });
    checkOk(res, tag);
  }

  // ── Node Services (placeholder host) ───────────────────────────
  const nodeSvcRes = authedGet(
    `/api/v1/infrastructure/nodes/${PLACEHOLDER_HOST}/services${qs}`,
    ctx,
    { tags: { name: "GET /infrastructure/nodes/{host}/services" } },
  );
  checkStatus(nodeSvcRes, nodeSvcRes.status, "GET /infrastructure/nodes/{host}/services");

  sleep(0.5);
}
