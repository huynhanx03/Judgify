#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)
. "$ROOT/scripts/lib/operations.sh"

CHAOS_CATALOG="$ROOT/test/chaos/scenarios.tsv"

verify_catalog() {
	awk -F'|' '
		/^#/ || NF == 0 { next }
		NF != 5 { exit 1 }
		$1 !~ /^[a-z][a-z0-9_]{2,127}$/ { exit 1 }
		{
			known = 0
			if ($2 == "process" && $3 == "kill" && ($4 == "api" || $4 == "judge-worker") && $5 == "restart") known = 1
			if ($2 == "postgres" && $3 == "pause" && $4 == "-" && $5 == "resume") known = 1
			if ($2 == "postgres" && $3 == "restart" && $4 == "-" && $5 == "none") known = 1
			if ($2 == "docker" && $3 == "pause" && $4 == "-" && $5 == "resume") known = 1
			if ($2 == "docker" && $3 == "restart" && $4 == "-" && $5 == "none") known = 1
			if ($2 == "disk" && $3 == "fill" && $4 == "-" && $5 == "clear") known = 1
			if (known == 0) exit 1
		}
		seen[$1]++ { exit 1 }
		{ count++ }
		END { if (count == 0) exit 1 }
	' "$CHAOS_CATALOG" || ops_die "chaos scenario catalog is malformed"
}

verify_assets() {
	for file in \
		test/load/http.js \
		test/load/websocket.js \
		test/load/submissions.js \
		test/load/soak.js \
		test/load/scenario.yaml \
		test/chaos/scenarios.tsv \
		test/chaos/README.md \
		test/chaos/process.sh \
		test/chaos/postgres.sh \
		test/chaos/docker.sh \
		test/chaos/disk.sh; do
		[ -s "$ROOT/$file" ] || ops_die "missing resilience asset: $file"
	done
	verify_catalog
}

invoke_fault() {
	kind=$1
	action=$2
	target=$3
	case "$kind" in
		process) "$ROOT/test/chaos/process.sh" "$action" "$target" ;;
		postgres) "$ROOT/test/chaos/postgres.sh" "$action" ;;
		docker) "$ROOT/test/chaos/docker.sh" "$action" ;;
		disk) "$ROOT/test/chaos/disk.sh" "$action" ;;
		*) return 1 ;;
	esac
}

invoke_recovery() {
	kind=$1
	action=$2
	target=$3
	[ "$action" != "none" ] || return 0
	invoke_fault "$kind" "$action" "$target"
}

run_assertion() {
	timeout --signal=TERM --kill-after=5s \
		"${JUDGIFY_RESILIENCE_STEP_TIMEOUT_SECONDS}s" \
		"$JUDGIFY_RESILIENCE_ASSERTION_DRIVER" "$@"
}

append_manifest_record() {
	id=$1
	kind=$2
	status=$3
	started=$4
	completed=$5
	printf '%s\n' \
		"{\"schema_version\":1,\"record\":\"scenario\",\"scenario_id\":\"$id\",\"fault_kind\":\"$kind\",\"status\":\"$status\",\"started_at\":\"$started\",\"completed_at\":\"$completed\"}" \
		>>"$JUDGIFY_RESILIENCE_MANIFEST"
}

run_scenario() {
	id=$1
	kind=$2
	fault_action=$3
	target=$4
	recovery_action=$5

	run_assertion prepare-fault \
		"$id" "$JUDGIFY_RESILIENCE_EVIDENCE_DIR" || return 1
	if ! invoke_fault "$kind" "$fault_action" "$target"; then
		invoke_recovery "$kind" "$recovery_action" "$target" >/dev/null 2>&1 || true
		return 1
	fi
	if ! invoke_recovery "$kind" "$recovery_action" "$target"; then
		invoke_recovery "$kind" "$recovery_action" "$target" >/dev/null 2>&1 || true
		return 1
	fi
	run_assertion await-convergence \
		"$id" "$JUDGIFY_RESILIENCE_EVIDENCE_DIR" || return 1
	run_assertion verify-domain-invariants \
		"$id" "$JUDGIFY_RESILIENCE_EVIDENCE_DIR" || return 1
	run_assertion sample-resources \
		"$id" "$JUDGIFY_RESILIENCE_EVIDENCE_DIR" || return 1
}

