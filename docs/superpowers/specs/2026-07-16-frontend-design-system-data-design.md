# Judgify Frontend Design System and Data Design

**Date:** 2026-07-16  
**Status:** Draft for written review  
**Runtime:** Next.js 16, React 19, TypeScript, Tailwind CSS 4, Base UI/shadcn components, next-themes, existing Framer Motion, native Fetch, and one native WebSocket provider

## 1. Outcome

The Judgify website becomes one coherent production product rather than a set of separately styled pages. It keeps Next.js and the current component stack, replaces runtime mock data with real backend contracts, moves browser authentication to secure cookies, uses at most one shared WebSocket connection per browser tab in anonymous or authenticated mode, and supports light, dark, and system themes across every route.

Launch copy is Vietnamese only. All product-authored UI copy is retrieved from one typed catalog so another language can be added without searching components for literals. Domain content—usernames, problem/material content, contest announcements, and admin-authored profile labels—remains typed backend data and is never hardcoded in components.

The frontend must make the following states first-class on every data surface:

- Loading with layout-preserving skeletons.
- Empty with an appropriate next action.
- Permission denied without leaking resource existence.
- Recoverable network/server error with retry.
- Stale/reconnecting realtime state with last-updated context.
- Success feedback that does not block keyboard or screen-reader flow.

## 2. Judgify Fusion visual direction

“Judgify Fusion” combines the precision of a professional developer tool with restrained cultivation identity:

- The coding workspace is dense, calm, and technical.
- Progression, rank, traits, and rewards add jade/amber accents without turning ordinary controls into game ornaments.
- Public learning and contest pages feel editorial and prestigious.
- Admin pages prioritize scanning, comparison, and safe action over decoration.
- Gradients and glass effects are reserved for hero moments, rank/trait artifacts, and selected status highlights; tables, forms, and working surfaces remain solid and high contrast.

The interface does not imitate fantasy UI at the cost of comprehension. Vietnamese cultivation names can provide personality, while labels, actions, validation, and status language remain immediately understandable.

## 3. Design token architecture

Tokens use three layers:

```text
primitive -> semantic -> component
```

### 3.1 Primitive tokens

- Neutral ink/slate scale for surfaces and text.
- Indigo/blue scale for product identity and primary interaction.
- Jade scale for accepted/success/progression.
- Amber/gold scale for cultivation rarity, warnings, and rating emphasis.
- Red scale for destructive/error states.
- Violet/cyan accents for contest and realtime indicators.
- A 4 px base spacing system with named steps.
- Radius steps for compact controls, cards, dialogs, and feature surfaces.
- Elevation steps that work in both themes without relying on excessive blur.
- Typography uses only current-stack `next/font/google` loading: Inter for all Vietnamese UI/body/admin text, Playfair Display for bounded editorial/material/cultivation display moments, and Geist Mono for source, IDs, timings, memory, and metrics. Russo One may remain only for an optional Latin brand mark and is never used for Vietnamese labels, controls, or body copy.
- The root exposes `--font-ui`, `--font-display`, `--font-mono`, and optional `--font-brand-latin`; feature code consumes those semantic names. The migration removes the undefined `--font-geist-mono` reference and inline Cinzel usage. It adds no font package or per-page font request.
- Typography has a named size/line-height/weight scale; long-form Vietnamese content uses readable measure and never uses the display face for dense tables or forms.
- Z-index uses one closed semantic scale: base/content `0`, raised/sticky `10`, dropdown/popover `20`, overlay `40`, modal/sheet `50`, and toast/critical status `60`. Components use the shared portal/layer primitives and do not invent local arbitrary z-index values.

### 3.2 Semantic tokens

Every component consumes semantic CSS variables:

```text
--background
--surface
--surface-raised
--surface-subtle
--text-primary
--text-secondary
--text-muted
--border
--border-strong
--focus-ring
--primary
--primary-foreground
--success
--warning
--danger
--info
--accepted
--wrong-answer
--time-limit
--memory-limit
--runtime-error
--compile-error
--pending
```

Raw color utilities are not used in feature modules except for documented data visualizations. Verdict and contest status colors always include text/icon shape and never rely on color alone.

### 3.3 Themes

`next-themes` supports:

- `light`
- `dark`
- `system`

