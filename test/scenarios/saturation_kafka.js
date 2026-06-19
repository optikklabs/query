// scenarios/saturation_kafka.js — Load test for Kafka saturation endpoints.

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
    // Producer
    "/api/v1/saturation/kafka/produce-rate-by-topic",
    // Consumer
    "/api/v1/saturation/kafka/consume-rate-by-topic",
    "/api/v1/saturation/kafka/consumer-lag-by-group",
    // Explorer — Cluster
    "/api/v1/saturation/kafka/cluster/health",
    // Explorer — Topics
    "/api/v1/saturation/kafka/topics/throughput",
    "/api/v1/saturation/kafka/topics/lag",
    "/api/v1/saturation/kafka/topics/consumers",
    "/api/v1/saturation/kafka/topics/backlog",
    // Explorer — Consumer Groups
    "/api/v1/saturation/kafka/groups/partitions",
    "/api/v1/saturation/kafka/groups/commits",
    "/api/v1/saturation/kafka/groups/fetches",
    "/api/v1/saturation/kafka/groups/health",
  ];

  for (const ep of endpoints) {
    const tag = `GET ${ep.replace("/api/v1", "")}`;
    const res = authedGet(`${ep}${qs}`, ctx, { tags: { name: tag } });
    checkOk(res, tag);
  }

  sleep(0.5);
}
