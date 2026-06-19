// scenarios/user.js — Load test for user profile and preferences endpoints.

import { sleep } from "k6";
import { DEFAULT_THRESHOLDS, activeProfile } from "../config.js";
import { bootstrapAuth } from "../helpers/setup.js";
import { authedGet, authedPut, checkOk } from "../helpers/http.js";

export const options = Object.assign(
  { thresholds: DEFAULT_THRESHOLDS },
  activeProfile(),
);

export function setup() {
  return bootstrapAuth();
}

export default function (ctx) {
  // ── GET /settings/profile ──────────────────────────────────────
  const profileRes = authedGet("/api/v1/settings/profile", ctx, {
    tags: { name: "GET /settings/profile" },
  });
  checkOk(profileRes, "GET /settings/profile");

  // ── PUT /settings/profile ──────────────────────────────────────
  const updateRes = authedPut("/api/v1/settings/profile", {
    name:      "Load Tester",
    avatarUrl: "",
  }, ctx, { tags: { name: "PUT /settings/profile" } });
  checkOk(updateRes, "PUT /settings/profile");

  // ── PUT /settings/preferences ──────────────────────────────────
  const prefsRes = authedPut("/api/v1/settings/preferences", {
    preferences: {
      theme:    "dark",
      timezone: "UTC",
    },
  }, ctx, { tags: { name: "PUT /settings/preferences" } });
  checkOk(prefsRes, "PUT /settings/preferences");

  sleep(0.5);
}