The theme choice is persisted in a non-sensitive cookie or local preference and applied before paint to prevent a flash. Server-rendered markup remains hydration-stable. Every page, modal, editor, markdown renderer, code block, chart, toast, skeleton, and empty state is verified in all three modes.

Light mode uses a warm near-white background and crisp cool surfaces. Dark mode uses deep ink surfaces rather than pure black. Both themes meet WCAG contrast requirements; dark mode is not produced by simple color inversion.

### 3.4 Motion

Motion communicates hierarchy and state:

- 120 ms for hover/focus feedback.
- 180 ms for menus, tooltips, and small state transitions.
- 240–280 ms for sheets, dialogs, and page-region transitions.
- Spring motion only for cultivation rewards and bounded celebratory moments.

Entrances use the shared ease-out curve and exits use the shared ease-in curve. Routine UI animation is limited to compositor-safe `opacity` and `transform`; height/width/top/left and layout-measuring animation are forbidden on hot interaction or realtime paths. Skeletons are used only when the expected wait exceeds 300 ms and preserve the final region's geometry; faster local transitions keep stable content or use restrained pending feedback.

Animations never delay data visibility, route navigation, form submission, or judge status. `prefers-reduced-motion` removes transforms, parallax, counters, and celebration while retaining immediate opacity/state changes. Motion primitives are shared and bounded through the existing Framer Motion dependency plus CSS for trivial transitions; feature modules do not ship one-off animation runtimes. No additional animation runtime is required for this release.

## 4. Layout system

### 4.1 Public and authenticated shell

- Responsive top navigation with active route, search entry, theme control, notification indicator, and account menu.
- Desktop content width follows page purpose: readable editorial width for material, wide workspace for problems/contests, and bounded dashboard width for profile/ranking.
- Mobile navigation uses an accessible sheet with the same information architecture.
- Important status such as WebSocket reconnect, contest freeze, or judge delay appears in contextual banners, not transient toasts only.

### 4.2 Problem workspace

Desktop problem detail uses a resizable two-region workspace:

- Left: statement, samples, constraints, and metadata. Editorial/discussion capabilities remain outside the stable launch scope.
- Right: source editor/file input, language/runtime selection, submit action, live judge timeline, and history.

Small screens switch to clearly labeled tabs and a sticky submit/status bar. Source and hidden data are fetched only after authorization. Submission detail distinguishes public summary from owner/admin source and diagnostics.

### 4.3 Admin shell

- Persistent collapsible sidebar grouped by capability, not database table.
- Header contains breadcrumbs, global search/command entry, environment badge, health indicator, and actor menu.
- Tables support server pagination, closed filters/sorts, keyboard navigation, density choice, column visibility, row selection, and sticky actions where useful.
- Destructive or broad changes show impact preview, version conflict state, required reason, and committed audit ID.
- Detail pages are preferred over oversized forms when an entity has history, revisions, policy, or operational state.

## 5. Component standards

The shared component layer extends the current Base UI/shadcn primitives and owns accessibility and styling contracts:

- Buttons: primary, secondary, outline, ghost, destructive, and loading; no click handler on non-interactive elements.
- Form fields: label, description, required marker, inline validation, pending state, and error summary.
- Dialog/sheet/menu/tooltip: focus trap, restore focus, Escape behavior, semantic labels, and correct portal theme.
- Data table: caption, headers, sort state, row actions, empty/error/loading, mobile fallback, and pagination.
- Status badge: stable icon + text + semantic color for verdicts, jobs, contests, accounts, publications, and delivery.
- Timeline: submission states, judge attempts, contest lifecycle, material revisions, and audit history.
- Diff viewer: permission before/after, content revision, configuration, and admin impact.
- Markdown/code: GFM, math, safe links, copy action, line wrapping/scrolling, and theme-aware syntax.
- Toasts: concise confirmation only; errors that need action remain inline or in banners.
- Skeletons reproduce final dimensions and avoid layout shift.

Feature modules do not fork shared primitives merely to change color or spacing. Variants are added through typed component contracts.

