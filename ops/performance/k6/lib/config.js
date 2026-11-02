export function requireTarget() {
  const target = __ENV.TARGET_URL;
  if (!target) {
    throw new Error("TARGET_URL is required; load tests never choose a target implicitly");
  }
  return target.replace(/\/$/, "");
}

export function configuredPath(name, fallback) {
  return __ENV[name] || fallback;
}

