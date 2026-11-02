#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
. "$SCRIPT_DIR/../lib/operations.sh"

state_root=${JUDGIFY_DEPLOY_STATE_ROOT:-/var/lib/judgify/deploy}
ops_require_secure_directory "$state_root" "deployment state root"
current="$state_root/current.env"

if [ ! -e "$current" ] && [ ! -L "$current" ]; then
	printf '%s\n' "deployment state: not initialized"
	exit 3
fi
ops_require_trusted_file "$current" "current deployment state"
[ "$(ops_manifest_value "$current" schema_version)" = "1" ] ||
	ops_die "unsupported current deployment state schema"

release_id=$(ops_manifest_value "$current" release_id)
previous_release_id=$(ops_manifest_value "$current" previous_release_id)
release_dir=$(ops_manifest_value "$current" release_dir)
ops_safe_path_segment "$release_id" 120 || ops_die "invalid current release identity"
ops_safe_path_segment "$previous_release_id" 120 || ops_die "invalid previous release identity"
ops_require_secure_directory "$release_dir" "current release directory"

printf '%s\n' "release_id=$release_id"
printf '%s\n' "previous_release_id=$previous_release_id"
printf '%s\n' "activated_at=$(ops_manifest_value "$current" activated_at)"

if [ -n "${JUDGIFY_DEPLOY_DRIVER:-}" ]; then
	ops_require_trusted_executable "$JUDGIFY_DEPLOY_DRIVER" "deployment driver"
	"$JUDGIFY_DEPLOY_DRIVER" status "$release_dir"
fi