Public Problem, Material, Contest authored content, and admin notification-campaign preview consume the shared content-render projection: immutable raw revision ID, `render_source_id`, exact artifact ID, renderer/sanitizer versions, checksum, and server-produced `sanitized_html`. Read components display only `sanitized_html` from that committed revision/source/artifact selection; they never render raw backend Markdown on demand, select an implicit latest artifact, or treat a browser sanitizer as the security boundary. Authoring editors retain Markdown as input, but preview calls the server renderer and is schedule/publish-valid only when the returned source/artifact IDs still match the current draft revision. The frontend contract uses the canonical `content_render_sources` parent relationship and does not model a fake polymorphic `(kind, revision UUID)` foreign key.

## 6. Typed Vietnamese text catalog

All user-visible text lives under one catalog entry point, initially:

```text
website/constants/text.ts
```

The catalog is reorganized by product domain instead of a single flat set:

```ts
export const vi = {
  common: { ... },
  navigation: { ... },
  auth: { ... },
  arena: { ... },
  submission: { ... },
  contest: { ... },
  cultivation: { ... },
  material: { ... },
  notification: { ... },
  admin: { ... },
  errors: { ... },
} as const;

export type TextCatalog = typeof vi;
export const text: TextCatalog = vi;
```

Rules:

- Components import text keys; they do not contain Vietnamese UI literals.
- Variable sentences are formatter functions with typed parameters, not string concatenation.
- Backend sends error/event codes and parameters. Frontend maps known codes to localized copy and uses a safe generic fallback.
- Stored notifications use title/body keys plus typed params.
- Dates, numbers, durations, memory, rating deltas, and plural-like forms use central Vietnamese formatters.
- Alt text and screen-reader-only labels are catalog entries too.
- Backend/domain content is rendered as data; it is not duplicated into the UI catalog or mistaken for component copy.
- Developer-only assertions and logs may remain English and are not presented to users.

This is one-language architecture, not a runtime language switch. A second catalog can later satisfy `TextCatalog` without changing components.

## 7. API contract and identifiers

### 7.1 Canonical wire types

- Every backend UUID is a JSON string and a TypeScript string. No route, hook, DTO, table key, or WebSocket topic accepts a numeric ID.
- Timestamps are RFC 3339 UTC strings at the boundary and are formatted only for display.
- Durations and limits include units in field names such as `cpu_time_limit_ms`, `wall_time_limit_ms`, and `memory_limit_kb`.
- Enums are closed string unions.
- Submission language/runtime choices are generated from the backend's enabled runtime-profile contract; launch labels C++17, Python 3, Java 17, and Go, while requests send the exact runtime key rather than a frontend-invented compiler command.
- Nullable fields are distinct from omitted fields.
- Monetary floating-point semantics are not introduced; ratings, counts, and EXP use integers.

Frontend code may use a branded alias to prevent accidental number/string mixing:

```ts
export type UUID = string & { readonly __uuid: unique symbol };
```

Route params remain strings and are validated before requests. The current numeric contest ID assumption is removed everywhere, including the realtime subscription layer.

### 7.2 Response and error contract

Successful HTTP responses use one stable envelope:

```ts
type ApiSuccess<T> = {
  code: string;
  data: T;
  meta?: { cid: string; pagination?: PaginationMeta };
};
```

Errors use:

```ts
type ApiFailure = {
  code: string;
  params?: Record<string, string | number | boolean>;
  details?: Record<string, unknown>;
  field_errors?: Record<string, Array<{
    code: string;
    params?: Record<string, string | number | boolean>;
  }>>;
  cid: string;
  retryable: boolean;
};
```

HTTP status carries protocol meaning; application and field-error `code` values are stable and are never arbitrary numeric copies of the status. The UI maps codes and typed parameters through the Vietnamese catalog; the backend does not send canonical user-facing UI sentences. `details` is bounded machine data for typed diagnostics and is never rendered as raw copy. Unknown codes render one safe catalog fallback plus the CID. Validation errors bind to fields and also appear in a focusable form summary.

Pagination uses a cursor for growing feeds and large tables. Page pagination remains only for small, stable admin catalogs. Sort/filter fields come from typed endpoint-specific allowlists.

### 7.3 Contract generation and CI

The backend publishes a checked-in OpenAPI contract for HTTP snapshots/commands and a checked-in JSON schema catalog for WebSocket events. TypeScript wire types and endpoint signatures are generated during development and verified in CI; feature modules do not hand-maintain divergent duplicate wire models.

