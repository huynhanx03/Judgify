#!/bin/sh
set -eu

# Executed by the final verification phase. This contract deliberately tests
# ordering with an isolated fake host driver; it never controls a real service.
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
. "$ROOT/scripts/lib/operations.sh"
work=$(mktemp -d "$(ops_temp_root)/judgify-rollout-test.XXXXXX")
cleanup() {
	rm -rf -- "$work"
}
trap cleanup EXIT HUP INT TERM

mkdir -p "$work/state" "$work/release"
cat >"$work/release/release.env" <<'EOF'
schema_version=1
release_id=release-n
previous_release_id=release-n-1
EOF

cat >"$work/driver" <<'EOF'
#!/bin/sh
set -eu
printf '%s\n' "$1" >>"$JUDGIFY_TEST_DRIVER_LOG"
EOF
chmod 700 "$work/driver"

JUDGIFY_DEPLOY_ACK=DEPLOY_JUDGIFY_RELEASE \
JUDGIFY_DEPLOY_DRIVER="$work/driver" \
JUDGIFY_DEPLOY_STATE_ROOT="$work/state" \
JUDGIFY_TEST_DRIVER_LOG="$work/steps" \
"$ROOT/scripts/deploy/release.sh" "$work/release"

cat >"$work/expected" <<'EOF'
verify-artifacts
verify-provenance
verify-config
verify-migration-compatibility
verify-fresh-offhost-backup
migrate-expand
start-worker-claims-off
smoke-worker-exact-tuples
enable-worker-claims
disable-previous-worker-claims
drain-previous-worker
start-api-candidate
ready-api-candidate
activate-exact-capabilities
switch-ingress-atomically
drain-previous-api
verify-product-smoke
EOF

cmp "$work/expected" "$work/steps"
printf '%s\n' "rollout state-machine contract passed"
