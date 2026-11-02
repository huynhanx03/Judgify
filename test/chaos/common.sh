#!/bin/sh

CHAOS_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
. "$CHAOS_DIR/../../scripts/lib/operations.sh"

chaos_guard() {
	[ "${JUDGIFY_CHAOS_ACK:-}" = "CHAOS_ONLY_JUDGIFY_TEST" ] ||
		ops_die "set JUDGIFY_CHAOS_ACK=CHAOS_ONLY_JUDGIFY_TEST"
	[ "${JUDGIFY_TEST_ENVIRONMENT:-}" = "judgify-resilience-test" ] ||
		ops_die "JUDGIFY_TEST_ENVIRONMENT must equal judgify-resilience-test"
	: "${JUDGIFY_CHAOS_MARKER_FILE:?set the isolated environment marker}"
	ops_require_trusted_file "$JUDGIFY_CHAOS_MARKER_FILE" "chaos environment marker"
	[ "$(sed -n '1p' "$JUDGIFY_CHAOS_MARKER_FILE")" = "judgify-resilience-test-v1" ] ||
		ops_die "chaos environment marker is invalid"
}

chaos_driver() {
	: "${JUDGIFY_CHAOS_DRIVER:?set the absolute isolated-host chaos driver}"
	ops_require_trusted_executable "$JUDGIFY_CHAOS_DRIVER" "chaos driver"
	chaos_timeout=${JUDGIFY_CHAOS_DRIVER_TIMEOUT_SECONDS:-120}
	case "$chaos_timeout" in
		''|*[!0-9]*) ops_die "chaos driver timeout must be an integer" ;;
	esac
	[ "$chaos_timeout" -ge 5 ] && [ "$chaos_timeout" -le 900 ] ||
		ops_die "chaos driver timeout must be between 5 and 900 seconds"
	ops_require_command timeout
	printf '%s\n' "planned chaos action: $*"
	timeout --signal=TERM --kill-after=5s "${chaos_timeout}s" \
		"$JUDGIFY_CHAOS_DRIVER" "$@"
}
