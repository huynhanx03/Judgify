# Single-host production deployment

This directory is an operator template, not a ready-to-run environment. The
example image digests are deliberately invalid, the hostname is illustrative,
and no secret is stored in Git.

## Trust boundaries

Only `ingress` publishes a host port. PostgreSQL is reachable only on the
internal backend network. The API has no Docker client, socket, device, added
capability, or privileged mode. The worker is the only service allowed to talk
to a Docker daemon.

The edge network gives ingress the exact address `172.31.248.2`; the production
application config trusts only that peer when interpreting `X-Forwarded-For`.
If the operator changes the edge subnet or ingress address, update
`server.trusted_proxies` to the same exact address. Never trust a public CIDR or
`0.0.0.0/0`, because forwarded headers are otherwise attacker-controlled.

Docker socket access is equivalent to full control of that daemon. For that
reason, `JUDGIFY_ROOTLESS_DOCKER_SOCKET` must point to a dedicated rootless
Docker daemon owned by the same numeric UID/GID configured for the worker. Do
not use `/var/run/docker.sock`, do not share this daemon with another product,
and do not add the API user to its group. Rootless mode reduces host impact; it
does not make untrusted execution safe by itself.

API and worker bind separate loopback-only probe listeners on container ports
`9090` and `9091`. Compose checks `/ready` from inside each container; the
ports are neither published nor advertised to either network, and ingress
refuses operational API paths. Worker readiness includes the current boot and claims gate, exact
approved capability evidence, the dedicated sandbox daemon, cgroup v2, disk,
and database checks. Before a release, also run a compile/run/cancel smoke job
against every exact capability tuple that will accept live submissions.

## Prepare the host

1. Install current Docker Engine with Compose and Buildx. Configure a dedicated
   rootless daemon/user for Judgify's worker.
2. Create the Forge data directory on a local filesystem and make its owner the
   rootless daemon UID/GID. Do not place the queue on NFS.
3. Copy `env.production.example` to an operator-owned location outside Git.
   Replace all zero digests, public identifiers, paths, UID/GID values, and the
   hostname. `JUDGIFY_RELEASE_MANIFEST_FILE` must be an absolute host path to
   the approved JSON artifact described below.
4. Copy `config.production.example.yaml` outside Git. Replace every
   `judgify.example.com` origin/redirect with the real HTTPS origin.
   Keep both probe listeners on loopback. Production configuration rejects
   symlinks, group/world-writable files, `.env` merging, and direct secret
   environment variables; every secret uses its `_FILE` source.
5. Create independent, high-entropy secret files for the PostgreSQL bootstrap,
   API, worker, and migrator roles; JWT HMAC material; OAuth providers; Resend;
   and TLS. Mount the RSA JWT private/public key pair using the paths shown in
   the example. Every config/secret path in the env file is absolute and
   Compose fails if one is omitted. File-backed Compose secrets are bind
   mounts, so their source ownership and mode are preserved. Put them in an
   operator-only `0700` host directory and make each mounted file readable by
   its service identity. For a secret shared by PostgreSQL initialization and
   an unprivileged application role, a root-owned `0444` file inside that
   non-traversable directory permits only explicitly granted containers to
   read the mounted file. Never make the directory group/world traversable.
6. Pull the exact sandbox digest into the dedicated rootless daemon before the
   worker starts. The image in the operator env file and the image loaded into
   that daemon must be identical.

PostgreSQL role initialization runs only when `postgres_data` is empty. Changing
a password file later does not rotate an existing database role; perform an
explicit `ALTER ROLE`, verify the client with the new secret, and only then
retire the old secret.

## Build and release

Local development and image builds may use the sibling `go-common` working
tree:

```sh
make verify-packaging
make docker-build VERSION=<release>
```

Release images must be pushed, scanned, signed, and then resolved to their
registry manifest digests. Put only `image@sha256:...` references in the
operator env file. Production Compose intentionally has no `build:` section.
CI requires the repository variable `GO_COMMON_REF` to be an immutable commit
SHA so a later branch update cannot silently change a release.

