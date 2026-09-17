# Frontend architecture boundaries

Cross-cutting product policy has one canonical owner. Feature pages consume the
policy; they do not recreate it.

## Canonical boundaries

| Concern | Canonical location | Feature-code rule |
|---|---|---|
| User-visible copy | `constants/text.ts` | Render catalog values; keep the call shape ready for future locale modules. |
| REST endpoint paths | `constants/api/` | Services consume typed endpoint builders; components never assemble backend paths. |
| HTTP transport | `lib/api/client.ts` | Services call the typed `api(path, options)` boundary with a response schema and cancellation signal for reads. Native Fetch uses same-origin cookie credentials, CSRF, CID, timeout/cancellation, checked envelopes, and one coordinator-owned 401 recovery path. |
| Session authority | `contexts/session-context.tsx`, `/session/me` | HttpOnly access/refresh cookies and the readable signed CSRF cookie are server-owned; JavaScript stores no credential or profile authority. |
| Idempotent commands | `lib/api/idempotency.ts`, `lib/auth/refresh-coordinator.ts` | Persist only bounded purpose/attempt UUID/expiry or the exact refresh coordinator tuple; never persist payload, subject, credential, cookie, CSRF, or session projection. |
| Runtime API origin | `constants/api/base.ts` | Browser calls are same-origin in every environment; local development uses the reverse-proxy contract too. |
| Theme and functional colors | `app/globals.css`, typed presentation catalogs | No raw hex/RGB/HSL/arbitrary-color values in feature modules. |
| Authorization | `types/admin.ts`, `services/role.service.ts` | Consume API-owned string resource/action catalog and atomic role snapshots; no role identity or bitmask rules in UI. |
| Realtime | `constants/api/realtime.ts`, `constants/realtime.ts`, `lib/realtime/websocket-client.ts` | No SSE. Public topics are anonymous, private topics use one ticket per upgrade, events invalidate REST snapshots, and readiness requires a subscription ACK. |
| Entity identity | `lib/api/contracts.ts`, API DTOs, endpoint builders | Backend entities use parsed/branded UUID strings end-to-end; UI indexes are presentation labels only. |
| Formatting | `lib/format.ts` | Dates, durations, memory, and units are not rebuilt inside table cells. |

The product currently ships one Vietnamese catalog. Adding another language is
an implementation change behind the catalog boundary, not a feature rewrite.

## Automated gate

Run:

```sh
npm run check:architecture
```

The guard parses TypeScript/TSX rather than grepping. It covers JSX copy and
accessible attributes, presentation defaults and conditional branches, API
calls and endpoints, credential storage, numeric permission operations, SSE,
raw feature colors, and structural emoji icons.

`architecture/frontend-architecture-baseline.json` is intentionally empty.
There is no accepted legacy debt: a new finding fails immediately and must be
moved to the canonical boundary.

Inspect the full machine-readable inventory with:

```sh
npm run check:architecture:inventory
```

## Verification entry points

- `npm run lint`: architecture guard plus ESLint.
- `npm run typecheck`: TypeScript contract verification.
- `npm run verify`: combined lint, architecture-contract, realtime-contract,
  and typecheck gate.
- `npm run build`: production Next.js compilation and route generation.
