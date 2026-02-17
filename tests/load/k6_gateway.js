// k6 load test for Gatify gateway
// Install: brew install k6  (or https://k6.io/docs/get-started/installation/)
//
// Usage:
//   k6 run tests/load/k6_gateway.js                              # default (local)
//   k6 run -e BASE_URL=https://api.siruyy.cloud tests/load/k6_gateway.js  # live
//
// Scenarios:
//   1. smoke      – 1 VU, 10s — sanity check
//   2. load       – ramp to 50 VUs over 1m, sustain 2m, ramp down
//   3. rate_limit – 1 VU firing 200 req to trigger 429s

import http from "k6/http";
import { check, sleep, group } from "k6";
import { Rate, Trend, Counter } from "k6/metrics";

// ── Custom metrics ──────────────────────────────────────────────
const rateLimited = new Rate("rate_limited");
const rateLimitRemaining = new Trend("ratelimit_remaining");
const blockedRequests = new Counter("blocked_requests");

// ── Configuration ───────────────────────────────────────────────
const BASE = (__ENV.BASE_URL || "http://localhost:3000").replace(/\/+$/, "");
const QUICK = __ENV.QUICK === "true";
const REQUIRE_BLOCKED = __ENV.REQUIRE_BLOCKED !== "false" && !QUICK;

function normalizePath(path, fallback) {
  const raw = (path || fallback || "").trim();
  const noLeadingSlashes = raw.replace(/^\/+/, "");
  return `/${noLeadingSlashes}`;
}

const PROXY_HEALTH_PATH = normalizePath(__ENV.PROXY_HEALTH_PATH, "/proxy/health");
const PROXY_API_PATH = normalizePath(__ENV.PROXY_API_PATH, "/proxy/api");

const loadVUs = QUICK ? 10 : 50;
const loadRampUp = QUICK ? "10s" : "30s";
const loadSustain = QUICK ? "30s" : "2m";
const loadRampDown = QUICK ? "10s" : "30s";
const rateLimitIterations = QUICK ? 20 : 200;
const rateLimitStartTime = QUICK ? "1m5s" : "3m30s";

// Mark 429 as expected for proxied requests only, since rate limiting is
// intentionally exercised by those scenarios.
const proxyResponseCallback = http.expectedStatuses(200, 429);

export const options = {
  summaryTrendStats: ["avg", "min", "med", "max", "p(90)", "p(95)", "p(99)"],
  scenarios: {
    smoke: {
      executor: "constant-vus",
      vus: 1,
      duration: "10s",
      tags: { scenario: "smoke" },
    },
    load: {
      executor: "ramping-vus",
      startVUs: 0,
      stages: [
        { duration: loadRampUp, target: loadVUs },
        { duration: loadSustain, target: loadVUs },
        { duration: loadRampDown, target: 0 },
      ],
      startTime: "15s", // start after smoke
      tags: { scenario: "load" },
    },
    rate_limit: {
      executor: "per-vu-iterations",
      vus: 1,
      iterations: rateLimitIterations,
      startTime: rateLimitStartTime,
      tags: { scenario: "rate_limit" },
    },
  },
  thresholds: {
    // Gateway health must always succeed
    "http_req_duration{scenario:smoke}": [QUICK ? "p(95)<10000" : "p(95)<500"],
    // Under load, p95 should stay under 1s
    "http_req_duration{scenario:load}": [QUICK ? "p(95)<5000" : "p(95)<1000"],
    // Keep transport/network-level failures low.
    http_req_failed: [QUICK ? "rate<0.02" : "rate<0.01"],
    // In full mode, at least some requests should be rate-limited in burst traffic.
    // Quick mode is optimized for fast CI signal and does not require 429 responses.
    "blocked_requests": [REQUIRE_BLOCKED ? "count>0" : "count>=0"],
  },
};

