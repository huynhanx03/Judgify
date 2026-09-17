#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
. "$SCRIPT_DIR/../lib/operations.sh"

umask 077

[ "$#" -eq 1 ] || ops_die "usage: release.sh /absolute/path/to/release-directory"
release_dir=$1
ops_require_secure_directory "$release_dir" "release directory"

[ "${JUDGIFY_DEPLOY_ACK:-}" = "DEPLOY_JUDGIFY_RELEASE" ] ||
	ops_die "set JUDGIFY_DEPLOY_ACK=DEPLOY_JUDGIFY_RELEASE"
: "${JUDGIFY_DEPLOY_DRIVER:?set the absolute host deployment driver}"
ops_require_trusted_executable "$JUDGIFY_DEPLOY_DRIVER" "deployment driver"
case "$JUDGIFY_DEPLOY_DRIVER" in
	"$release_dir"/*) ops_die "deployment driver must be outside the release directory" ;;
esac
driver_sha=$(ops_sha256 "$JUDGIFY_DEPLOY_DRIVER")

manifest="$release_dir/release.env"
ops_require_trusted_file "$manifest" "release manifest"
[ "$(ops_manifest_value "$manifest" schema_version)" = "1" ] ||
	ops_die "unsupported release manifest schema"
release_id=$(ops_manifest_value "$manifest" release_id)
previous_id=$(ops_manifest_value "$manifest" previous_release_id)
ops_safe_path_segment "$release_id" 120 || ops_die "invalid release identity"
ops_safe_path_segment "$previous_id" 120 || ops_die "invalid previous release identity"
[ "$release_id" != "$previous_id" ] || ops_die "release and previous release must differ"

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
current_tmp=
cleanup() {
	cleanup_status=$?
	trap - EXIT
	[ -z "$manifest_identity_tmp" ] || rm -f -- "$manifest_identity_tmp"
	[ -z "$driver_identity_tmp" ] || rm -f -- "$driver_identity_tmp"
	[ -z "$marker_tmp" ] || rm -f -- "$marker_tmp"
	[ -z "$current_tmp" ] || rm -f -- "$current_tmp"
	rmdir "$lock" 2>/dev/null || true
	exit "$cleanup_status"
}
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

releases_root=$(ops_ensure_secure_subdirectory "$state_root" releases "release transaction root")
transaction=$(ops_ensure_secure_subdirectory "$releases_root" "$release_id" "release transaction")
manifest_sha=$(ops_sha256 "$manifest")
activation="$transaction/activation.env"
current="$state_root/current.env"
if [ -e "$current" ] || [ -L "$current" ]; then
	ops_require_trusted_file "$current" "current deployment state"
	[ "$(ops_manifest_value "$current" schema_version)" = "1" ] ||
		ops_die "unsupported current deployment state schema"
	current_id=$(ops_manifest_value "$current" release_id)
	ops_safe_path_segment "$current_id" 120 || ops_die "invalid current release identity"
	if [ "$current_id" = "$release_id" ]; then
		[ "$(ops_manifest_value "$current" previous_release_id)" = "$previous_id" ] ||
			ops_die "active release lineage does not match the requested release"
		[ "$(ops_manifest_value "$current" release_dir)" = "$release_dir" ] ||
			ops_die "active release directory does not match the requested release"
		manifest_identity="$transaction/release-manifest.sha256"
		ops_require_trusted_file "$manifest_identity" "recorded release manifest identity"
		[ "$(sed -n '1p' "$manifest_identity")" = "$(ops_sha256 "$manifest")" ] ||
			ops_die "active release manifest differs from the activated manifest"
		driver_identity="$transaction/deployment-driver.sha256"
		ops_require_trusted_file "$driver_identity" "recorded deployment driver identity"
		[ "$(sed -n '1p' "$driver_identity")" = "$driver_sha" ] ||
			ops_die "active release driver differs from the rollout driver"
		ops_record_transaction_activation "$transaction" "$manifest_sha" "$driver_sha" "$release_id"
		printf '%s\n' "release already active: $release_id"
		exit 0
	fi
	[ "$current_id" = "$previous_id" ] ||
		ops_die "release previous identity does not match the active release"
fi
[ ! -e "$activation" ] && [ ! -L "$activation" ] || {
	ops_verify_transaction_activation "$activation" "$manifest_sha" "$driver_sha" "$release_id"
	ops_die "release identity was already activated and cannot be reused"
}
[ ! -e "$transaction/verify-product-smoke.done" ] && [ ! -L "$transaction/verify-product-smoke.done" ] ||
	ops_die "completed legacy rollout transaction cannot be reused; create a new release identity"

journal="$transaction/rollout.log"
if [ -e "$journal" ] || [ -L "$journal" ]; then
	ops_require_trusted_file "$journal" "release journal"
else
	: >"$journal"
	chmod 600 "$journal"
fi
manifest_identity="$transaction/release-manifest.sha256"
if [ -e "$manifest_identity" ] || [ -L "$manifest_identity" ]; then
	ops_require_trusted_file "$manifest_identity" "recorded release manifest identity"
	[ "$(sed -n '1p' "$manifest_identity")" = "$manifest_sha" ] ||
		ops_die "release manifest changed after rollout began"
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
		ops_die "deployment driver changed after rollout began"
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
		ops_require_trusted_file "$marker" "release step marker"
		[ "$(ops_manifest_value "$marker" schema_version)" = "1" ] ||
			ops_die "unsupported release step marker schema"
		[ "$(ops_manifest_value "$marker" release_manifest_sha256)" = "$manifest_sha" ] ||
			ops_die "release step marker belongs to a different manifest"
		[ "$(ops_manifest_value "$marker" deployment_driver_sha256)" = "$driver_sha" ] ||
			ops_die "deployment driver changed after release step completed"
		return 0
	fi
	[ "$(ops_sha256 "$manifest")" = "$manifest_sha" ] ||
		ops_die "release manifest changed before step $step"
	[ "$(ops_sha256 "$JUDGIFY_DEPLOY_DRIVER")" = "$driver_sha" ] ||
		ops_die "deployment driver changed before step $step"
	printf '%s step=%s state=started\n' "$(date -u '+%Y-%m-%dT%H:%M:%SZ')" "$step" >>"$journal"
	"$JUDGIFY_DEPLOY_DRIVER" "$step" "$release_dir"
	[ "$(ops_sha256 "$manifest")" = "$manifest_sha" ] ||
		ops_die "release manifest changed during step $step"
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

# Every transition is individually retryable. The host driver must implement
# these exact commands without inferring a later stage from an earlier one.
run_step verify-artifacts
run_step verify-provenance
run_step verify-config
run_step verify-migration-compatibility
run_step verify-fresh-offhost-backup
run_step migrate-expand
run_step start-worker-claims-off
run_step smoke-worker-exact-tuples
run_step enable-worker-claims
run_step disable-previous-worker-claims
run_step drain-previous-worker
run_step start-api-candidate
run_step ready-api-candidate
run_step activate-exact-capabilities
run_step switch-ingress-atomically
run_step drain-previous-api
run_step verify-product-smoke

current_tmp=$(mktemp "$state_root/.current.env.XXXXXX")
cat >"$current_tmp" <<EOF
schema_version=1
release_id=$release_id
previous_release_id=$previous_id
activated_at=$(date -u '+%Y-%m-%dT%H:%M:%SZ')
release_dir=$release_dir
EOF
chmod 600 "$current_tmp"
mv -f "$current_tmp" "$state_root/current.env"
current_tmp=
ops_record_transaction_activation "$transaction" "$manifest_sha" "$driver_sha" "$release_id"
printf '%s\n' "release activated: $release_id"
