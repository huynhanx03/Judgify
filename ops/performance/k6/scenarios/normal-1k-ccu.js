import { sleep } from "k6";
import { requireTarget, configuredPath } from "../lib/config.js";
import { get } from "../lib/http.js";

const target = requireTarget();

export const options = {
  scenarios: {
    normal_1k_ccu: {
      executor: "ramping-vus",
      stages: [
        { duration: "5m", target: 1000 },
        { duration: "30m", target: 1000 },
        { duration: "5m", target: 0 },
      ],
    },
  },
  thresholds: {
    http_req_failed: ["rate<0.01"],
    http_req_duration: ["p(95)<500", "p(99)<1000"],
  },
};

export default function () {
  get(target, configuredPath("BROWSE_PATH", "/"));
  sleep(Number(__ENV.THINK_TIME_SECONDS || 10));
}
