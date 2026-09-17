# Frontend architecture inventory

This document describes the zero-debt source gates for `Judgify/website`.
Counts are intentionally discovered at verification time rather than copied
into documentation and allowed to become stale.

## Enforced boundaries

| Rule | Boundary |
|---|---|
| Browser-stored auth credentials | Access and refresh credentials remain opaque HttpOnly cookies; JavaScript stores neither tokens nor profile authority. |
| Backend endpoint outside catalog | Endpoint builders live under `constants/api/`. |
| Direct Fetch outside transport | REST uses the checked `api(path, options)` boundary in `lib/api/client.ts`. |
| Numeric permission bitmask | Authorization consumes server-owned resource/action keys and revisioned snapshots. |
| Raw feature color | Raw color values are limited to `app/globals.css`; features consume semantic tokens. |
| SSE or data polling | Browser realtime uses the single shared WebSocket. The only `setInterval` outside that protocol is the local recovery-contact countdown; it performs no network I/O. |
| Multiple WebSocket owners | `lib/realtime/websocket-client.ts` contains the only browser `new WebSocket(...)` call. |
| Structural emoji icon | Product icons use the shared Lucide system. |
| User-visible literal | Product-authored Vietnamese copy resolves from `i18n/catalog.vi.ts` through the stable catalog entry point. |
| Runtime mock or fixture | Production source has no mock fallback. Browser fixtures identify real seeded records and never intercept API traffic. |
| Route acceptance drift | Every App Router page must appear exactly once in `architecture/browser-matrix.json`. |

`npm run check:architecture:inventory` prints the TypeScript-aware architecture
inventory. `npm run check:routes` independently checks the route matrix,
transport ownership, runtime fixtures, polling, numeric ID coercion, blanket
lint suppression, and unbounded animation patterns. Both gates have no accepted
legacy allow-list.

## Real integration state

- Admin overview, audit, policy catalog, operations, notifications, judge
  controls, contest communication, and observability surfaces call typed APIs.
- Session bootstrap reads the canonical `/api/session/me` projection. No JWT is
  decoded and no zero-filled profile is fabricated in the browser.
- Ranking and material adapters preserve absent backend data rather than
  inventing titles, categories, difficulty IDs, or records.
- Role policy editing consumes the server catalog and expected revision; the UI
  does not reconstruct Casbin internals or generate permission identifiers.
- Realtime has one provider per tab. Anonymous and ticketed connections share
  the same client lifecycle, ordered replay, topic acknowledgement, reconnect,
  and REST resynchronization contract.

## Verification

```sh
npm run check:architecture:inventory
npm run check:routes
npm run verify
npm run build
npm run check:bundle
```

Browser acceptance and its seeded-environment contract are documented in
[`browser-acceptance.md`](browser-acceptance.md).