run_websocket_load() {
	ready_file="$JUDGIFY_RESILIENCE_EVIDENCE_DIR/websocket-ready.json"
	JUDGIFY_LOAD_EVIDENCE="$JUDGIFY_RESILIENCE_EVIDENCE_DIR/websocket.json" \
	JUDGIFY_WS_READY_FILE="$ready_file" \
		node "$ROOT/test/load/websocket.js" &
	websocket_pid=$!
	attempt=0
	while [ ! -s "$ready_file" ]; do
		if ! kill -0 "$websocket_pid" 2>/dev/null; then
			wait "$websocket_pid"
			return 1
		fi
		attempt=$((attempt + 1))
		[ "$attempt" -le 90 ] || {
			kill "$websocket_pid" 2>/dev/null || true
			wait "$websocket_pid" 2>/dev/null || true
			return 1
		}
		sleep 1
	done
	run_assertion exercise-websocket-pressure \
		"$JUDGIFY_RESILIENCE_EVIDENCE_DIR" || {
		kill "$websocket_pid" 2>/dev/null || true
		wait "$websocket_pid" 2>/dev/null || true
		return 1
	}
	wait "$websocket_pid" || return 1
	run_assertion verify-websocket-backpressure \
		"$JUDGIFY_RESILIENCE_EVIDENCE_DIR"
}

