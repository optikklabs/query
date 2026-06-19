// scenarios/auth.js — Load test for authentication endpoints.

import http from "k6/http";
import { check, sleep } from "k6";
import { BASE_URL, USER_EMAIL, USER_PASS, DEFAULT_THRESHOLDS, activeProfile } from "../config.js";
import { bootstrapAuth } from "../helpers/setup.js";
import { authedGet, checkOk } from "../helpers/http.js";

export const options = Object.assign(
  { thresholds: DEFAULT_THRESHOLDS },
  activeProfile(),
);

export function setup() {
  return bootstrapAuth();
}

export default function (ctx) {
  // ── Login ──────────────────────────────────────────────────────
  const loginRes = http.post(
    `${BASE_URL}/api/v1/auth/login`,
    JSON.stringify({ email: USER_EMAIL, password: USER_PASS }),
    { headers: { "Content-Type": "application/json" }, tags: { name: "POST /auth/login" } },
  );
  check(loginRes, {
    "POST /auth/login 200":      (r) => r.status === 200,
    "POST /auth/login has token": (r) => {
      try { return !!r.json().data.accessToken; } catch (_) { return false; }
    },
  });

  // ── Auth Me ────────────────────────────────────────────────────
  const meRes = authedGet("/api/v1/auth/me", ctx, { tags: { name: "GET /auth/me" } });
  checkOk(meRes, "GET /auth/me");

  // ── Logout ─────────────────────────────────────────────────────
  const logoutRes = http.post(
    `${BASE_URL}/api/v1/auth/logout`,
    null,
    { headers: { "Content-Type": "application/json" }, tags: { name: "POST /auth/logout" } },
  );
  check(logoutRes, { "POST /auth/logout 200": (r) => r.status === 200 });

  sleep(0.5);
}
