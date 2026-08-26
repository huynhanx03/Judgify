# Judgify product design system

This is the canonical product-design contract for every public, authenticated,
and administrator surface. Page-specific files may add constraints, but may not
weaken accessibility, theming, localization, responsive, or performance rules.

## Product character

Judgify is a serious programming-learning and online-judge product: precise,
fast, trustworthy, and quietly distinctive. The interface combines three visual
roles without turning them into separate products:

- Public learning surfaces are content-first and calm.
- Solving and submission surfaces feel like a focused developer workspace.
- Administrator surfaces are compact, auditable, and operationally explicit.

Use blue for navigation, selection, information, and data; amber for the single
primary action or cultivation emphasis; jade for success. Decoration must never
compete with code, problem statements, results, or operational state.

## Source-of-truth rules

- Components consume semantic tokens from `app/globals.css`; feature code must
  not introduce light/dark color pairs or raw brand colors.
- Shared token names and theme preferences live in `design/tokens.ts`.
- Product-authored Vietnamese copy is resolved through `i18n/text.ts`. API-owned
  content, names, identifiers, and user-generated text are not localization copy.
- Use the existing shadcn/Radix primitives and Lucide icons. Extend a shared
  primitive instead of cloning behavior inside a route.
- Every server-backed state comes from a typed service. Do not add mock data,
  browser-generated domain IDs, polling, or SSE.

## Theme contract

Light, dark, and system preferences are equally supported. The system preference
must follow the operating-system change without a reload; an explicit preference
must remain stable across sessions.

| Semantic role | Light intent | Dark intent |
|---|---|---|
| Background | cool neutral canvas | deep navy canvas |
| Surface | white, low elevation | neutral navy, low elevation |
| Raised surface | white | lighter neutral navy |
| Primary | deep blue | readable bright blue |
| Action | accessible amber | warm amber with dark foreground |
| Success | deep jade | bright jade with dark foreground |
| Border | subtle blue-neutral | neutral slate |
| Focus | strong blue ring | light-blue ring |

The semantic tokens own exact values. Status meaning must never be conveyed by
color alone: pair it with an icon, label, shape, or text. Code/editor surfaces
stay dark in both themes when this improves syntax readability, but surrounding
controls still use the active theme.

Avoid transparent glass on dense data screens. Subtle blur is acceptable only
for transient navigation overlays where underlying text cannot reduce contrast.

## Typography and density

- Body and controls: Fira Sans with system fallbacks.
- Code, identifiers, shortcuts, and compact technical headings: Fira Code.
- Long-form editorial display type is exceptional, never used in admin tables.
- Body copy starts at 16px on public and form surfaces. Dense table metadata may
  use 13–14px while retaining adequate line height and contrast.
- Prefer sentence case. Uppercase is limited to short status or category labels.
- Numeric operational values use tabular figures when columns must align.

Use the 4px spacing grid. Default controls are 44px high; compact admin controls
may be 36–40px only when every target remains keyboard accessible and at least
44px of coarse-pointer hit area is provided. Public content width is 68–76ch;
admin content uses the available viewport with deliberate column priority.

## Layout families

### Public and learning

- Persistent header, clear page title, one primary action, and readable content.
- Problem catalog exposes search/filter state, result count, and empty/error state.
- Material and problem statements prioritize hierarchy and reading rhythm over
  decorative cards.
- At 375px, stack controls and keep the primary task visible without horizontal
  document scrolling.

### Solver workspace

- Preserve problem context, editor, run/submit action, and result feedback as the
  dominant hierarchy.
- Desktop may use resizable or balanced panes; mobile switches to explicit tabs
  rather than squeezing two columns.
- Submission progress is pushed over WebSocket. Reconnect state, stale data, and
  terminal state must be distinguishable and announced accessibly.
- Destructive navigation while code is dirty requires an explicit warning.

### Administrator

- Desktop uses stable navigation and a content header with title, context, and
  the primary route action. Mobile uses a labeled sheet/drawer.
- Tables retain filters and pagination in the URL when shareable. Prioritize key
  columns; move secondary fields into a row detail panel on narrow screens.
- Bulk, destructive, permission, publish, rollback, judge, and campaign actions
  show scope and consequence before confirmation.
- Permission-driven absence is not a loading state. Explain read-only/forbidden
  states and never render controls that will certainly fail authorization.

## Components and interaction

### Actions

- One visually primary action per local decision area.
- Icon-only buttons require an accessible name and tooltip; use them only for
  universally recognizable, repeated actions.
- Loading buttons retain their width, expose `aria-busy`, and prevent duplicate
  mutation. Success feedback must not hide a subsequent failure.
- Destructive actions use the destructive semantic token and a confirmation that
  names the affected resource; do not rely on red styling alone.

### Cards and surfaces

- Static cards do not lift, scale, or show a pointer on hover.
- Interactive cards are a single semantic link/button with a visible focus ring;
  nested actions must be separate and must not create invalid nested controls.
- Use borders and surface contrast before shadows. Reserve strong elevation for
  dialogs, popovers, and drag state.

