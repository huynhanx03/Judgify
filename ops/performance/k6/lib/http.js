import http from "k6/http";
import { check } from "k6";

export function get(target, path) {
  const response = http.get(`${target}${path}`, { tags: { endpoint: path } });
  check(response, { "response is successful": (r) => r.status >= 200 && r.status < 400 });
  return response;
}

