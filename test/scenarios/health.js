// scenarios/health.js — Load test for health-check endpoints.
// These are unauthenticated and exercise the readiness probes.

import http from "k6/http";
import { check, sleep } from "k6";
import { BASE_URL, DEFAULT_THRESHOLDS, activeProfile } from "../config.js";

export const options = Object.assign(
  { thresholds: DEFAULT_THRESHOLDS },
  activeProfile(),
);

export default function () {
  const res1 = http.get(`${BASE_URL}/health`);
  check(res1, { "GET /health 200": (r) => r.status === 200 });

  const res2 = http.get(`${BASE_URL}/health/live`);
  check(res2, { "GET /health/live 200": (r) => r.status === 200 });

  const res3 = http.get(`${BASE_URL}/health/ready`);
  check(res3, { "GET /health/ready 200": (r) => r.status === 200 });

  sleep(0.5);
}