### Forms

- Every field has a persistent visible label. Placeholder text is an example,
  never the only label.
- Place validation next to the field and add a top summary for long forms.
- Preserve user input after recoverable failures. Mark optional fields explicitly
  rather than marking every required field.
- Password, token, recovery, OAuth, and permission forms must never echo secrets
  in error details, query parameters, telemetry, or notifications.

### Tables, feeds, and charts

- Always provide loading, empty, filtered-empty, error, stale/reconnecting, and
  success states where applicable.
- Skeletons reflect the final geometry; do not use layout-shifting generic blocks.
- Headers identify sort state; row actions are reachable by keyboard; pagination
  announces the current result range.
- Charts include a textual summary/table and do not depend on tooltip hover.
- Live feeds append predictably, offer a pause/follow control where movement can
  disrupt reading, and never steal focus.

### Dialogs and notifications

- Dialogs have an accessible title/description, initial focus, trapped focus, an
  Escape path unless unsafe, and focus restoration.
- Use toast for transient confirmation, inline feedback for actionable failures,
  and notification center entries for durable domain events.
- Error copy explains what happened, whether data was saved, and the safest next
  action. It must not leak stack traces or infrastructure details.

## State and realtime behavior

- Mutations use server-issued idempotency where supported and reconcile against
  canonical server responses.
- WebSocket is the browser realtime transport. UI handles connecting, live,
  reconnecting, degraded, unauthorized, and terminal shutdown explicitly.
- Reconnection uses bounded jittered backoff and resynchronizes from an ordered
  cursor/snapshot before applying new events.
- Optimistic updates are limited to reversible, low-risk interactions. Judge,
  authorization, publication, rollback, and campaign state wait for server truth.
- Dates are formatted centrally and expose exact time when relative time is shown.

## Motion

Motion communicates continuity; it is not ornament.

- Micro-interactions: 150ms; normal state transitions: 200ms; overlays: up to 300ms.
- Animate opacity and transform where possible. Never animate layout-sized values
  in frequently updated feeds, tables, or the editor.
- No layout-shifting scale hover effects, infinite decorative motion, parallax, or
  autoplay motion that competes with code/results.
- `prefers-reduced-motion: reduce` disables non-essential motion and smooth scroll;
  state changes must remain understandable without animation.

## Accessibility baseline

- Target WCAG 2.2 AA: 4.5:1 normal text, 3:1 large text and UI boundaries.
- All workflows are complete with keyboard only. Focus order follows visual order,
  and focus is always visible on interactive elements.
- Use semantic landmarks, one page-level heading, correctly nested headings,
  native controls, table headers, and accessible names before ARIA patches.
- Skip navigation reaches the main content. Route changes and asynchronous critical
  states are announced without moving focus unexpectedly.
- Touch targets are 44x44 CSS px where practical. Zoom to 200% and text reflow must
  not hide controls or require two-dimensional page scrolling.
- Do not use emoji as product icons. Decorative icons are hidden from assistive
  technology; meaningful icons include text or an accessible name.

## Responsive checkpoints

Design continuously, then explicitly verify:

| Width | Required outcome |
|---|---|
| 375px | No document overflow; primary flows and dialogs fully usable |
| 768px | Navigation transition and forms/tables remain coherent |
| 1280px | Normal desktop client/admin density |
| 1536px | Line lengths stay bounded; data surfaces use space intentionally |

Account for browser zoom, long Vietnamese copy, long identifiers, empty values,
and 200% text size. Do not hide essential actions at any breakpoint.

## Performance contract

- Prefer Server Components for static/read-heavy shells; isolate Client Components
  around interaction, browser APIs, and realtime state.
- Keep route-specific code route-specific. Lazy-load heavy editor, chart, and rich
  document code, with stable skeleton dimensions.
- Avoid context providers that rerender the whole app for a high-frequency event;
  select or partition realtime state by resource.
- Use cursor pagination/virtualization for genuinely large datasets; never render
  an unbounded event, submission, audit, or notification list.
- Images declare dimensions, use the Next image pipeline when suitable, and avoid
  shipping decorative raster assets above their rendered resolution.
- No timer-based data polling. No client-side mock fallback in production paths.

## Pre-delivery acceptance

- [ ] All authored copy resolves from the Vietnamese catalog.
- [ ] Light, dark, and system themes are correct without hydration flash.
- [ ] Loading, empty, error, forbidden, stale/reconnecting, and success states exist.
- [ ] Keyboard workflow, focus restoration, accessible names, and announcements work.
- [ ] Reduced motion and 200% zoom preserve the complete workflow.
- [ ] 375, 768, 1280, and 1536px layouts have no accidental overflow.
- [ ] Interactive targets have appropriate pointer/focus/disabled/busy behavior.
- [ ] No raw feature colors, emoji icons, hard-coded API routes, mocks, polling, or SSE.
- [ ] Heavy route code is lazy where justified and lists are bounded.
- [ ] Browser acceptance covers critical client and admin journeys with real APIs.