Domain view models may map wire fields for display, but mapping occurs once at the service boundary. No production service returns `any`, unchecked JSON, or fabricated default objects that look like successful backend data.

## 8. Authentication and request architecture

### 8.1 Same-origin secure cookies

Production routing exposes the site under one origin:

```text
/           -> Next.js
/api/*      -> Judgify Go API
/ws         -> Judgify Go WebSocket
```

Access and refresh credentials are HttpOnly, Secure production cookies. JavaScript never reads, decodes, stores, or logs them. `localStorage` is not used for tokens or authoritative profile state.

State-changing requests send `credentials: "include"` and a CSRF header derived from the server-issued non-HttpOnly CSRF token. Refresh rotation is performed only by the backend cookie endpoint. Commands retry only when they carry an idempotency key and the contract permits it.

Anonymous password-login, registration, and OAuth-finalize commands also generate one non-secret issuance idempotency UUID per logical command. A bounded command record containing only purpose, attempt UUID, and receipt-aligned expiry survives reload/unknown response so the exact retry can recover the server issuance receipt; it is removed only after validated success/session revalidation, explicit user reset, or expiry and never contains the OAuth ticket, profile/onboarding payload, username, password, credential, cookie, or CSRF value.

The onboarding trait-offer POST uses the same minimal recovery shape in a separate purpose-scoped record: offer-attempt UUID and server-bounded expiry only. Replaying that UUID under the signed pre-session returns the canonical offer, so the browser stores neither drawn trait payloads nor random state. A deliberate reroll replaces this record only after the newer offer response commits; an unknown outcome keeps/retries the same attempt rather than accidentally consuming another configured offer.

Login/registration recovery also depends on the signed anonymous CSRF cookie's server-generated `pre_session_id`. The bootstrap client reuses an existing valid cookie and never requests/forces rotation merely because the page reloads, a component remounts, or a request is retried. It does not copy that identity into localStorage/BroadcastChannel. Validated session issuance replaces it with session CSRF state; an explicit reset abandons the pending issuance record and asks the server for a new pre-session identity; natural expiry makes the old recovery terminal.

Password login uses the same attempt UUID for the initial POST and every unknown-outcome retry. While the page remains alive it may retry with the in-memory form value; after a reload it asks the user to enter the password again but reuses the unexpired UUID. It never persists or broadcasts the password, and a timeout alone never generates a replacement UUID. The backend re-verifies the password before releasing the exact receipt, so this non-secret coordinator record is not a login credential.

Registration likewise retries the canonical validated username/recovery-email/profile/trait-offer payload and password only from live form memory. Password onboarding labels why a recovery email is required, validates it without account-existence leakage, creates the account/session even while the contact is pending, and then shows a persistent verify/resend state backed by the durable email operation. The cultivation step calls the CSRF-protected trait-offer POST with one logical idempotency UUID, renders the server's one root bone and six distinct talents, shows expiry/remaining offers, and allows exactly three of those talents to be selected. An unknown offer response retries with the same offer-attempt UUID; remount/reload does not silently spend another offer. Responses carry a monotonic pre-session offer sequence, so a late superseded response is ignored and cannot replace the current choice. The read-only codex explains traits/rarities but cannot inject an unoffered selection. After reload the page first revalidates `/api/session/me`; without a delivered session it restores only the non-secret offer/issuance identities while the server remains canonical, asks the user to re-enter sensitive/profile form data, and reuses the unexpired registration issuance UUID only for the exact payload. Password, recovery email, and onboarding profile data are never persisted by the coordinator. An expired/superseded offer or request-fingerprint conflict is shown honestly and cannot be “fixed” by silently generating another key/account branch.

### 8.2 Cross-tab refresh coordinator

All HTTP 401 recovery and WebSocket `4001 authentication_expired` handling call one coordinator module. Within a tab, concurrent callers share one promise. Across tabs, the leader is serialized by the Web Locks API; the fallback uses a compare-and-verify localStorage lease plus BroadcastChannel. Before starting a new rotation, a lock winner checks the current coordinator record and session generation so a refresh completed by another tab is not repeated.

The only persistent refresh-coordination record is:

```ts
type RefreshCoordinatorRecord = {
  attempt_id: UUID;
  lease_owner: string;
  expires_at: number;
};
```

