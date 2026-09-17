"use strict";

const {
  positiveInteger,
  required,
  writeEvidence,
} = require("./lib");

function safeMetricsURL() {
  const value = new URL(required("JUDGIFY_LOAD_METRICS_URL"));
  const safeHost =
    value.hostname === "127.0.0.1" ||
    value.hostname === "localhost" ||
    value.hostname.endsWith(".test");
  if (
    process.env.JUDGIFY_LOAD_ACK !== "LOAD_ONLY_JUDGIFY_TEST" ||
    !safeHost ||
    !["http:", "https:"].includes(value.protocol)
  ) {
    throw new Error("metrics URL must be loopback/.test in the acknowledged test environment");
  }
  return value;
}

function parseMetric(document, name) {
  const line = document
    .split("\n")
    .find((candidate) => candidate.startsWith(`${name} `));
  if (!line) throw new Error(`required metric is absent: ${name}`);
  const value = Number(line.slice(name.length + 1));
  if (!Number.isFinite(value) || value < 0) throw new Error(`invalid metric: ${name}`);
  return value;
}

async function main() {
  const url = safeMetricsURL();
  const durationSeconds = positiveInteger("JUDGIFY_SOAK_DURATION_SECONDS", 14400, 28800);
  const intervalSeconds = positiveInteger("JUDGIFY_SOAK_SAMPLE_INTERVAL_SECONDS", 15, 300);
  const samples = [];
  const deadline = Date.now() + durationSeconds * 1000;
  while (Date.now() < deadline) {
    const response = await fetch(url, { signal: AbortSignal.timeout(5000) });
    if (!response.ok) throw new Error(`metrics endpoint returned ${response.status}`);
    const document = await response.text();
    samples.push({
      at: new Date().toISOString(),
      resident_memory_bytes: parseMetric(document, "judgify_process_resident_memory_bytes"),
      heap_bytes: parseMetric(document, "judgify_process_heap_bytes"),
      goroutines: parseMetric(document, "judgify_process_goroutines"),
      open_fds: parseMetric(document, "judgify_process_open_fds"),
    });
    await new Promise((resolve) => setTimeout(resolve, intervalSeconds * 1000));
  }

  const first = samples[0];
  const last = samples.at(-1);
  const ratio = (end, start) => end / Math.max(start, 1);
  const growth = {
    resident_memory: ratio(last.resident_memory_bytes, first.resident_memory_bytes),
    heap: ratio(last.heap_bytes, first.heap_bytes),
    goroutines: ratio(last.goroutines, first.goroutines),
    open_fds: ratio(last.open_fds, first.open_fds),
  };
  const passed =
    growth.resident_memory <= 1.2 &&
    growth.heap <= 1.2 &&
    growth.goroutines <= 1.1 &&
    growth.open_fds <= 1.1;
  writeEvidence("soak-resources", {
    duration_seconds: durationSeconds,
    sample_interval_seconds: intervalSeconds,
    sample_count: samples.length,
    growth_ratio: growth,
    first,
    last,
    passed,
  });
  if (!passed) process.exitCode = 1;
}

main().catch((error) => {
  process.stderr.write(`soak sampling failed: ${error.message}\n`);
  process.exitCode = 1;
});

