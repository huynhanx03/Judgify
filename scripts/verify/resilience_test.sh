#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
temporary="$(mktemp -d)"

cleanup() {
  rm -rf -- "$temporary"
}
trap cleanup EXIT

chmod 700 "$temporary"
bin="$temporary/bin"
evidence="$temporary/evidence"
volume="$temporary/volume"
mkdir -m 700 "$bin" "$evidence" "$volume"
driver_log="$temporary/chaos-driver.log"
assertion_log="$temporary/assertion-driver.log"
marker="$temporary/environment.marker"
printf '%s\n' 'judgify-resilience-test-v1' >"$marker"
printf '%s\n' 'judgify-chaos-volume-v1' >"$volume/.judgify-chaos-volume"
chmod 600 "$marker" "$volume/.judgify-chaos-volume"

cat >"$bin/node" <<'STUB'
#!/bin/sh
set -eu
case "${1##*/}" in
  websocket.js)
    printf '%s\n' '{"schema_version":1,"ready":true}' >"$JUDGIFY_WS_READY_FILE"
    printf '%s\n' '{"schema_version":1,"scenario":"websocket","passed":true}' >"$JUDGIFY_LOAD_EVIDENCE"
    ;;
  http.js|submissions.js|soak.js)
    printf '%s\n' '{"schema_version":1,"passed":true}' >"$JUDGIFY_LOAD_EVIDENCE"
    ;;
  *) exit 64 ;;
esac
STUB

cat >"$bin/chaos-driver" <<'STUB'
#!/bin/sh
set -eu
printf '%s\n' "$*" >>"$JUDGIFY_TEST_CHAOS_LOG"
STUB

cat >"$bin/assertion-driver" <<'STUB'
#!/bin/sh
set -eu
printf '%s\n' "$*" >>"$JUDGIFY_TEST_ASSERTION_LOG"
STUB

chmod 700 "$bin/node" "$bin/chaos-driver" "$bin/assertion-driver"

PATH="$bin:$PATH" \
JUDGIFY_LOAD_ACK=LOAD_ONLY_JUDGIFY_TEST \
JUDGIFY_LOAD_BASE_URL=https://judgify.test \
JUDGIFY_CHAOS_ACK=CHAOS_ONLY_JUDGIFY_TEST \
JUDGIFY_TEST_ENVIRONMENT=judgify-resilience-test \
JUDGIFY_CHAOS_DRIVER="$bin/chaos-driver" \
JUDGIFY_CHAOS_DRIVER_TIMEOUT_SECONDS=5 \
JUDGIFY_CHAOS_MARKER_FILE="$marker" \
JUDGIFY_CHAOS_VOLUME="$volume" \
JUDGIFY_CHAOS_DISK_BYTES=1048576 \
JUDGIFY_RESILIENCE_ASSERTION_DRIVER="$bin/assertion-driver" \
JUDGIFY_RESILIENCE_STEP_TIMEOUT_SECONDS=5 \
JUDGIFY_RESILIENCE_EVIDENCE_DIR="$evidence" \
JUDGIFY_WS_UNREAD_TOPIC=notifications:test-pressure \
JUDGIFY_TEST_CHAOS_LOG="$driver_log" \
JUDGIFY_TEST_ASSERTION_LOG="$assertion_log" \
  "$root/scripts/verify/resilience.sh" --run >/dev/null

[[ "$(grep -c '^prepare-fault ' "$assertion_log")" -eq 23 ]] || {
  printf '%s\n' 'resilience self-test: not every fault boundary was prepared' >&2
  exit 1
}
[[ "$(grep -c '^await-convergence ' "$assertion_log")" -eq 23 ]] || {
  printf '%s\n' 'resilience self-test: not every dependency recovery converged' >&2
  exit 1
}
[[ "$(grep -c '^verify-domain-invariants ' "$assertion_log")" -eq 25 ]] || {
  printf '%s\n' 'resilience self-test: domain assertions were skipped' >&2
  exit 1
}
[[ "$(grep -c '^sample-resources ' "$assertion_log")" -eq 24 ]] || {
  printf '%s\n' 'resilience self-test: resource samples were skipped' >&2
  exit 1
}
[[ "$(wc -l <"$driver_log" | tr -d ' ')" -eq 42 ]] || {
  printf '%s\n' 'resilience self-test: guarded host faults/recoveries were skipped' >&2
  exit 1
}
[[ "$(grep -c '"record":"scenario"' "$evidence/manifest.jsonl")" -eq 23 ]] || {
  printf '%s\n' 'resilience self-test: scenario manifest is incomplete' >&2
  exit 1
}
grep -Fq '"record":"run_complete","status":"passed"' "$evidence/manifest.jsonl" || {
  printf '%s\n' 'resilience self-test: terminal pass evidence is missing' >&2
  exit 1
}
[[ -s "$evidence/websocket-ready.json" && -s "$evidence/websocket.json" ]] || {
  printf '%s\n' 'resilience self-test: WebSocket pressure handshake evidence is missing' >&2
  exit 1
}
[[ ! -e "$volume/judgify-pressure.bin" ]] || {
  printf '%s\n' 'resilience self-test: disk pressure was not cleared' >&2
  exit 1
}

printf '%s\n' 'resilience orchestration self-test passed'