`attempt_id` is a non-secret client-generated refresh idempotency UUID. The record TTL never exceeds the server receipt-recovery TTL. The initial refresh POST and every retry for an unknown outcome use that exact UUID. If the server committed but the response was lost, or the leader tab/process crashes, the next leader reloads the unexpired record and recovers with the same UUID; it must not generate a new one for the still-presented refresh cookie. The record is cleared only after a validated successful refresh/session revalidation or after receipt-window expiry. A terminal result stops retries but does not turn the record into credential storage.

Broadcast messages contain only attempt/generation/outcome timing needed to wake followers. localStorage/BroadcastChannel never contains access/refresh tokens, CSRF values, cookies, user/session projections, request bodies, or provider secrets. After coordinator success, each original request may retry at most once; unsafe commands retry only with their own valid command idempotency key. After failure, callers converge on the signed-out/error state rather than forming another refresh branch.

### 8.3 Session bootstrap

The root server layout obtains the safe session projection from `/api/session/me`, forwarding the incoming cookie server-side. It provides initial user ID, username, safe profile summary, effective navigation permissions, session expiry, and CSRF state. Client authentication context hydrates from this projection and revalidates when:

- Login/logout/refresh completes.
- A session or account WebSocket event arrives.
- The tab becomes active after a long suspension.
- A protected request returns 401/403.

Logout is a backend command that revokes the session and clears cookies. It is not only local state cleanup.

## 9. Data fetching and cache policy

### 9.1 Server-first rendering

Server Components are the default for initial public and authenticated snapshots. Client Components are limited to interaction, browser APIs, editors, animations, and realtime state.

- Public problem/material/contest catalogs may use bounded Next caching with explicit tags and backend revision-aware invalidation.
- Authenticated, permission-sensitive, submission, notification, and admin data uses `private/no-store` semantics.
- Mutations execute through typed service functions and revalidate the exact affected tags/routes.
- Search/filter state that should be shareable lives in URL search params.
- Debounced search cancels superseded requests.
- Infinite/cursor feeds bound retained client pages and do not grow memory indefinitely.

### 9.2 Submission command, processing, and active-result state

The submission workspace does not collapse request delivery, current judge work, and the contestant-visible result into one status. Its normalized state is:

```ts
type SubmissionViewState = {
  command: "idle" | "sending" | "unknown_outcome" | "acknowledged" | "rejected";
  command_idempotency_key?: string;
  processing: {
    generation: number;
    phase: SubmissionPhase;
    outcome: SubmissionGenerationOutcome | null;
    updated_at: string;
  };
  active_judgement: null | {
    judgement_id: UUID;
    generation: number;
    verdict: SubmissionVerdict;
    score?: number;
    finalized_at: string;
  };
  aggregate_version: number;
};
```

`SubmissionPhase`, `SubmissionGenerationOutcome`, and `SubmissionVerdict` are generated closed unions from the versioned contract, not frontend-invented strings. `phase` answers where the latest generation is in the pipeline; `outcome` is null until that generation is terminal and then represents its contestant verdict, `system_error`, or `cancelled` result. `active_judgement` separately remains the currently committed contestant-visible judgement. Top-level `aggregate_version` is the sole ordering/version field for the whole submission view; processing and active-judgement fields never introduce a second independently comparable wire version.

Before `202 Accepted`, a lost response is `unknown_outcome`; the client retries or queries with the same submission idempotency key and request hash and never fabricates a durable ID. After acknowledgement, REST is the canonical snapshot for both processing and active judgement. A WebSocket event is applied only when its `aggregate_version` is exactly the local version plus one; an equal/older version is ignored and a larger gap triggers REST resync. A rejudge can show `processing.phase = running` and later a terminal failed/cancelled outcome while retaining the prior `active_judgement`; that terminal latest-generation outcome never erases a previously valid contestant result. Only a committed active-judgement change replaces it.

Reload restores the snapshot by submission ID. The UI labels processing and active verdict separately, and terminal presentation never infers a verdict from transport disconnect or command state.

### 9.3 No runtime mocks

Every production route reads from a real service contract. Hardcoded arrays, random statistics, fake latency, generated standings, mock admin rows, and fallback “success” objects are removed from runtime modules.

Fixtures may exist only under test-specific paths and cannot be imported by production entry points. When the backend capability is unavailable, the UI shows an honest unavailable/empty state; it never substitutes mock product data.

