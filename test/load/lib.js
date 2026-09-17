"use strict";

const fs = require("fs");
const { performance } = require("perf_hooks");

const LATENCY_BUCKETS_MS = [
  5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000, 30000,
];

function required(name) {
  const value = process.env[name];
  if (!value) throw new Error(`${name} is required`);
  return value;
}

function positiveInteger(name, fallback, maximum) {
  const raw = process.env[name] ?? String(fallback);
  if (!/^[1-9][0-9]*$/.test(raw)) throw new Error(`${name} must be a positive integer`);
  const value = Number(raw);
  if (!Number.isSafeInteger(value) || value > maximum) {
    throw new Error(`${name} exceeds the safety bound ${maximum}`);
  }
  return value;
}

function testBaseURL() {
  const value = new URL(required("JUDGIFY_LOAD_BASE_URL"));
  const explicitlyTest = process.env.JUDGIFY_LOAD_ACK === "LOAD_ONLY_JUDGIFY_TEST";
  const safeHost =
    value.hostname === "127.0.0.1" ||
    value.hostname === "localhost" ||
    value.hostname.endsWith(".test");
  if (!explicitlyTest || !safeHost || !["http:", "https:"].includes(value.protocol)) {
    throw new Error("load target must be loopback/.test with JUDGIFY_LOAD_ACK=LOAD_ONLY_JUDGIFY_TEST");
  }
  return value;
}

function requestHeaders(extra = {}) {
  const headers = {
    Accept: "application/json",
    ...extra,
  };
  if (process.env.JUDGIFY_LOAD_COOKIE) headers.Cookie = process.env.JUDGIFY_LOAD_COOKIE;
  if (process.env.JUDGIFY_LOAD_CSRF) {
    headers["X-CSRF-Token"] = process.env.JUDGIFY_LOAD_CSRF;
  }
  return headers;
}

function latencyAccumulator() {
  return {
    count: 0,
    totalMs: 0,
    maximumMs: 0,
    buckets: LATENCY_BUCKETS_MS.map((upperMs) => ({ upperMs, count: 0 })),
  };
}

function recordLatency(accumulator, durationMs) {
  accumulator.count += 1;
  accumulator.totalMs += durationMs;
  accumulator.maximumMs = Math.max(accumulator.maximumMs, durationMs);
  for (const bucket of accumulator.buckets) {
    if (durationMs <= bucket.upperMs) bucket.count += 1;
  }
}

function percentile(accumulator, ratio) {
  const wanted = Math.ceil(accumulator.count * ratio);
  const bucket = accumulator.buckets.find((candidate) => candidate.count >= wanted);
  return bucket?.upperMs ?? accumulator.maximumMs;
}

async function pacedWork({ total, concurrency, ratePerSecond, execute }) {
  let next = 0;
  const startedAt = performance.now();
  async function worker() {
    for (;;) {
      const index = next++;
      if (index >= total) return;
      const scheduledAt = startedAt + (index * 1000) / ratePerSecond;
      const delay = scheduledAt - performance.now();
      if (delay > 0) await new Promise((resolve) => setTimeout(resolve, delay));
      await execute(index);
    }
  }
  await Promise.all(Array.from({ length: Math.min(concurrency, total) }, worker));
}

function writeEvidence(name, value) {
  const path = required("JUDGIFY_LOAD_EVIDENCE");
  const document = {
    schema_version: 1,
    scenario: name,
    completed_at: new Date().toISOString(),
    ...value,
  };
  fs.writeFileSync(path, `${JSON.stringify(document, null, 2)}\n`, {
    encoding: "utf8",
    mode: 0o600,
    flag: "wx",
  });
}

module.exports = {
  latencyAccumulator,
  pacedWork,
  percentile,
  positiveInteger,
  recordLatency,
  requestHeaders,
  required,
  testBaseURL,
  writeEvidence,
};

