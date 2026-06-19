// scenarios/alerting.js — Load test for alerting monitors and notifications.

import { sleep } from "k6";
import { DEFAULT_THRESHOLDS, activeProfile } from "../config.js";
import { bootstrapAuth } from "../helpers/setup.js";
import { authedGet, authedPost, authedPut, authedDel, checkOk, checkStatus } from "../helpers/http.js";
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

  // ── Monitors: List ─────────────────────────────────────────────
  const listRes = authedGet("/api/v1/monitors/", ctx, {
    tags: { name: "GET /monitors/" },
  });
  checkOk(listRes, "GET /monitors/");

  // ── Monitors: Create ──────────────────────────────────────────
  const createRes = authedPost("/api/v1/monitors/", {
    name:       `loadtest-monitor-${Date.now()}`,
    type:       "metric",
    enabled:    true,
    query: {
      metric_name: "system.cpu.usage",
      aggregation: "avg",
      comparator:  "above",
      threshold:   90,
      window:      "5m",
    },
    message:    "Load test monitor alert",
    priority:   "P3",
  }, ctx, { tags: { name: "POST /monitors/" } });

  let monitorId = null;
  if (createRes.status === 200) {
    try { monitorId = createRes.json().data.id; } catch (_) { /* ignore */ }
  }
  checkStatus(createRes, createRes.status, "POST /monitors/");

  // ── Monitors: Activity Feed ────────────────────────────────────
  const activityRes = authedGet(`/api/v1/monitors/activity${qs}`, ctx, {
    tags: { name: "GET /monitors/activity" },
  });
  checkOk(activityRes, "GET /monitors/activity");

  // ── Monitors: Per-ID operations (if create succeeded) ──────────
  if (monitorId) {
    const getRes = authedGet(`/api/v1/monitors/${monitorId}`, ctx, {
      tags: { name: "GET /monitors/{id}" },
    });
    checkOk(getRes, "GET /monitors/{id}");

    const updateRes = authedPut(`/api/v1/monitors/${monitorId}`, {
      name:    `loadtest-monitor-updated-${Date.now()}`,
      enabled: false,
    }, ctx, { tags: { name: "PUT /monitors/{id}" } });
    checkStatus(updateRes, updateRes.status, "PUT /monitors/{id}");

    const testRes = authedPost(`/api/v1/monitors/${monitorId}/test`, {}, ctx, {
      tags: { name: "POST /monitors/{id}/test" },
    });
    checkStatus(testRes, testRes.status, "POST /monitors/{id}/test");

    const eventsRes = authedGet(`/api/v1/monitors/${monitorId}/events${qs}`, ctx, {
      tags: { name: "GET /monitors/{id}/events" },
    });
    checkStatus(eventsRes, eventsRes.status, "GET /monitors/{id}/events");

    const seriesRes = authedGet(`/api/v1/monitors/${monitorId}/series${qs}`, ctx, {
      tags: { name: "GET /monitors/{id}/series" },
    });
    checkStatus(seriesRes, seriesRes.status, "GET /monitors/{id}/series");

    const timelineRes = authedGet(`/api/v1/monitors/${monitorId}/status-timeline${qs}`, ctx, {
      tags: { name: "GET /monitors/{id}/status-timeline" },
    });
    checkStatus(timelineRes, timelineRes.status, "GET /monitors/{id}/status-timeline");

    // Cleanup: delete the monitor.
    const delRes = authedDel(`/api/v1/monitors/${monitorId}`, ctx, {
      tags: { name: "DELETE /monitors/{id}" },
    });
    checkStatus(delRes, delRes.status, "DELETE /monitors/{id}");
  }

  // ── Notifications: Read-only list endpoints ────────────────────
  const notifEndpoints = [
    "/api/v1/notifications/integrations",
    "/api/v1/notifications/channels/",
    "/api/v1/notifications/policies/",
    "/api/v1/notifications/templates/",
  ];

  for (const ep of notifEndpoints) {
    const tag = `GET ${ep.replace("/api/v1", "")}`;
    const res = authedGet(ep, ctx, { tags: { name: tag } });
    checkOk(res, tag);
  }

  sleep(0.5);
}