## 10. One WebSocket provider

The root application shell mounts one provider for public and authenticated routes. It owns at most one `/ws` connection per tab and provides topic subscriptions through a ref-counted registry; changing principal mode reconnects that same provider rather than mounting a second socket.

```text
RealtimeProvider
  connection/session state
  cursor store
  topic -> subscriber set
  aggregate version map
  reconnect/backoff
  resync dispatcher
```

Rules:

- Feature components never call `new WebSocket()` directly.
- With no credential cookie, the provider may upgrade in explicit anonymous mode and can subscribe only to the server's closed public-topic registry, such as standings for a publicly visible contest. If a credential is present but invalid/expired, upgrade failure is handled as authentication failure and is never silently downgraded to anonymous access.
- First subscriber sends `subscribe`; last subscriber sends `unsubscribe`.
- Reconnect uses exponential backoff with jitter and is paused when offline.
- Authentication failure stops reconnect and triggers session revalidation.
- Close code `4001 authentication_expired` runs the cross-tab refresh coordinator once and reconnects only after session revalidation succeeds.
- After login/session issuance, the provider closes any anonymous socket, validates the canonical session, reconnects once with cookies, and restores still-authorized subscriptions from the registry. After committed logout/revocation and cookie clearing, it discards private topic state and may reconnect anonymously only for remaining public subscribers.
- Each topic stores last cursor and aggregate version; duplicate/older events are ignored.
- `resync_required` invokes the registered REST snapshot loader and replaces local state atomically.
- Public contest/standings pages first render the canonical REST snapshot/cursor, then use the same fenced replay subscription as authenticated pages. During disconnect they retain and label the last snapshot as stale. A stable subscription/quota protocol error is mapped through the text catalog, stops automatic resubscribe churn for that topic, and leaves a bounded REST/manual-refresh fallback; it never blanks the page or fabricates live state.
- Latest-state events replace the topic snapshot; notification/verdict events update bounded normalized stores.
- Connection status is available for banners and diagnostics but does not cause every consumer to rerender on ping traffic.
- The old contest SSE hook and submission polling are removed after migration.

## 11. Page capability map

### 11.1 Public product

- **Home:** product value, active/upcoming contests, and curated/recent problem/material lists from focused real query APIs, with a clear practice entry. Personalized recommendations remain post-launch.
- **Arena:** URL-driven search, tag/difficulty/status filters, stable sort, cursor/page navigation, solved/attempted state when authenticated.
- **Problem detail:** statement, samples, constraints, limits, tags, submissions workspace, live status, secure history, owner-visible detail.
- **Contest list/detail:** lifecycle and registration, countdown, problems when visible, announcements, clarifications, live/frozen standings, rating results.
- **Ranking:** rating and EXP leaderboards, rank/level explanation, stable pagination.
- **Materials:** search/filter, article cards, safe article detail, canonical slug, metadata/read time.
- **About:** static product explanation using the same shell/theme/text system.

### 11.2 Authenticated account

- **Login/register/OAuth/password recovery:** complete keyboard flow, server errors, secure callback handling, required recovery-email/verification state for password onboarding, durable idempotent cultivation offer, remaining-offer/expiry state, and server-enforced selection validation.
- **Profile:** personal data, solved/submission statistics, cultivation, rating history, recent activity, edit profile.
- **Submissions:** own cursor-paginated history, filters, attempt/test timeline, source access, rejudge comparison when applicable.
- **Notifications:** inbox, unread filter/count, mark read/all, archive, action navigation, preferences.
- **Settings & security:** profile preferences, theme, active sessions/devices, revoke session, change password, verified/pending recovery contact with resend/replace/remove and recent reauthentication, linked OAuth accounts, account lifecycle.

### 11.3 Admin product

- Overview/health and capacity.
- Users, account state, sessions, roles, policy matrix, audit preview.
- Problems, immutable revisions, samples/testsets/checkers, publish validation.
- Submissions, judge jobs, dead jobs, attempts, rejudge batches.
- Contests, participants, problems, lifecycle, freeze, announcements, clarifications, rating runs.
- Cultivation catalogs, validation, ledger, adjustments, leaderboards.
- Materials categories, drafts, review queue, revision compare, publish/archive/rollback.
- Notification campaign composer, exact-revision preview/dry-run, and durable operation/delivery state.
- Audit and operations with correlation ID search.