mode=${1:---check}
case "$mode" in
	--check)
		verify_assets
		printf '%s\n' "resilience assets verified; no load or chaos action was run"
		;;
	--run)
		verify_assets
		[ "${JUDGIFY_LOAD_ACK:-}" = "LOAD_ONLY_JUDGIFY_TEST" ] ||
			ops_die "load execution requires the explicit test acknowledgement"
		[ "${JUDGIFY_CHAOS_ACK:-}" = "CHAOS_ONLY_JUDGIFY_TEST" ] ||
			ops_die "chaos execution requires the explicit test acknowledgement"
		[ "${JUDGIFY_TEST_ENVIRONMENT:-}" = "judgify-resilience-test" ] ||
			ops_die "resilience run requires the isolated test environment"
		ops_require_command node
		ops_require_command timeout
		JUDGIFY_RESILIENCE_STEP_TIMEOUT_SECONDS=${JUDGIFY_RESILIENCE_STEP_TIMEOUT_SECONDS:-300}
		case "$JUDGIFY_RESILIENCE_STEP_TIMEOUT_SECONDS" in
			''|*[!0-9]*) ops_die "resilience step timeout must be an integer" ;;
		esac
		[ "$JUDGIFY_RESILIENCE_STEP_TIMEOUT_SECONDS" -ge 5 ] &&
			[ "$JUDGIFY_RESILIENCE_STEP_TIMEOUT_SECONDS" -le 900 ] ||
			ops_die "resilience step timeout must be between 5 and 900 seconds"
		export JUDGIFY_RESILIENCE_STEP_TIMEOUT_SECONDS
		: "${JUDGIFY_RESILIENCE_EVIDENCE_DIR:?set an empty evidence directory}"
		ops_require_secure_directory \
			"$JUDGIFY_RESILIENCE_EVIDENCE_DIR" \
			"resilience evidence directory"
		[ -z "$(find "$JUDGIFY_RESILIENCE_EVIDENCE_DIR" -mindepth 1 -maxdepth 1 -print -quit)" ] ||
			ops_die "evidence directory must be empty"
		: "${JUDGIFY_RESILIENCE_ASSERTION_DRIVER:?set the domain-invariant assertion driver}"
		ops_require_trusted_executable \
			"$JUDGIFY_RESILIENCE_ASSERTION_DRIVER" \
			"resilience assertion driver"
		: "${JUDGIFY_CHAOS_DRIVER:?set the isolated-host chaos driver}"
		ops_require_trusted_executable "$JUDGIFY_CHAOS_DRIVER" "chaos driver"
		: "${JUDGIFY_CHAOS_MARKER_FILE:?set the isolated environment marker}"
		ops_require_trusted_file "$JUDGIFY_CHAOS_MARKER_FILE" "chaos environment marker"
		: "${JUDGIFY_CHAOS_VOLUME:?set the isolated bounded test volume}"
		ops_require_secure_directory "$JUDGIFY_CHAOS_VOLUME" "chaos volume"
		: "${JUDGIFY_WS_UNREAD_TOPIC:?set the authorized high-volume WebSocket test topic}"

		umask 077
		JUDGIFY_RESILIENCE_MANIFEST="$JUDGIFY_RESILIENCE_EVIDENCE_DIR/manifest.jsonl"
		export JUDGIFY_RESILIENCE_MANIFEST
		catalog_sha=$(ops_sha256 "$CHAOS_CATALOG")
		assertion_driver_sha=$(ops_sha256 "$JUDGIFY_RESILIENCE_ASSERTION_DRIVER")
		chaos_driver_sha=$(ops_sha256 "$JUDGIFY_CHAOS_DRIVER")
		printf '%s\n' \
			"{\"schema_version\":1,\"record\":\"run\",\"catalog_sha256\":\"$catalog_sha\",\"assertion_driver_sha256\":\"$assertion_driver_sha\",\"chaos_driver_sha256\":\"$chaos_driver_sha\",\"started_at\":\"$(date -u '+%Y-%m-%dT%H:%M:%SZ')\"}" \
			>"$JUDGIFY_RESILIENCE_MANIFEST"

		JUDGIFY_LOAD_EVIDENCE="$JUDGIFY_RESILIENCE_EVIDENCE_DIR/http.json" \
			node "$ROOT/test/load/http.js"
		run_websocket_load || ops_die "WebSocket load/backpressure scenario failed"
		JUDGIFY_LOAD_EVIDENCE="$JUDGIFY_RESILIENCE_EVIDENCE_DIR/submissions.json" \
			node "$ROOT/test/load/submissions.js"
		run_assertion verify-domain-invariants \
			baseline "$JUDGIFY_RESILIENCE_EVIDENCE_DIR"

		while IFS='|' read -r id kind fault_action target recovery_action; do
			case "$id" in ''|'#'*) continue ;; esac
			started=$(date -u '+%Y-%m-%dT%H:%M:%SZ')
			if run_scenario "$id" "$kind" "$fault_action" "$target" "$recovery_action"; then
				append_manifest_record "$id" "$kind" passed "$started" \
					"$(date -u '+%Y-%m-%dT%H:%M:%SZ')"
			else
				append_manifest_record "$id" "$kind" failed "$started" \
					"$(date -u '+%Y-%m-%dT%H:%M:%SZ')"
				ops_die "resilience scenario failed after recovery attempt: $id"
			fi
		done <"$CHAOS_CATALOG"

		if [ "${JUDGIFY_RUN_SOAK:-0}" = "1" ]; then
			JUDGIFY_LOAD_EVIDENCE="$JUDGIFY_RESILIENCE_EVIDENCE_DIR/soak.json" \
				node "$ROOT/test/load/soak.js"
		fi
		run_assertion verify-domain-invariants \
			final "$JUDGIFY_RESILIENCE_EVIDENCE_DIR"
		run_assertion sample-resources \
			final "$JUDGIFY_RESILIENCE_EVIDENCE_DIR"
		printf '%s\n' \
			"{\"schema_version\":1,\"record\":\"run_complete\",\"status\":\"passed\",\"completed_at\":\"$(date -u '+%Y-%m-%dT%H:%M:%SZ')\"}" \
			>>"$JUDGIFY_RESILIENCE_MANIFEST"
		printf '%s\n' "resilience evidence retained at $JUDGIFY_RESILIENCE_EVIDENCE_DIR"
		;;
	*)
		ops_die "usage: resilience.sh --check|--run"
		;;
esac
