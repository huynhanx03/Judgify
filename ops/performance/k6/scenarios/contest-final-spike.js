import { sleep } from "k6";
import { requireTarget, configuredPath } from "../lib/config.js";
import { get } from "../lib/http.js";

const target = requireTarget();

export const options = {
  scenarios: {
    contest_readers: {
      executor: "constant-vus",
      vus: 1000,
      duration: "5m",
      exec: "browse",
    },
    submission_spike_template: {
      executor: "constant-arrival-rate",
      rate: 50,
      timeUnit: "1s",
      duration: "10s",
      preAllocatedVUs: 100,
      maxVUs: 300,
      exec: "submitTemplate",
      startTime: "2m",
    },
  },
  thresholds: {
    http_req_failed: ["rate<0.01"],
    http_req_duration: ["p(95)<500"],
  },
};

export function browse() {
  get(target, configuredPath("BROWSE_PATH", "/"));
  sleep(Number(__ENV.THINK_TIME_SECONDS || 10));
}

export function submitTemplate() {
  // Exact auth and submission contracts are added only with future application integration.
  get(target, configuredPath("SUBMISSION_READINESS_PATH", "/healthz"));
}
