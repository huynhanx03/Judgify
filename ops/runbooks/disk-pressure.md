# Disk pressure

## Symptoms

`judgify_host_disk_available_bytes` crosses the 5 GiB safety watermark, backup
staging cannot publish, PostgreSQL reports storage errors, or the worker stops
claiming jobs.

## Safe containment

Stop new judge claims and optional bulk operations before the filesystem is
exhausted. Keep submission admission retryable and bounded. Do not remove
PostgreSQL files, WAL, release manifests, audit data, active staging
directories, or unknown files. Never run the chaos disk script outside its
marked test volume.

```sh
df -h /var/lib/postgresql/data /var/lib/judgify /var/lib/judgify/backup-staging
scripts/deploy/status.sh
```

Expected free capacity is above configured admission and backup watermarks.

## Recovery

Expire only policy-owned artifacts using their supported retention command,
move verified backup bundles to the off-host destination, or expand the
filesystem. Re-enable claims only after worker readiness includes a passing
disk check. Recovery requires stable free-space trend, successful encrypted
backup, database checkpoint health, no lost lease/finalization, and a judge
smoke. Record before/after bytes, deleted artifact classes, actor, and evidence.

