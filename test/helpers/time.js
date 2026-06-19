// helpers/time.js — Reusable time-range generators for load test scenarios.
// Returns epoch milliseconds matching the query service's expected format.

import { TIME_RANGE } from "../config.js";

/**
 * Returns { startTime, endTime } for the last N milliseconds.
 * @param {number} ms - Duration in milliseconds.
 */
function lastMs(ms) {
  const now = Date.now();
  return { startTime: now - ms, endTime: now };
}

/** Last 15 minutes. */
export function last15m() { return lastMs(15 * 60 * 1000); }

/** Last 1 hour. */
export function last1h() { return lastMs(60 * 60 * 1000); }

/** Last 6 hours. */
export function last6h() { return lastMs(6 * 60 * 60 * 1000); }

/** Last 24 hours. */
export function last24h() { return lastMs(24 * 60 * 60 * 1000); }

/** Returns time range based on TIME_RANGE config. */
export function getActiveTimeRange() {
  switch (TIME_RANGE) {
    case "15m": return last15m();
    case "6h": return last6h();
    case "24h": return last24h();
    case "1h":
    default: return last1h();
  }
}

/**
 * Converts a time range object to a query-string suffix.
 * @param {{ startTime: number, endTime: number }} range
 * @returns {string} e.g. "?startTime=123&endTime=456"
 */
export function timeParams(range) {
  return `?startTime=${range.startTime}&endTime=${range.endTime}`;
}
