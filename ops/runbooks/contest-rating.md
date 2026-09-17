# Contest projection or rating lag

## Symptoms

Product metrics show standing/content/lifecycle projection lag above 60
seconds or a completed contest awaiting rating longer than five minutes.

## Containment

Keep immutable contest submissions and terminal judgements authoritative.
Pause only the failing projection/operation. Do not edit standings, rating
history, or user rating totals directly, and do not rerun a rating under a new
identity merely to bypass an error.

```sh
scripts/deploy/status.sh
docker compose --env-file /etc/judgify/env.production --file deploy/compose.production.yml \
  exec -T api wget --quiet --output-document=- http://127.0.0.1:9090/ready
```

Use the admin operation detail and immutable audit/event history to locate the
stable failure code. Protect participant-question privacy while inspecting
clarification/content events.

## Recovery

Fix or roll back the deterministic projector, then resume/replay through its
fenced operation. Reconciliation must prove the immutable input checksum and
exactly one rating history row per contest/user outcome. Recovery requires lag
below thresholds, standings and rating totals equal their durable history, no
duplicate notifications, and a public contest snapshot smoke. Record operation
ID only in the audit record, plus actor, reason, release, counts, and checksums.
