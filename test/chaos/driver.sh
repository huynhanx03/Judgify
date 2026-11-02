#!/bin/sh
# Isolated-host chaos driver.
#
# test/chaos/{process,postgres,docker}.sh validate the request and then hand the
# actual fault injection here, so the host-control policy lives in one place.
# This implementation drives the one-host Docker Compose topology described by
# deploy/compose.production.yml.
#
#   process        kill|restart   api|judge-worker
#   postgres       pause|resume|restart
#   sandbox-daemon pause|resume|restart
#
# Every action is confined to the Compose project named by
# JUDGIFY_CHAOS_COMPOSE_PROJECT. The driver refuses to run against a project that
# is not explicitly marked as a disposable test environment.
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)
. "$ROOT/scripts/lib/operations.sh"

[ "${JUDGIFY_CHAOS_ACK:-}" = "CHAOS_ONLY_JUDGIFY_TEST" ] ||
	ops_die "chaos driver requires the explicit test acknowledgement"
[ "${JUDGIFY_TEST_ENVIRONMENT:-}" = "judgify-resilience-test" ] ||
	ops_die "chaos driver requires the isolated test environment"

: "${JUDGIFY_CHAOS_COMPOSE_PROJECT:?set the disposable Compose project name}"
: "${JUDGIFY_CHAOS_COMPOSE_FILE:?set the Compose file that defines the test topology}"
ops_require_regular_file "$JUDGIFY_CHAOS_COMPOSE_FILE" "chaos compose file"
ops_safe_token "$JUDGIFY_CHAOS_COMPOSE_PROJECT" 63 ||
	ops_die "chaos compose project name is malformed"
case "$JUDGIFY_CHAOS_COMPOSE_PROJECT" in
	*judgify-resilience-test*) ;;
	*) ops_die "chaos compose project must be a resilience-test project" ;;
esac
ops_require_command docker

compose() {
	docker compose \
		--project-name "$JUDGIFY_CHAOS_COMPOSE_PROJECT" \
		--file "$JUDGIFY_CHAOS_COMPOSE_FILE" \
		"$@"
}

# A service must already be part of the disposable project before it is touched.
require_service() {
	compose ps --services 2>/dev/null | grep -Fxq "$1" ||
		ops_die "service $1 is not part of the disposable chaos project"
}

[ "$#" -ge 2 ] || ops_die "usage: driver.sh process|postgres|sandbox-daemon action [target]"
domain=$1
action=$2
target=${3:-}

case "$domain" in
	process)
		[ -n "$target" ] || ops_die "process chaos requires a target service"
		case "$target" in
			api|judge-worker) ;;
			*) ops_die "unsupported process chaos target" ;;
		esac
		service=$target
		[ "$service" != "judge-worker" ] || service=worker
		require_service "$service"
		case "$action" in
			# SIGKILL, not a graceful stop: recovery must be proven from an
			# abrupt loss, which is what a real crash looks like.
			kill)    compose kill --signal SIGKILL "$service" ;;
			restart) compose up --detach --no-deps "$service" ;;
			*) ops_die "unsupported process chaos action" ;;
		esac
		;;
	postgres)
		require_service postgres
		case "$action" in
			pause)   compose pause postgres ;;
			resume)  compose unpause postgres ;;
			restart) compose restart postgres ;;
			*) ops_die "unsupported postgres chaos action" ;;
		esac
		;;
	sandbox-daemon)
		# The sandbox daemon is the rootless Docker endpoint the worker uses to
		# execute contestant code; it is never the daemon running this project.
		: "${JUDGIFY_CHAOS_SANDBOX_CONTAINER:?set the isolated sandbox daemon container}"
		ops_safe_token "$JUDGIFY_CHAOS_SANDBOX_CONTAINER" 63 ||
			ops_die "sandbox daemon container name is malformed"
		case "$JUDGIFY_CHAOS_SANDBOX_CONTAINER" in
			*judgify-resilience-test*) ;;
			*) ops_die "sandbox daemon container must belong to the test environment" ;;
		esac
		case "$action" in
			pause)   docker pause "$JUDGIFY_CHAOS_SANDBOX_CONTAINER" ;;
			resume)  docker unpause "$JUDGIFY_CHAOS_SANDBOX_CONTAINER" ;;
			restart) docker restart "$JUDGIFY_CHAOS_SANDBOX_CONTAINER" ;;
			*) ops_die "unsupported sandbox daemon chaos action" ;;
		esac
		;;
	*)
		ops_die "unsupported chaos domain"
		;;
esac

printf '%s\n' "chaos action applied: $domain $action ${target:-}"
