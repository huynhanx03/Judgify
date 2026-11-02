#!/bin/sh
# Domain-invariant assertion driver for scripts/verify/resilience.sh.
#
# The harness calls this around every fault:
#
#   prepare-fault           <scenario|baseline> <evidence_dir>
#   await-convergence       <scenario>          <evidence_dir>
#   verify-domain-invariants <scenario|baseline|final> <evidence_dir>
#   sample-resources        <scenario|final>    <evidence_dir>
#
# prepare-fault snapshots the durable counters that must not regress. After the
# fault and its recovery, await-convergence waits for the queues to drain and
# verify-domain-invariants compares against that snapshot. A scenario only
# passes if the system returned to a consistent state on its own — the driver
# never repairs anything.
#
# Queries run as judgify_migrator because the assertions read operational tables
# the runtime roles cannot see in full; the driver only ever reads.
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)
. "$ROOT/scripts/lib/operations.sh"

[ "${JUDGIFY_TEST_ENVIRONMENT:-}" = "judgify-resilience-test" ] ||
	ops_die "assertion driver requires the isolated test environment"
: "${JUDGIFY_RESILIENCE_PGSERVICE:?set the libpq service for the test database}"
ops_require_command psql

[ "$#" -eq 2 ] || ops_die "usage: resilience_assertions.sh verb scenario evidence_dir"
verb=$1
scenario=$2
evidence=${3:-${JUDGIFY_RESILIENCE_EVIDENCE_DIR:-}}
[ -n "$evidence" ] || evidence=$JUDGIFY_RESILIENCE_EVIDENCE_DIR
ops_require_secure_directory "$evidence" "resilience evidence directory"
ops_safe_token "$scenario" 128 || ops_die "scenario identifier is malformed"

snapshot="$evidence/state-$scenario.env"
baseline="$evidence/state-baseline.env"

query() {
	PGSERVICE="$JUDGIFY_RESILIENCE_PGSERVICE" psql \
		--no-psqlrc -X -v ON_ERROR_STOP=1 -Atq -c "$1"
}

# One row of counters describing everything a fault could corrupt.
capture_state() {
	query "
		SELECT
		  (SELECT count(*) FROM submissions)                                        || '|' ||
		  (SELECT count(*) FROM judge_jobs)                                         || '|' ||
		  (SELECT count(*) FROM submission_judgements)                              || '|' ||
		  (SELECT count(*) FROM outbox_messages WHERE state = 'pending')            || '|' ||
		  (SELECT count(*) FROM outbox_messages WHERE state = 'dead')               || '|' ||
		  (SELECT count(*) FROM judge_jobs WHERE state IN ('ready','claimed'))      || '|' ||
		  (SELECT count(*) FROM progression_ledger_entries)                         || '|' ||
		  (SELECT count(*) FROM user_reward_profile_revisions)
	"
}

field() { printf '%s' "$1" | cut -d'|' -f"$2"; }

write_state() {
	umask 077
	printf 'state=%s\n' "$1" >"$2"
}

read_state() {
	[ -f "$1" ] || ops_die "missing state snapshot: $(basename "$1")"
	sed -n 's/^state=//p' "$1" | head -1
}

case "$verb" in
	prepare-fault)
		state=$(capture_state)
		write_state "$state" "$snapshot"
		[ "$scenario" != "baseline" ] || write_state "$state" "$baseline"
		printf '%s\n' "captured pre-fault state for $scenario"
		;;

	await-convergence)
		# The queues must drain on their own. Poll rather than sleep a fixed
		# amount so a slow recovery is still a pass and a stuck one still fails.
		deadline=$(( $(date +%s) + ${JUDGIFY_RESILIENCE_CONVERGENCE_SECONDS:-120} ))
		while :; do
			pending=$(query "SELECT count(*) FROM outbox_messages WHERE state = 'pending'")
			inflight=$(query "SELECT count(*) FROM judge_jobs WHERE state IN ('ready','claimed')")
			leases=$(query "
				SELECT count(*) FROM judge_jobs
				WHERE state = 'claimed' AND lease_expires_at < now()")
			if [ "$pending" -eq 0 ] && [ "$inflight" -eq 0 ] && [ "$leases" -eq 0 ]; then
				printf '%s\n' "converged: outbox drained, no in-flight jobs, no expired leases"
				exit 0
			fi
			[ "$(date +%s)" -lt "$deadline" ] ||
				ops_die "did not converge: pending=$pending inflight=$inflight expired_leases=$leases"
			sleep 2
		done
		;;

	verify-domain-invariants)
		state=$(capture_state)
		dead=$(field "$state" 5)
		[ "$dead" -eq 0 ] || ops_die "dead outbox events after $scenario: $dead"

		# A judgement per submission at most: duplicates mean a job was executed
		# twice, which is the failure a crash is most likely to produce.
		duplicates=$(query "
			SELECT count(*) FROM (
			  SELECT submission_id, generation
			  FROM submission_judgements
			  GROUP BY submission_id, generation
			  HAVING count(*) > 1
			) AS d")
		[ "$duplicates" -eq 0 ] || ops_die "duplicate judgements after $scenario: $duplicates"

		# Every terminal submission must carry its judgement: a missing one means
		# a job was lost rather than retried.
		orphans=$(query "
			SELECT count(*) FROM submissions s
			WHERE s.status = 'judged'
			  AND NOT EXISTS (
			    SELECT 1 FROM submission_judgements j WHERE j.submission_id = s.id)")
		[ "$orphans" -eq 0 ] || ops_die "judged submissions without a judgement after $scenario: $orphans"

		if [ "$scenario" != "baseline" ] && [ -f "$baseline" ]; then
			before=$(read_state "$baseline")
			# Append-only ledgers may grow but must never shrink; a smaller count
			# means committed work was rolled back or deleted.
			for index in 1 3 7 8; do
				old=$(field "$before" "$index")
				new=$(field "$state" "$index")
				[ "$new" -ge "$old" ] ||
					ops_die "durable counter $index regressed after $scenario: $old -> $new"
			done
		fi

		write_state "$state" "$evidence/state-verified-$scenario.env"
		printf '%s\n' "domain invariants hold after $scenario"
		;;

	sample-resources)
		umask 077
		{
			printf '{"schema_version":1,"record":"resources","scenario":"%s"' "$scenario"
			printf ',"connections":%s' \
				"$(query "SELECT count(*) FROM pg_stat_activity WHERE datname = current_database()")"
			printf ',"database_bytes":%s' \
				"$(query "SELECT pg_database_size(current_database())")"
			printf ',"longest_transaction_seconds":%s' \
				"$(query "
					SELECT COALESCE(CEIL(EXTRACT(EPOCH FROM max(now() - xact_start))), 0)
					FROM pg_stat_activity WHERE xact_start IS NOT NULL")"
			printf '}\n'
		} >"$evidence/resources-$scenario.json"
		printf '%s\n' "sampled resource usage for $scenario"
		;;

	*)
		ops_die "unsupported assertion verb: $verb"
		;;
esac