Every page consumes the permissions/session projection and the backend still enforces every operation.

The notification campaign surface is launch in-app only and is visible only with `notification.compose` (plus the scoped role/all-active broadcast capability when selected). It uses a revisioned draft with `expected_version`; after any title, body, action, audience, or schedule edit, the Schedule action remains disabled until dry-run succeeds again for that exact revision. Audience controls expose only a configured-bounded explicit/imported user set, one authorized role, or all active accounts—never arbitrary query JSON—and show the server's timestamped count, safe bounded sample, exclusions, and batch estimate without treating them as a delivery guarantee.

Scheduling presents the sanitized content preview, internal action-path preview, audience kind/count, scheduled time, required nonempty reason, and broad-audience recent-reauth/second confirmation in one review step. For role/all-active, the frontend holds the server's short-lived single-use confirmation nonce only in memory and sends it with the exact expected version plus one logical-command idempotency UUID; a network-unknown outcome retries with that same UUID and unchanged request so the server can return the committed operation. An edit, expiry, resolved conflict, explicit abandon, or failed binding discards the nonce/UUID and requires a new dry-run/confirmation; a mere timeout does not manufacture a new schedule command. Acceptance displays a durable operation ID, not a success illusion; progress, created/skipped/failed counts, bounded safe failure summaries, resume, and cancellation come from canonical operation queries. Cancellation explains that committed notifications cannot be recalled. Version conflicts preserve the local draft and show the canonical current revision/diff, and no sample/export reveals private recipient fields. Email and push options do not appear before those backend channels exist.

## 12. Accessibility

The target is WCAG 2.2 AA:

- Full keyboard access and visible focus.
- One logical heading hierarchy and landmark structure per route.
- Skip link to primary content.
- Dialog/menu/sheet focus containment and restoration.
- Labels and descriptions connected to fields; invalid fields announced.
- Tables have captions, scoped headers, sortable state, and a meaningful mobile alternative.
- Countdown, judge progress, standings, and notification updates use restrained live regions; high-frequency updates are batched.
- Verdict, status, rarity, and rank never rely on color alone.
- Touch targets are at least 44×44 CSS pixels for primary mobile actions.
- Zoom to 200%, text spacing overrides, high contrast, and reduced motion remain usable.
- Markdown images require meaningful alt text or explicit decorative treatment.

## 13. Performance budgets

Production targets on representative mid-tier mobile hardware and network:

- LCP at the 75th percentile: at most 2.5 seconds for public entry pages.
- INP at the 75th percentile: at most 200 ms.
- CLS: at most 0.1.
- Initial route JavaScript is kept below 200 KiB compressed for ordinary public pages; the existing source-editor implementation and heavy authoring/markdown/chart/admin-dialog islands are dynamically loaded only where used. Introducing a different editor library is a later explicit dependency decision, not part of this design.
- No page performs a client waterfall for data the server can fetch concurrently.
- Images use explicit dimensions and Next image optimization where appropriate.
- Tables paginate/virtualize when rows exceed the bounded client threshold.
- WebSocket messages update the smallest normalized store slice and are batched per animation frame for bursty standings.
- Fonts and critical CSS do not block content with avoidable external round trips.

Bundle analysis, Web Vitals collection, and route-level performance checks run in release verification.

## 14. Error, offline, and realtime behavior

- Every API error surfaces its correlation ID in an expandable support detail.
- 401 revalidates the session; 403 displays permission guidance; 404 does not disclose private resources; 409 shows version conflict/diff; 429 respects retry timing; 5xx preserves entered form data.
- Offline state disables unsafe commands and retains local editor text in a bounded non-secret draft only when explicitly supported.
- WebSocket disconnect does not erase the last snapshot. UI labels it stale, attempts reconnect, and offers manual refresh.
- Submission creation navigates only after a durable ID; an unknown response outcome is recovered with the same command idempotency key, and processing versus active result are restored independently from REST after reload.
- Optimistic updates are limited to reversible presentation state such as marking a notification read. Permission, scoring, judge, publication, and progression changes wait for committed server responses.

## 15. Migration sequence

