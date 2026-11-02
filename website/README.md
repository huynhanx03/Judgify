# Judgify web application

This directory contains the public and administrative Judgify product built on
the Next.js App Router. Production output is standalone and is served behind
the same ingress as the API. Browser authentication uses protected cookies;
tokens are never persisted in Web Storage.

## Local development

Use the repository-pinned Node version and install exactly the lockfile:

```sh
npm ci
npm run dev
```

The defaults use same-origin `/api` HTTP routes and derive the WebSocket origin
from the current page. Copy `.env.example` to `.env.local` only when the Go API
does not use the default loopback origin:

```text
JUDGIFY_DEV_API_ORIGIN=http://127.0.0.1:8000
```

`next dev` proxies browser `/api/*` and `/ws` traffic to that loopback-only
origin. Keep `NEXT_PUBLIC_API_URL` and `NEXT_PUBLIC_REALTIME_WS_URL` unset in
normal development so cookies, CSRF, Origin validation, and WebSocket upgrades
exercise the same-origin production contract. Never put credentials or private
service addresses in `NEXT_PUBLIC_*`; Next.js embeds them in browser assets.

## Source boundaries

- `app/` owns routing, layouts, loading/error boundaries, and metadata.
- `modules/` owns product workflows and page-level composition.
- `components/ui/` and `design-system/` own reusable accessible primitives and
  tokens.
- `services/` is the only feature-facing HTTP boundary; it uses `lib/api/` for
  strict transport and response parsing.
- `lib/realtime/` owns the resumable WebSocket client and typed event parsing.
- `constants/api/` owns route builders; `constants/text.ts` owns product copy so
  another language can be added without rewriting components.
- `types/` and the domain schema modules validate IDs, cursors, timestamps,
  enums, and server payloads at the browser boundary.

Do not add page-local fetch wrappers, fake fallback records, token storage,
network polling, literal API paths, or user-facing strings inside components.
See [`../docs/FRONTEND_GUIDE.md`](../docs/FRONTEND_GUIDE.md) for the complete
architecture and UI conventions.

## Verification

During focused development, use the narrow command for the area being changed.
Before release, the repository entry point is authoritative:

```sh
npm run verify
npm run build
npm run check:bundle
npm run test:e2e
cd .. && make verify-frontend
```

`verify` covers architecture rules, API/role contracts, realtime contracts,
lint, and TypeScript. The repository release workflow additionally performs a
clean install, production build, dependency audit, browser journeys, and
standalone artifact checks.

The route, viewport, theme, authenticated-fixture, and retained-evidence
contract is documented in
[`docs/architecture/browser-acceptance.md`](docs/architecture/browser-acceptance.md).
Playwright packages are lockfile-pinned; install the matching Chromium binary
on the verification host only when browser acceptance is ready to run.

## Production

Build and run the standalone image through the root Makefile and production
Compose topology; do not deploy this directory directly to an unrelated
hosting preset:

```sh
cd ..
make docker-build-website VERSION=<immutable-release>
make verify-packaging
```

Ingress owns TLS and routes `/api` plus the WebSocket upgrade to the Go API.
The website container is non-root, read-only, and has no database, queue, or
Docker access.
