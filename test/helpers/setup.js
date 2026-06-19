// helpers/setup.js — Bootstrap flow: create team → create user → login.
// Called once in k6 setup() before any VUs start.

import http from "k6/http";
import { check } from "k6";
import {
  BASE_URL,
  TEAM_NAME,
  ORG_NAME,
  USER_EMAIL,
  USER_NAME,
  USER_PASS,
} from "../config.js";

const JSON_HEADERS = { "Content-Type": "application/json" };

/**
 * Bootstraps the full auth context:
 *   1. Create team (idempotent — tolerates 409)
 *   2. Create user (idempotent — tolerates 409)
 *   3. Login       → returns accessToken
 *
 * @returns {{ accessToken: string, teamId: number, userId: number }}
 */
export function bootstrapAuth() {
  // ── Step 1: Create Team ──────────────────────────────────────────
  const teamRes = http.post(
    `${BASE_URL}/api/v1/teams`,
    JSON.stringify({
      team_name:   TEAM_NAME,
      org_name:    ORG_NAME,
      slug:        "loadtest",
      description: "Load testing team",
      color:       "#3B82F6",
    }),
    { headers: JSON_HEADERS, tags: { name: "setup_create_team" } },
  );

  let teamId;
  if (teamRes.status === 200) {
    const body = teamRes.json();
    teamId = body.data.id;
    console.log(`✓ Team created: id=${teamId}`);
  } else {
    // Team may already exist — try to proceed with login to discover teamId.
    console.log(`⚠ Team creation returned ${teamRes.status}, proceeding to login.`);
  }

  // ── Step 2: Create User ──────────────────────────────────────────
  if (teamId) {
    const userRes = http.post(
      `${BASE_URL}/api/v1/users`,
      JSON.stringify({
        email:    USER_EMAIL,
        name:     USER_NAME,
        password: USER_PASS,
        role:     "admin",
        teamId:   teamId,
      }),
      { headers: JSON_HEADERS, tags: { name: "setup_create_user" } },
    );

    if (userRes.status === 200) {
      console.log(`✓ User created: ${USER_EMAIL}`);
    } else {
      console.log(`⚠ User creation returned ${userRes.status}, proceeding to login.`);
    }
  }

  // ── Step 3: Login ────────────────────────────────────────────────
  const loginRes = http.post(
    `${BASE_URL}/api/v1/auth/login`,
    JSON.stringify({
      email:    USER_EMAIL,
      password: USER_PASS,
    }),
    { headers: JSON_HEADERS, tags: { name: "setup_login" } },
  );

  const loginOk = check(loginRes, {
    "setup: login status 200": (r) => r.status === 200,
    "setup: login has token":  (r) => {
      try { return !!r.json().data.accessToken; }
      catch (_) { return false; }
    },
  });

  if (!loginOk) {
    console.error(`✗ Login failed: status=${loginRes.status} body=${loginRes.body}`);
    throw new Error("Bootstrap login failed — cannot proceed with load test.");
  }

  const loginData = loginRes.json().data;
  const accessToken = loginData.accessToken;

  // Resolve teamId from login response if team creation was skipped.
  if (!teamId) {
    teamId = loginData.team.id;
  }

  const userId = loginData.user.id;
  console.log(`✓ Logged in: userId=${userId}, teamId=${teamId}`);

  return { accessToken, teamId, userId };
}
