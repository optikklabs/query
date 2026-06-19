// scenarios/saturation_database.js — Load test for database saturation endpoints.

import { sleep } from "k6";
import { DEFAULT_THRESHOLDS, activeProfile } from "../config.js";
import { bootstrapAuth } from "../helpers/setup.js";
import { authedGet, checkOk } from "../helpers/http.js";
import { getActiveTimeRange, timeParams } from "../helpers/time.js";

export const options = Object.assign(
  { thresholds: DEFAULT_THRESHOLDS },
  activeProfile(),
);

export function setup() {
  return bootstrapAuth();
}

export default function (ctx) {
  const qs = timeParams(getActiveTimeRange());

  const endpoints = [
    "/api/v1/saturation/datastores/systems",
    "/api/v1/saturation/database/latency/by-system",
    "/api/v1/saturation/database/slow-queries/patterns",
    "/api/v1/saturation/database/ops/by-system",
  ];

  for (const ep of endpoints) {
    const tag = `GET ${ep.replace("/api/v1", "")}`;
    const res = authedGet(`${ep}${qs}`, ctx, { tags: { name: tag } });
    checkOk(res, tag);
  }

  sleep(0.5);
}
