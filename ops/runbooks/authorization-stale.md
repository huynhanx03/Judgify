# Authorization desired/applied revision mismatch

## Symptoms

API readiness reports `authorization_revision_unhealthy`, or the dashboard
shows desired and applied revisions diverging for more than two minutes.

## Containment

Fail closed for protected routes. Do not bypass Casbin middleware, edit Casbin
tables directly, or reduce the desired revision. Keep the protected
superadministrator invariant intact. Public content may remain available only
when the API's readiness/admission policy explicitly permits it.

```sh
docker compose --env-file /etc/judgify/env.production --file deploy/compose.production.yml \
  exec -T api wget --quiet --output-document=- http://127.0.0.1:9090/ready
scripts/deploy/status.sh
```

Use the admin authorization revision view and immutable audit log to identify
the last policy mutation. Expected state is one monotonically increasing
desired revision and an exactly equal applied revision.

## Recovery

Repair database/runtime connectivity or roll back the code that cannot load
the current catalog. Reload the exact desired policy through the supported
authorization runtime; never invent a new revision solely to clear the alert.
Recovery requires equality across API instances, capability checks for sample
allow/deny paths, and one role-edit audit round trip. Record actor, policy
revision, release, CID, and verification results.
