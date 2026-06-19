// helpers/http.js — DRY HTTP wrappers with auth-header injection and response checks.
// Every scenario depends on these instead of raw k6 http calls.

import http from "k6/http";
import { check } from "k6";
import { BASE_URL } from "../config.js";

/**
 * Builds the standard auth headers from a bootstrap context.
 * @param {{ accessToken: string, teamId: number }} ctx
 * @returns {object}
 */
export function makeAuthHeaders(ctx) {
  return {
    Authorization:  `Bearer ${ctx.accessToken}`,
    "X-Team-Id":    String(ctx.teamId),
    "Content-Type": "application/json",
  };
}

/**
 * Authenticated GET request.
 * @param {string} path  - Relative path (e.g. "/api/v1/health").
 * @param {object} ctx   - Auth context from setup.
 * @param {object} [params] - Extra k6 params (tags, etc.).
 */
export function authedGet(path, ctx, params = {}) {
  const url = `${BASE_URL}${path}`;
  const res = http.get(url, Object.assign({ headers: makeAuthHeaders(ctx) }, params));
  return res;
}

/**
 * Authenticated POST request with JSON body.
 * @param {string} path
 * @param {object} body  - Will be JSON.stringify'd.
 * @param {object} ctx
 * @param {object} [params]
 */
export function authedPost(path, body, ctx, params = {}) {
  const url = `${BASE_URL}${path}`;
  const res = http.post(url, JSON.stringify(body), Object.assign({ headers: makeAuthHeaders(ctx) }, params));
  return res;
}

/**
 * Authenticated PUT request with JSON body.
 */
export function authedPut(path, body, ctx, params = {}) {
  const url = `${BASE_URL}${path}`;
  const res = http.put(url, JSON.stringify(body), Object.assign({ headers: makeAuthHeaders(ctx) }, params));
  return res;
}

/**
 * Authenticated DELETE request.
 */
export function authedDel(path, ctx, params = {}) {
  const url = `${BASE_URL}${path}`;
  const res = http.del(url, null, Object.assign({ headers: makeAuthHeaders(ctx) }, params));
  return res;
}

/**
 * Standard response check: status 200 + JSON success flag.
 * @param {object} res  - k6 http response.
 * @param {string} name - Human-readable tag for the check.
 * @returns {boolean}
 */
export function checkOk(res, name) {
  return check(res, {
    [`${name} status 200`]: (r) => r.status === 200,
    [`${name} success`]:    (r) => {
      try { return r.json().success === true; }
      catch (_) { return false; }
    },
  });
}

/**
 * Check for a specific status code (for non-200 expected responses).
 * @param {object} res
 * @param {number} status
 * @param {string} name
 */
export function checkStatus(res, status, name) {
  return check(res, {
    [`${name} status ${status}`]: (r) => r.status === status,
  });
}
