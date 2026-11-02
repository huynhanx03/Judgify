# Sandbox daemon, image, or cgroup incident

## Symptoms

Worker readiness fails its daemon/cgroup/exact-capability check, golden smoke
fails, sandbox OOM/pids/CPU events rise, or cleanup leaves a container behind.

## Containment

Disable claims for the exact current boot before touching the daemon. Keep API
and PostgreSQL isolated; never grant the API Docker access. Quarantine the
digest rather than retagging it. Do not weaken network, non-root, read-only,
seccomp/AppArmor, cgroup v2, output, PID, file, memory, or CPU limits to make a
submission pass.

```sh
docker compose --env-file /etc/judgify/env.production --file deploy/compose.production.yml \
  exec -T worker wget --quiet --output-document=- http://127.0.0.1:9091/ready
scripts/deploy/status.sh
```

The production worker must use its dedicated rootless daemon and an immutable
approved image digest.

## Recovery

Restart the isolated daemon or roll back to a retained signed image/manifest.
Register a fresh boot claims-off, run every exact tuple probe and golden smoke,
persist evidence, then enable claims. Recovery requires no orphan containers,
passing cgroup accounting, network denial, queue convergence, and one
compile/run/cancel smoke. Record image digest, boot ID, tuple hashes, release,
operator, and external incident reference in the restricted audit record.
