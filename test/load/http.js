"use strict";

const { performance } = require("perf_hooks");
const {
  latencyAccumulator,
  pacedWork,
  percentile,
  positiveInteger,
  recordLatency,
  requestHeaders,
  testBaseURL,
  writeEvidence,
} = require("./lib");

async function main() {
  const base = testBaseURL();
  const rate = positiveInteger("JUDGIFY_HTTP_RPS", 250, 5000);
  const durationSeconds = positiveInteger("JUDGIFY_HTTP_DURATION_SECONDS", 60, 14400);
  const concurrency = positiveInteger("JUDGIFY_HTTP_CONCURRENCY", 128, 2048);
  const paths = (process.env.JUDGIFY_HTTP_PATHS ?? "/ping,/problems,/contests,/materials")
    .split(",")
    .map((path) => path.trim())
    .filter((path) => /^\/[A-Za-z0-9_?&=./:%-]*$/.test(path))
    .slice(0, 16);
  if (paths.length === 0) throw new Error("JUDGIFY_HTTP_PATHS has no safe path");

  const total = rate * durationSeconds;
  const latency = latencyAccumulator();
  const statuses = {};
  let networkErrors = 0;
  await pacedWork({
    total,
    concurrency,
    ratePerSecond: rate,
    execute: async (index) => {
      const startedAt = performance.now();
      try {
        const response = await fetch(new URL(paths[index % paths.length], base), {
          method: "GET",
          headers: requestHeaders(),
          signal: AbortSignal.timeout(10_000),
        });
        await response.body?.cancel();
        const statusClass = `${Math.floor(response.status / 100)}xx`;
        statuses[statusClass] = (statuses[statusClass] ?? 0) + 1;
      } catch {
        networkErrors += 1;
      } finally {
        recordLatency(latency, performance.now() - startedAt);
      }
    },
  });

  const errors = networkErrors + (statuses["5xx"] ?? 0);
  const errorRatio = errors / total;
  const p95Ms = percentile(latency, 0.95);
  const maximumErrorRatio = Number(process.env.JUDGIFY_HTTP_MAX_ERROR_RATIO ?? "0.005");
  const maximumP95Ms = positiveInteger("JUDGIFY_HTTP_MAX_P95_MS", 500, 30000);
  writeEvidence("mixed-http", {
    requested: total,
    statuses,
    network_errors: networkErrors,
    error_ratio: errorRatio,
    latency_ms: {
      p95_upper_bound: p95Ms,
      maximum: latency.maximumMs,
      average: latency.totalMs / Math.max(latency.count, 1),
    },
    passed: errorRatio <= maximumErrorRatio && p95Ms <= maximumP95Ms,
  });
  if (errorRatio > maximumErrorRatio || p95Ms > maximumP95Ms) process.exitCode = 1;
}

main().catch((error) => {
  process.stderr.write(`http load failed: ${error.message}\n`);
  process.exitCode = 1;
});

