import { sleep } from "k6";
import { requireTarget, configuredPath } from "../lib/config.js";
import { get } from "../lib/http.js";

const target = requireTarget();

export const options = {
  scenarios: {
    soak: { executor: "constant-vus", vus: 200, duration: "4h" },
  },
  thresholds: {
    http_req_failed: ["rate<0.01"],
    http_req_duration: ["p(95)<500"],
  },
};

export default function () {
  get(target, configuredPath("BROWSE_PATH", "/"));
  sleep(Number(__ENV.THINK_TIME_SECONDS || 10));
}
