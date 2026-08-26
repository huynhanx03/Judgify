# Browser acceptance and evidence contract

`architecture/browser-matrix.json` is the canonical inventory for all App
Router pages. It records the audience, journey family, and dynamic seed fixture
for each route; it also fixes the explicit review widths at 375, 768, 1280, and
1536 CSS pixels and the light, dark, and system theme choices. Each audience
suite exercises every route against the full theme-by-viewport Cartesian
product; the focused theme/responsive suite retains interaction-specific zoom
and reduced-motion checks.

## Real-service rule

Browser tests run against a deployed or locally started Judgify API, database,
Forge worker, and WebSocket endpoint. They do not use `page.route`, HAR replay,
MSW, synthetic DTOs, or a mock server. Dynamic route values point at disposable
seeded records. Credentials belong to disposable users and are provided only as
masked environment secrets.

Set `JUDGIFY_E2E_REQUIRE_FULL=1` in release acceptance. In that mode the suite
fails before opening a browser unless every required fixture is present:

```text
JUDGIFY_E2E_USER_USERNAME
JUDGIFY_E2E_USER_PASSWORD
JUDGIFY_E2E_ADMIN_USERNAME
JUDGIFY_E2E_ADMIN_PASSWORD
JUDGIFY_E2E_PUBLIC_PROBLEM_ID
JUDGIFY_E2E_PUBLIC_CONTEST_ID
JUDGIFY_E2E_PUBLIC_MATERIAL_SLUG
JUDGIFY_E2E_PUBLIC_USERNAME
JUDGIFY_E2E_ADMIN_PROBLEM_ID
JUDGIFY_E2E_ADMIN_CONTEST_ID
JUDGIFY_E2E_POLICY_ROLE_KEY
JUDGIFY_E2E_POLICY_RESOURCE
JUDGIFY_E2E_POLICY_ACTION
```

The policy fixture must be a disposable, non-protected role that is not relied
on by the administrator running the suite. The test changes exactly one
catalog-provided permission through the UI and restores its original value in a
`finally` recovery path.

## Runtime profiles

- Default: `http://127.0.0.1:3000`, with Playwright starting `npm run dev`.
- Existing deployment: set `JUDGIFY_E2E_BASE_URL` and
  `JUDGIFY_E2E_MANAGED_WEB_SERVER=0`.
- Production bundle: build first and set
  `JUDGIFY_E2E_SERVER_COMMAND="npm start"`.
- Web Vitals enforcement: set `JUDGIFY_E2E_ENFORCE_PERFORMANCE=1`. The suite
  enforces LCP ≤ 2.5 s, CLS ≤ 0.1, and INP ≤ 200 ms when Event Timing is
  available under the documented host profile.

The npm packages do not include the Chromium executable. Provision the exact
browser version reported by the pinned Playwright package on the local or
self-hosted verification machine before running browser tests. No browser
download occurs during ordinary source, unit, lint, type, or build work.

## Commands

```sh
npm run test:e2e:public
npm run test:e2e:member
npm run test:e2e:admin
npm run test:e2e:performance
npm run test:e2e
```

## Retained evidence

Playwright writes under `test-results/playwright` and retains traces,
screenshots, and video on failure. Each route attaches redacted browser/API
diagnostics and WebSocket connection counts. The performance suite attaches a
per-route Web Vitals JSON record. `npm run check:bundle` writes
`test-results/bundle-budget.json` and fails when an ordinary public route
exceeds 200 KiB of unique initial JavaScript compressed with gzip.

Evidence must never contain passwords, cookies, CSRF values, OAuth/reset
credentials, request bodies, notification recipient data, source code, or
unredacted backend errors. Release automation retains the entire evidence
directory as a protected artifact.
