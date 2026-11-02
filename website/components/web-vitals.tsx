"use client";

import { useEffect } from "react";
import { useReportWebVitals } from "next/web-vitals";
import { routeKey, type VitalName } from "@/lib/observability/vitals";

// One page load contributes at most a handful of signals. The server applies
// a second per-process budget before logging any of them.
const sampled = typeof window !== "undefined" && Math.random() < 0.01;

function sendVital(name: VitalName, value: number) {
  if (!sampled || !Number.isFinite(value) || value < 0) return;
  const body = JSON.stringify({ route: routeKey(window.location.pathname), name, value });
  const url = "/telemetry/vitals";
  if (navigator.sendBeacon?.(url, new Blob([body], { type: "application/json" }))) return;
  void fetch(url, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body,
    credentials: "omit",
    keepalive: true,
  }).catch(() => undefined);
}

const report: Parameters<typeof useReportWebVitals>[0] = (metric) => {
  if (metric.name === "LCP" || metric.name === "INP" ||
      metric.name === "CLS" || metric.name === "TTFB") {
    sendVital(metric.name, metric.value);
  }
};

export function WebVitals() {
  useReportWebVitals(report);
  useEffect(() => {
    const onError = () => sendVital("client_error", 1);
    window.addEventListener("error", onError);
    return () => window.removeEventListener("error", onError);
  }, []);
  return null;
}
