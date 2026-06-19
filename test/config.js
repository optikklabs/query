// config.js — Shared configuration for all load test scenarios.
// All values are overridable via k6 __ENV variables.

export const BASE_URL = __ENV.BASE_URL || "http://localhost:19090";

// ── Bootstrap Credentials ────────────────────────────────────────────
export const TEAM_NAME   = __ENV.TEAM_NAME   || "loadtest-team";
export const ORG_NAME    = __ENV.ORG_NAME    || "loadtest-org";
export const USER_EMAIL  = __ENV.USER_EMAIL  || "loadtest@optikk.dev";
export const USER_NAME   = __ENV.USER_NAME   || "Load Tester";
export const USER_PASS   = __ENV.USER_PASS   || "LoadTest@2026!";

// ── Time Range Configuration ─────────────────────────────────────────
export const TIME_RANGE  = __ENV.TIME_RANGE  || "1h"; // e.g. "15m", "1h", "6h", "24h"

// ── Default Thresholds ──────────────────────────────────────────────
export const DEFAULT_THRESHOLDS = {
  http_req_duration: ["p(95)<500"],
  http_req_failed:   ["rate<0.01"],
};

// ── Profile Presets (selected via --env PROFILE=smoke|load|stress) ──
export const PROFILES = {
  smoke: {
    vus:      1,
    duration: "30s",
  },
  load: {
    stages: [
      { duration: "30s", target: 10 },
      { duration: "3m",  target: 50 },
      { duration: "1m",  target: 50 },
      { duration: "30s", target: 0 },
    ],
  },
  stress: {
    stages: [
      { duration: "1m",  target: 50 },
      { duration: "5m",  target: 200 },
      { duration: "2m",  target: 200 },
      { duration: "2m",  target: 0 },
    ],
  },
};

export function activeProfile() {
  const name = (__ENV.PROFILE || "smoke").toLowerCase();
  return PROFILES[name] || PROFILES.smoke;
}