A release is stricter: `api/go.mod` must reference a published immutable
`go-common` version and may not contain a parent-directory filesystem replace.
`make verify-release` requires a measured coverage floor, migration replay,
clean Git state, digest-pinned inventory for all five product images, SBOMs,
vulnerability and secret scans, and an empty durable evidence directory. The
script generates unsigned evidence only; signing and provenance are performed
by the separately authorized release environment.

### Configure CI, integration, and release evidence

The three workflows deliberately have different trust levels:

- `CI` runs on GitHub-hosted runners with a read-only token. It verifies Go,
  frontend, static contracts, a clean migration replay, and production image
  packaging through the same scripts used locally.
- `Integration evidence` is manual. Database replay runs on a disposable hosted
  runner; destructive backup/restore and optional load probes run only on a
  self-hosted runner carrying the `judgify-integration` label and protected by
  the `judgify-integration` environment. It cannot deploy or publish images.
- `Unsigned release evidence` is manual, accepts only a version-tag context,
  runs on a `judgify-release` runner behind the
  `judgify-release-evidence` environment, and uploads unsigned evidence. It has
  no write token and cannot sign, publish, or deploy a release.

Set repository variable `GO_COMMON_REF` to the exact 40-character commit used
by Judgify. Configure these protected environment variables on the isolated
runner rather than accepting operator-entered workflow paths or target URLs:

| Environment | Required variables |
| --- | --- |
| `judgify-integration` | `JUDGIFY_BACKUP_RESTORE_DRIVER`, `JUDGIFY_TEST_SOURCE_PGSERVICE`, `JUDGIFY_TEST_TARGET_PGSERVICE`; and, for resilience runs, `JUDGIFY_LOAD_BASE_URL`, `JUDGIFY_LOAD_METRICS_URL`, `JUDGIFY_RESILIENCE_ASSERTION_DRIVER`, `JUDGIFY_RUN_SOAK` |
| `judgify-release-evidence` | `JUDGIFY_GO_COVERAGE_FLOOR_PERCENT`, `JUDGIFY_RELEASE_IMAGES_FILE` |

The release runner must already provide Docker, Atlas dependencies, `syft`,
`grype`, and `gitleaks`; the integration restore runner also needs PostgreSQL
client tools, `age`, and OpenSSL. The image inventory is an absolute,
operator-owned file outside the checkout and contains all five digest-pinned
images. Environment reviewers approve access to those runner-local assets.
Uploaded logs/SBOMs are supporting evidence, not signatures or provenance.

### Author and approve the judge release manifest

The API and worker fail closed unless they load the same strict, checksum-bound
release artifact. The artifact contains one shared immutable sandbox image
digest, all four code-owned runtime identities, the three checker identities,
and an explicit allow-list of complete runtime/checker capability pairs. It is
not generated by combining two independent lists during worker startup.

Use an operator workspace outside Git. Every path accepted by the command must
be absolute, every output must be new, and the command refuses symlinks,
trailing JSON, unknown fields, placeholder digests, and group/world-writable
inputs.

```sh
RELEASE=judge-release.2026.07.28.1
OPERATOR_DIR=/var/lib/judgify-release-authoring/${RELEASE}
mkdir -p "${OPERATOR_DIR}"

cd api
go run ./cmd/judgify release-manifest template \
  --release-id "${RELEASE}" \
  --output "${OPERATOR_DIR}/draft.json"

go run ./cmd/judgify release-manifest sandbox-env \
  --draft "${OPERATOR_DIR}/draft.json" \
  --output "${OPERATOR_DIR}/sandbox.mk"
cd ..
```

The generated draft pins the runtime artifact contracts and checker
implementation identities owned by the current source. Review it and add only
the intended entries to `capabilities`; for example, selecting `cpp17` with
`exact` is one explicit entry. Do not synthesize every runtime/checker
combination unless every pair has actually passed release review.

Build the sandbox with the generated identities. The Make fragment is public
build metadata, not a secret:

```sh
make docker-build-sandbox \
  VERSION="${RELEASE}" \
  SANDBOX_RELEASE_ENV="${OPERATOR_DIR}/sandbox.mk"
```

Push, scan, sign, and resolve that image to its registry digest. Record the
digest body in `sha256:<64 lowercase hex>` form as `SANDBOX_DIGEST`; the
deployment environment may retain the full `registry/name@sha256:...`
reference for pulling.

Approval metadata is deliberate and mandatory. `APPROVER_ID` identifies the
reviewing administrator, `AUDIT_ID` points to the external immutable review
record, and `APPROVED_AT` is the actual RFC3339 approval time:

```sh
cd api
go run ./cmd/judgify release-manifest approve \
  --draft "${OPERATOR_DIR}/draft.json" \
  --output "${OPERATOR_DIR}/${RELEASE}.json" \
  --sandbox-image-digest "${SANDBOX_DIGEST}" \
  --approved-by "${APPROVER_ID}" \
  --audit-id "${AUDIT_ID}" \
  --approved-at "${APPROVED_AT}"

go run ./cmd/judgify release-manifest verify \
  --manifest "${OPERATOR_DIR}/${RELEASE}.json" \
  --release-id "${RELEASE}" \
  --sandbox-image-digest "${SANDBOX_DIGEST}"
cd ..
```

The approval command derives every capability hash, canonicalizes the
document, computes its approval checksum, and runs the same complete catalog
validation used at process startup. The checksum is an integrity and audit
binding, not a substitute for artifact signing or the external approval
system.

Install the approved, non-secret file at the absolute
`JUDGIFY_RELEASE_MANIFEST_FILE` path with mode `0444` so the distinct
unprivileged API and worker identities can both read the same artifact. Set
`judge.release_id` to the exact `release_id`, keep
`judge.api_authoring_catalog_mode` at `approved_release`, and leave
`judge.release_manifest_path` at `/run/judgify/release-manifest.json`. Compose
mounts the same host file read-only into API and worker. Before startup, pull
the full `JUDGIFY_SANDBOX_IMAGE` reference into the worker's dedicated
rootless daemon and verify its digest equals `SANDBOX_DIGEST`.

Never edit an approved file in place. A capability, image, checker, protocol,
or audit change is a new release ID, new approval record, and new output path.
Retain the prior signed manifest and images for schema-compatible rollback.

## Start and inspect

Run Compose with the operator-owned env file and this topology:

```sh
docker compose \
  --env-file /etc/judgify/env.production \
  --file deploy/compose.production.yml \
  config

docker compose \
  --env-file /etc/judgify/env.production \
  --file deploy/compose.production.yml \
  up -d
```

The migrator is finite. API and worker start only after it exits successfully;
ingress starts only after API and website healthchecks pass. Treat any failed
migration as a stopped release, inspect logs by correlation ID where available,
and fix forward. Do not bypass the dependency gate.

Inspect the private probes from inside their containers; do not publish their
ports just to run an operator check:

```sh
docker compose \
  --env-file /etc/judgify/env.production \
  --file deploy/compose.production.yml \
  exec -T api wget --quiet --output-document=- http://127.0.0.1:9090/ready

docker compose \
  --env-file /etc/judgify/env.production \
  --file deploy/compose.production.yml \
  exec -T worker wget --quiet --output-document=- http://127.0.0.1:9091/ready
```

Use `scripts/backup/postgres.sh` for encrypted, signed, off-host database
backups and regularly restore them into an isolated blank database. Forge may
also be snapshotted for operational recovery, but it is not the system of
record and must never overwrite PostgreSQL state during restore. A backup that
has not passed a restore drill is not a recovery plan.

Run exactly one `worker` replica against one Forge data directory. Forge is a
single-host accelerator, not the system of record: PostgreSQL owns solve
transitions, and worker startup reconciles missing reward projections from
those immutable rows before accepting live queue work. This makes a
PostgreSQL-newer-than-Forge restore recoverable, but it does not replace backing
up useful Forge diagnostics or monitoring disk and corruption failures.