// ── Helpers ─────────────────────────────────────────────────────
function checkRateLimitHeaders(res) {
  const remaining =
    res.headers["X-RateLimit-Remaining"] ??
    res.headers["X-Ratelimit-Remaining"];

  if (remaining !== undefined) {
    const parsed = parseInt(remaining, 10);
    if (!Number.isNaN(parsed)) {
      rateLimitRemaining.add(parsed);
    }
  }

  return res.status === 429;
}

// ── Default function (runs per iteration for each scenario) ─────
export default function () {
  group("health", () => {
    const res = http.get(`${BASE}/health`);
    check(res, {
      "health status 200": (r) => r.status === 200,
      "health has service field": (r) => {
        try {
          return JSON.parse(r.body).service !== undefined;
        } catch {
          return false;
        }
      },
    });
  });

  group("proxy_health", () => {
    const res = http.get(`${BASE}${PROXY_HEALTH_PATH}`, {
      responseCallback: proxyResponseCallback,
    });
    const is429 = checkRateLimitHeaders(res);
    rateLimited.add(is429);
    if (is429) blockedRequests.add(1);

    check(res, {
      "proxy returns 200 or 429": (r) => r.status === 200 || r.status === 429,
    });
  });

  group("proxy_api", () => {
    const res = http.get(`${BASE}${PROXY_API_PATH}`, {
      responseCallback: proxyResponseCallback,
    });
    const is429 = checkRateLimitHeaders(res);
    rateLimited.add(is429);
    if (is429) blockedRequests.add(1);

    check(res, {
      "api returns 200 or 429": (r) => r.status === 200 || r.status === 429,
    });
  });

  // Slight pause between iterations to avoid pure CPU spin
  sleep(0.1);
}

// ── Teardown ────────────────────────────────────────────────────
export function handleSummary(data) {
  const metricValue = (name, stat) => {
    const metric = data.metrics?.[name];
    if (!metric) return null;

    // k6 summary data commonly exposes stats directly on the metric object.
    if (typeof metric[stat] === "number") return metric[stat];

    // Some adapters expose under `values`.
    if (metric.values && typeof metric.values[stat] === "number") {
      return metric.values[stat];
    }

    return null;
  };

  const totalReqs = metricValue("http_reqs", "count") ?? 0;
  const blocked = metricValue("blocked_requests", "count") ?? 0;
  const failRateRaw =
    metricValue("http_req_failed", "rate") ?? metricValue("http_req_failed", "value");
  const smokeP95Raw = metricValue("http_req_duration{scenario:smoke}", "p(95)");
  const loadP95Raw = metricValue("http_req_duration{scenario:load}", "p(95)");
  const p95Raw = metricValue("http_req_duration", "p(95)");
  const p99Raw = metricValue("http_req_duration", "p(99)");
  const p95 = typeof p95Raw === "number" ? p95Raw.toFixed(2) : "N/A";
  const p99 = typeof p99Raw === "number" ? p99Raw.toFixed(2) : "N/A";
  const smokeP95 = typeof smokeP95Raw === "number" ? smokeP95Raw.toFixed(2) : "N/A";
  const loadP95 = typeof loadP95Raw === "number" ? loadP95Raw.toFixed(2) : "N/A";
  const failRate = typeof failRateRaw === "number" ? (failRateRaw * 100).toFixed(3) : "N/A";

  const summary = `
========================================
  Gatify Load Test Summary
========================================
  Mode:              ${QUICK ? "quick" : "full"}
  Total requests:    ${totalReqs}
  Transport fail %:  ${failRate}%
  Blocked (429):     ${blocked}
  Block rate:        ${totalReqs > 0 ? ((blocked / totalReqs) * 100).toFixed(1) : 0}%
  Smoke p95:         ${smokeP95} ms
  Load p95:          ${loadP95} ms
  p95 latency:       ${p95} ms
  p99 latency:       ${p99} ms
========================================
`;

  console.log(summary);

  // Return default text summary
  return {
    stdout: summary,
  };
}
