# Contributing to Judgify

Judgify is security-sensitive infrastructure: it authenticates users, makes
authorization decisions, stores contest state, and executes untrusted code.
Changes should preserve the documented boundaries even during local-only
development.

## Prerequisites

- Go 1.27.1
- Node.js 22.22.0 with npm lockfile installs
- Docker Engine with Compose and Buildx
- Atlas 1.0.0 for schema authoring/verification
- `golangci-lint`, `govulncheck`, and Make for the complete release gate
- a sibling `../go-common` checkout during local multi-module development

## Local setup

```sh
export POSTGRES_USER=judgify_dev
export POSTGRES_PASSWORD='local-development-only'
export POSTGRES_DB=judgify
export DATABASE_USERNAME="$POSTGRES_USER"
export DATABASE_PASSWORD="$POSTGRES_PASSWORD"
export DATABASE_DATABASE="$POSTGRES_DB"
make docker-up
make migrate-apply
make run-api
```

Run `make run-website` in another terminal. `make seed` is optional local
fixture setup and must not be used as an application-startup dependency.

## Architecture rules

- Keep business policy inside its domain; add reusable infrastructure only to
  domain-neutral `go-common` packages.
- PostgreSQL is canonical. Forge/outbox work must be idempotent and replayable;
  an acknowledged submission cannot depend on process memory.
- Schema changes are authored through Ent/Atlas and checked in as SQL. Runtime
  processes never call automatic schema creation.
- Every protected route has a named resource/action policy and is enforced on
  the server. UI capability checks improve UX but never replace authorization.
- Propagate `context.Context` and correlation IDs through HTTP, WebSocket,
  transactions, queue records, workers, and outbound calls. Logs must use
  bounded fields and must not contain secrets, source code, test data, SQL
  arguments, or raw external error bodies.
- Frontend feature code uses the canonical API/realtime clients, strict schema
  decoders, centralized API route builders, and the typed `i18n/` catalog (via
  `text()` or its stable `constants/text.ts` compatibility export). Do not add
  fake fallback data, browser token storage, SSE, or network polling.
- Preserve light/dark/system tokens, keyboard operation, visible focus,
  reduced-motion behavior, responsive layouts, and semantic labels.

See [`docs/BACKEND_GUIDE.md`](docs/BACKEND_GUIDE.md) and
[`docs/FRONTEND_GUIDE.md`](docs/FRONTEND_GUIDE.md) before introducing a new
pattern.

## Development checks

Use narrow tests while implementing, then run the canonical gates before
requesting review:

```sh
make verify-go
make verify-frontend
make verify-contracts
make verify-supply-chain
```

`make verify` combines the ordinary source gates. A pull request is displayed
in four CI groups: `API / Gate`, `Website / Gate`, `Cross-stack / Gate`, and
`PR / Final gate`. The final gate is the branch-protection check and fails when
any required track fails. API checks cover Go static analysis, race tests,
coverage, `govulncheck`, and migration replay. Website checks cover lint,
type checks, unit coverage, `npm audit`, production build headers, and bundle
rules. Cross-stack checks cover contracts, browser E2E, production image
builds, SBOM generation, Grype vulnerability scans, and Gitleaks.

The scheduled/manual integration workflow covers encrypted backup/restore,
load, chaos, and resilience evidence. The release workflow additionally
requires a clean immutable source tree, digest-pinned image inventory, and
unsigned release evidence. Neither workflow publishes or deploys artifacts.
Destructive database, backup/restore, load, chaos, and release-evidence checks
require an isolated test environment and explicit acknowledgement variables;
never point them at production or a shared developer database.

Generated Ent and mock output must match its source. Do not manually repair
generated files; update schemas/interfaces and run the repository generation
targets, then inspect the diff.

## Pull requests

1. Branch from `main` and keep the change focused.
2. Document behavior, migration, compatibility, and operational impact.
3. Add failure/concurrency/security cases in addition to the happy path.
4. Run the relevant canonical gates and record any intentionally deferred
   environment-only evidence.
5. Fill in the pull-request template. Never include credentials, private logs,
   contestant source, hidden tests, or production data in an issue or PR.

Use conventional commit prefixes when the repository owner asks for commits.
Security findings follow [`SECURITY.md`](SECURITY.md), not public issues.
