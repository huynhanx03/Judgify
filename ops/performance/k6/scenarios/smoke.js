import { sleep } from "k6";
import { requireTarget, configuredPath } from "../lib/config.js";
import { get } from "../lib/http.js";

const target = requireTarget();

export const options = {
  vus: 1,
  duration: "15s",
  thresholds: {
    http_req_failed: ["rate<0.01"],
    http_req_duration: ["p(95)<1000"],
  },
};

export default function () {
  get(target, configuredPath("SMOKE_PATH", "/healthz"));
  sleep(1);
}