1. Lock UUID/string, timestamp, error, pagination, and event contracts; add CI contract checks.
2. Introduce semantic tokens and shared primitives while preserving route behavior.
3. Convert the shell and every route to light/dark/system and centralized typed text.
4. Add same-origin routing, cookie authentication, CSRF, and server session bootstrap; remove token/profile `localStorage` authority.
5. Replace mock-backed services page by page with typed real APIs and complete loading/empty/error/permission states.
6. Add the single WebSocket provider and migrate verdict, standings, notification, and admin telemetry consumers.
7. Remove SSE, polling, numeric IDs, duplicate socket code, and runtime mocks.
8. Complete admin capability pages and responsive/accessibility verification.
9. Enforce performance budgets and visual regression snapshots for both themes.

## 16. Verification and acceptance

The frontend is accepted when:

- Production code contains no numeric domain IDs, browser token storage, runtime mocks, direct feature-level WebSocket construction, or user-facing string literals outside the typed catalog.
- The production backend and frontend contain no SSE route, `EventSource` usage, legacy SSE hook, or fixed-interval submission-status polling; a bounded compatibility window is allowed only before the final switch gate.
- Every route renders correctly in light, dark, and system modes without hydration flash.
- Anonymous, authenticated, admin, revoked-session, and permission-change journeys converge correctly.
- Refresh recovery reuses the same persisted non-secret attempt UUID across a lost response and leader-tab crash/reload within the receipt TTL, serializes concurrent 401/WebSocket-expiry callers, clears no credential into coordination storage, and retries each original request at most once.
- Password-login recovery reuses its unexpired issuance UUID and server-preserved pre-session cookie after a lost response/reload, prompts for and re-verifies the password after reload, recovers one canonical session, and persists/broadcasts neither password nor subject data.
- Registration recovery reuses the exact issuance UUID and canonical request payload, re-proves the password, converges on one onboarding/session transaction, and renders request-fingerprint/offer conflict without persisting the password/recovery-email/profile payload or fabricating success.
- Trait-offer tests cover CSPRNG-backed one-root/six-distinct-talent shape, exact offer retry without extra quota, finite remaining offers, remount/reload unknown-outcome recovery, expiry/supersession, codex non-authority, three-from-six enforcement, atomic offer consumption with password/OAuth onboarding, and removal of the legacy unauthenticated roll GET.
- Password registration/recovery-contact journeys cover required bounded email input, pending verify/resend, verified switch, recent-reauthenticated replace/remove, provider-verified OAuth consent, generic forgot-password outcomes, provider outage/dead-delivery state, and absence of email/token/profile data in browser persistence or support details.
- Anonymous bootstrap tests prove reload/remount/retry preserves a valid pre-session identity through receipt recovery and that only validated success, explicit reset, or expiry changes it.
- REST snapshot plus WebSocket replay survives refresh, reconnect, duplicate events, and expired cursor resync.
- One-socket tests cover anonymous public snapshot/replay, denial of private topics, present-but-invalid credential failure without anonymous downgrade, anonymous-to-authenticated reconnect after login, private-state removal/authenticated-to-anonymous reconnect after logout, and a quota error that stops resubscribe churn while preserving the stale/manual REST fallback.
- Submission tests cover unknown intake outcome recovery, live processing with a prior active judgement/verdict, terminal `SubmissionGenerationOutcome` for failed/cancelled/system-error rejudge while preserving that prior judgement, committed active-judgement replacement, duplicate events, version gaps, reload, and REST resync.
- Admin campaign journeys cover exact-revision dry-run invalidation, expected-version conflict recovery, explicit/role/all-active capability gates, broad-audience reauthentication plus nonce expiry/replay/wrong-binding rejection, commit/lost-response recovery with the same schedule idempotency UUID and operation ID, bounded operation progress, transient-pending versus terminal-failed presentation, crash/resume, partial cancellation semantics, and honest in-app-only delivery state.
- Keyboard-only and screen-reader smoke journeys cover auth, submission, contest registration/standings, notifications, and critical admin commands.
- Mobile, tablet, desktop, 200% zoom, reduced motion, and high-contrast checks pass.
- Type checking, lint, production build, contract checks, component tests, and end-to-end critical journeys pass.
- Agreed Web Vitals and bundle budgets pass on the release build.
