#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
website="$root/website"
target="${1:-all}"
command -v node >/dev/null 2>&1 || { echo "frontend verification: node is required" >&2; exit 2; }
command -v npm >/dev/null 2>&1 || { echo "frontend verification: npm is required" >&2; exit 2; }
[[ -f "$website/package.json" && -f "$website/package-lock.json" ]] || {
  echo "frontend verification: package manifest or lockfile is missing" >&2
  exit 2
}

case "$target" in
  source|unit|build|security|all) ;;
  *) echo "usage: $0 {source|unit|build|security|all}" >&2; exit 2 ;;
esac

cd "$website"
npm ci

verify_source() {
  npm run lint
  npm run check:routes
  npm run check:user-role-contract
  npm run test:contracts
  npm run typecheck
}

verify_unit() {
  npm run test:coverage
}

verify_build() {
  npm run build
  npm run check:bundle
  verify_runtime_headers
}

verify_runtime_headers() (
  local port=3100
  local response_file
  local server_pid=""
  response_file="$(mktemp)"

  PORT="$port" HOSTNAME=127.0.0.1 node .next/standalone/server.js > /dev/null 2>&1 &
  server_pid=$!
  trap 'if [[ -n "${server_pid:-}" ]]; then kill "$server_pid" 2>/dev/null || true; fi; rm -f "$response_file"' RETURN

  for _attempt in {1..30}; do
    if curl --fail --silent --head "http://127.0.0.1:${port}/" > "$response_file"; then
      break
    fi
    sleep 0.2
  done

  if [[ ! -s "$response_file" ]]; then
    echo "frontend verification: standalone server did not respond" >&2
    return 1
  fi

  node - "$response_file" <<'NODE'
const fs = require("fs");
const headers = fs.readFileSync(process.argv[2], "utf8").toLowerCase();
const csp = headers.match(/^content-security-policy:\s*(.+)$/m)?.[1] ?? "";
for (const directive of [
  "default-src 'self'",
  "object-src 'none'",
  "base-uri 'self'",
  "form-action 'self'",
  "frame-ancestors 'none'",
]) {
  if (!csp.includes(directive)) {
    throw new Error(`frontend verification: production CSP misses ${directive}`);
  }
}
if (csp.includes("'unsafe-eval'")) {
  throw new Error("frontend verification: production CSP must not allow unsafe-eval");
}
NODE
)

verify_security() {
  npm audit --audit-level=moderate
}

case "$target" in
  source) verify_source ;;
  unit) verify_unit ;;
  build) verify_build ;;
  security) verify_security ;;
  all)
    verify_source
    verify_unit
    verify_build
    verify_security
    ;;
esac

if [[ "$target" == "build" || "$target" == "all" ]]; then
node - "$website" <<'NODE'
const fs = require("fs");
const path = require("path");
const website = process.argv[2];
const next = path.join(website, ".next");
for (const required of ["BUILD_ID", "routes-manifest.json", "standalone/server.js"]) {
  if (!fs.existsSync(path.join(next, required))) {
    throw new Error(`frontend verification: missing standalone artifact ${required}`);
  }
}
const staticRoot = path.join(next, "static");
const pending = fs.existsSync(staticRoot) ? [staticRoot] : [];
while (pending.length > 0) {
  const current = pending.pop();
  for (const entry of fs.readdirSync(current, {withFileTypes: true})) {
    const full = path.join(current, entry.name);
    if (entry.isDirectory()) pending.push(full);
    if (entry.isFile() && entry.name.endsWith(".map")) {
      throw new Error(`frontend verification: browser source map shipped: ${full}`);
    }
  }
}
NODE
fi

printf 'Frontend verification passed; target=%s\n' "$target"
