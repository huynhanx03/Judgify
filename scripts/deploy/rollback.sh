#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
. "$SCRIPT_DIR/../lib/operations.sh"

umask 077

[ "$#" -eq 1 ] || ops_die "usage: rollback.sh /absolute/path/to/previous-release-directory"
target_dir=$1
ops_require_secure_directory "$target_dir" "rollback release directory"

[ "${JUDGIFY_ROLLBACK_ACK:-}" = "ROLL_BACK_JUDGIFY_BINARY_ONLY" ] ||
	ops_die "set JUDGIFY_ROLLBACK_ACK=ROLL_BACK_JUDGIFY_BINARY_ONLY"
: "${JUDGIFY_DEPLOY_DRIVER:?set the absolute host deployment driver}"
ops_require_trusted_executable "$JUDGIFY_DEPLOY_DRIVER" "deployment driver"
case "$JUDGIFY_DEPLOY_DRIVER" in
	"$target_dir"/*) ops_die "deployment driver must be outside the rollback release directory" ;;
esac
driver_sha=$(ops_sha256 "$JUDGIFY_DEPLOY_DRIVER")

target_manifest="$target_dir/release.env"
ops_require_trusted_file "$target_manifest" "rollback release manifest"
[ "$(ops_manifest_value "$target_manifest" schema_version)" = "1" ] ||
	ops_die "unsupported rollback release manifest schema"
target_id=$(ops_manifest_value "$target_manifest" release_id)
ops_safe_path_segment "$target_id" 120 || ops_die "invalid rollback release identity"

state_root=${JUDGIFY_DEPLOY_STATE_ROOT:-/var/lib/judgify/deploy}
ops_require_secure_directory "$state_root" "deployment state root"
case "$JUDGIFY_DEPLOY_DRIVER" in
	"$state_root"/*) ops_die "deployment driver must be outside the writable deployment state root" ;;
esac

lock="$state_root/.deploy.lock"
if ! mkdir "$lock" 2>/dev/null; then
	ops_die "another deployment or rollback owns the host lock"
fi
manifest_identity_tmp=
driver_identity_tmp=
marker_tmp=
state_tmp=
cleanup() {
	cleanup_status=$?
	trap - EXIT
	[ -z "$manifest_identity_tmp" ] || rm -f -- "$manifest_identity_tmp"
	[ -z "$driver_identity_tmp" ] || rm -f -- "$driver_identity_tmp"
	[ -z "$marker_tmp" ] || rm -f -- "$marker_tmp"
	[ -z "$state_tmp" ] || rm -f -- "$state_tmp"
	rmdir "$lock" 2>/dev/null || true
	exit "$cleanup_status"
}
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

current="$state_root/current.env"
ops_require_trusted_file "$current" "current deployment state"
[ "$(ops_manifest_value "$current" schema_version)" = "1" ] ||
	ops_die "unsupported current deployment state schema"
current_id=$(ops_manifest_value "$current" release_id)
ops_safe_path_segment "$current_id" 120 || ops_die "invalid current release identity"
expected_target_id=$(ops_manifest_value "$current" previous_release_id)
ops_safe_path_segment "$expected_target_id" 120 || ops_die "invalid previous release identity"
if [ "$target_id" = "$current_id" ]; then
	previous_id=$(ops_manifest_value "$current" previous_release_id)
	ops_safe_path_segment "$previous_id" 120 || ops_die "invalid rollback source identity"
	[ "$(ops_manifest_value "$current" release_dir)" = "$target_dir" ] ||
		ops_die "active release directory does not match the rollback target"
	grep -qx 'rollback=true' "$current" ||
		ops_die "rollback target is current without rollback state"
	rollbacks_root=$(ops_ensure_secure_subdirectory "$state_root" rollbacks "rollback transaction root")
	transaction=$(ops_ensure_secure_subdirectory "$rollbacks_root" "${previous_id}-to-${target_id}" "rollback transaction")
	manifest_identity="$transaction/release-manifest.sha256"
	ops_require_trusted_file "$manifest_identity" "recorded rollback manifest identity"
	[ "$(sed -n '1p' "$manifest_identity")" = "$(ops_sha256 "$target_manifest")" ] ||
		ops_die "active rollback manifest differs from the activated manifest"
	driver_identity="$transaction/deployment-driver.sha256"
	ops_require_trusted_file "$driver_identity" "recorded deployment driver identity"
	[ "$(sed -n '1p' "$driver_identity")" = "$driver_sha" ] ||
		ops_die "active rollback driver differs from the rollback driver"
	ops_record_transaction_activation "$transaction" "$(ops_sha256 "$target_manifest")" "$driver_sha" "$target_id"
	printf '%s\n' "binary rollback already active: $target_id"
	exit 0
fi
[ "$target_id" = "$expected_target_id" ] ||
	ops_die "rollback target must be the immediately previous N-1 release"

rollbacks_root=$(ops_ensure_secure_subdirectory "$state_root" rollbacks "rollback transaction root")
transaction=$(ops_ensure_secure_subdirectory "$rollbacks_root" "${current_id}-to-${target_id}" "rollback transaction")
manifest_sha=$(ops_sha256 "$target_manifest")
activation="$transaction/activation.env"
[ ! -e "$activation" ] && [ ! -L "$activation" ] || {
	ops_verify_transaction_activation "$activation" "$manifest_sha" "$driver_sha" "$target_id"
	ops_die "rollback transaction was already activated and cannot be reused"
}
[ ! -e "$transaction/verify-product-smoke.done" ] && [ ! -L "$transaction/verify-product-smoke.done" ] ||
	ops_die "completed legacy rollback transaction cannot be reused"
journal="$transaction/rollback.log"
if [ -e "$journal" ] || [ -L "$journal" ]; then
	ops_require_trusted_file "$journal" "rollback journal"
else
	: >"$journal"
	chmod 600 "$journal"
fi
manifest_identity="$transaction/release-manifest.sha256"
if [ -e "$manifest_identity" ] || [ -L "$manifest_identity" ]; then
	ops_require_trusted_file "$manifest_identity" "recorded rollback manifest identity"
	[ "$(sed -n '1p' "$manifest_identity")" = "$manifest_sha" ] ||
		ops_die "rollback release manifest changed after rollback began"
else
	manifest_identity_tmp=$(mktemp "$transaction/.release-manifest.sha256.XXXXXX")
	printf '%s\n' "$manifest_sha" >"$manifest_identity_tmp"
	chmod 600 "$manifest_identity_tmp"
	mv -f "$manifest_identity_tmp" "$manifest_identity"
	manifest_identity_tmp=
fi
driver_identity="$transaction/deployment-driver.sha256"
if [ -e "$driver_identity" ] || [ -L "$driver_identity" ]; then
	ops_require_trusted_file "$driver_identity" "recorded deployment driver identity"
	[ "$(sed -n '1p' "$driver_identity")" = "$driver_sha" ] ||
		ops_die "deployment driver changed after rollback began"
else
	driver_identity_tmp=$(mktemp "$transaction/.deployment-driver.sha256.XXXXXX")
	printf '%s\n' "$driver_sha" >"$driver_identity_tmp"
	chmod 600 "$driver_identity_tmp"
	mv -f "$driver_identity_tmp" "$driver_identity"
	driver_identity_tmp=
fi

run_step() {
	step=$1
	marker="$transaction/$step.done"
	if [ -e "$marker" ] || [ -L "$marker" ]; then
		ops_require_trusted_file "$marker" "rollback step marker"
		[ "$(ops_manifest_value "$marker" schema_version)" = "1" ] ||
			ops_die "unsupported rollback step marker schema"
		[ "$(ops_manifest_value "$marker" release_manifest_sha256)" = "$manifest_sha" ] ||
			ops_die "rollback step marker belongs to a different manifest"
		[ "$(ops_manifest_value "$marker" deployment_driver_sha256)" = "$driver_sha" ] ||
			ops_die "deployment driver changed after rollback step completed"
		return 0
	fi
	[ "$(ops_sha256 "$target_manifest")" = "$manifest_sha" ] ||
		ops_die "rollback manifest changed before step $step"
	[ "$(ops_sha256 "$JUDGIFY_DEPLOY_DRIVER")" = "$driver_sha" ] ||
		ops_die "deployment driver changed before step $step"
	printf '%s step=%s state=started\n' "$(date -u '+%Y-%m-%dT%H:%M:%SZ')" "$step" >>"$journal"
	"$JUDGIFY_DEPLOY_DRIVER" "$step" "$target_dir" "$current"
	[ "$(ops_sha256 "$target_manifest")" = "$manifest_sha" ] ||
		ops_die "rollback manifest changed during step $step"
	[ "$(ops_sha256 "$JUDGIFY_DEPLOY_DRIVER")" = "$driver_sha" ] ||
		ops_die "deployment driver changed during step $step"
	printf '%s step=%s state=completed\n' "$(date -u '+%Y-%m-%dT%H:%M:%SZ')" "$step" >>"$journal"
	marker_tmp=$(mktemp "$transaction/.$step.done.XXXXXX")
	cat >"$marker_tmp" <<EOF
schema_version=1
release_manifest_sha256=$manifest_sha
deployment_driver_sha256=$driver_sha
completed_at=$(date -u '+%Y-%m-%dT%H:%M:%SZ')
EOF
	chmod 600 "$marker_tmp"
	mv -f "$marker_tmp" "$marker"
	marker_tmp=
}

# Database rollback is deliberately absent. The target must declare
# compatibility with the already-applied schema before any process changes.
run_step verify-artifacts
run_step verify-provenance
run_step verify-current-schema-compatible
run_step start-worker-claims-off
run_step smoke-worker-exact-tuples
run_step enable-worker-claims
run_step disable-current-worker-claims
run_step drain-current-worker
run_step start-api-candidate
run_step ready-api-candidate
run_step activate-exact-capabilities
run_step switch-ingress-atomically
run_step drain-current-api
run_step verify-product-smoke

state_tmp=$(mktemp "$state_root/.current.env.XXXXXX")
cat >"$state_tmp" <<EOF
schema_version=1
release_id=$target_id
previous_release_id=$current_id
activated_at=$(date -u '+%Y-%m-%dT%H:%M:%SZ')
release_dir=$target_dir
rollback=true
EOF
chmod 600 "$state_tmp"
mv -f "$state_tmp" "$current"
state_tmp=
ops_record_transaction_activation "$transaction" "$manifest_sha" "$driver_sha" "$target_id"
printf '%s\n' "binary rollback activated: $target_id"
