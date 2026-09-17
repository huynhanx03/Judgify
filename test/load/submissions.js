"use strict";

const { performance } = require("perf_hooks");
const {
  latencyAccumulator,
  pacedWork,
  percentile,
  positiveInteger,
  recordLatency,
  requestHeaders,
  required,
  testBaseURL,
  writeEvidence,
} = require("./lib");

async function main() {
  const base = testBaseURL();
  const problemID = required("JUDGIFY_SUBMISSION_PROBLEM_ID");
  const runtimeKey = required("JUDGIFY_SUBMISSION_RUNTIME_KEY");
  const sourceCode = required("JUDGIFY_SUBMISSION_SOURCE_FILE");
  const fs = require("fs");
  const source = fs.readFileSync(sourceCode, "utf8");
  const count = positiveInteger("JUDGIFY_SUBMISSION_COUNT", 500, 5000);
  const rate = positiveInteger("JUDGIFY_SUBMISSION_RPS", 50, 500);
  const concurrency = positiveInteger("JUDGIFY_SUBMISSION_CONCURRENCY", 64, 512);
  const settleSeconds = positiveInteger("JUDGIFY_SUBMISSION_SETTLE_SECONDS", 600, 3600);
  const accepted = [];
  const latency = latencyAccumulator();
  let rejected = 0;

  await pacedWork({
    total: count,
    concurrency,
    ratePerSecond: rate,
    execute: async () => {
      const key = crypto.randomUUID();
      const startedAt = performance.now();
      try {
        const response = await fetch(new URL("/submissions", base), {
          method: "POST",
          headers: requestHeaders({
            "Content-Type": "application/json",
            "Idempotency-Key": key,
            "X-Correlation-ID": crypto.randomUUID(),
          }),
          body: JSON.stringify({
            problem_id: problemID,
            runtime_key: runtimeKey,
            source_code: source,
          }),
          signal: AbortSignal.timeout(10000),
        });
        const payload = await response.json().catch(() => null);
        const id = payload?.data?.id ?? payload?.id;
        if (response.status === 202 && typeof id === "string") accepted.push(id);
        else rejected += 1;
      } catch {
        rejected += 1;
      } finally {
        recordLatency(latency, performance.now() - startedAt);
      }
    },
  });

  const unique = new Set(accepted);
  const terminal = new Map();
  const deadline = Date.now() + settleSeconds * 1000;
  while (terminal.size < unique.size && Date.now() < deadline) {
    const pending = [...unique].filter((id) => !terminal.has(id));
    for (let index = 0; index < pending.length; index += 32) {
      await Promise.all(pending.slice(index, index + 32).map(async (id) => {
        const response = await fetch(new URL(`/submissions/${encodeURIComponent(id)}`, base), {
          headers: requestHeaders(),
          signal: AbortSignal.timeout(5000),
        });
        if (!response.ok) return;
        const payload = await response.json();
        const item = payload?.data ?? payload;
        if (["accepted", "wrong_answer", "compile_error", "runtime_error", "time_limit_exceeded", "memory_limit_exceeded", "system_error", "cancelled"].includes(item?.status)) {
          terminal.set(id, item.status);
        }
      }));
    }
    if (terminal.size < unique.size) {
      await new Promise((resolve) => setTimeout(resolve, 1000));
    }
  }

  const p99Ms = percentile(latency, 0.99);
  const lost = unique.size - terminal.size;
  const passed =
    rejected === 0 &&
    unique.size === count &&
    accepted.length === unique.size &&
    lost === 0 &&
    p99Ms <= 1000;
  writeEvidence("submissions", {
    requested: count,
    accepted: accepted.length,
    unique_submissions: unique.size,
    rejected,
    terminal: terminal.size,
    lost,
    acknowledgement_p99_upper_bound_ms: p99Ms,
    terminal_status_counts: Object.fromEntries(
      [...terminal.values()].reduce((counts, status) => {
        counts.set(status, (counts.get(status) ?? 0) + 1);
        return counts;
      }, new Map()),
    ),
    passed,
  });
  if (!passed) process.exitCode = 1;
}

main().catch((error) => {
  process.stderr.write(`submission load failed: ${error.message}\n`);
  process.exitCode = 1;
});

