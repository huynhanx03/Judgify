# Frontend test strategy

## Goal

The non-E2E suite protects user-visible behavior and trust boundaries without
duplicating browser acceptance coverage or asserting implementation trivia.
Tests should fail when a user-facing flow, accessibility contract, security
boundary, or durable domain rule regresses.

## Test layers

1. **Pure unit tests** cover parsers, schemas, formatters, idempotency
   identities, and query models. They are deterministic and use table-driven
   invalid-boundary cases where useful.
2. **Hook integration tests** cover asynchronous state machines: loading,
   cancellation, stale-response fencing, retry, pagination, and disabled
   states. Shared hooks are tested once because product features compose them.
3. **Component tests** use Testing Library roles, labels, and user-level
   interactions. They cover accessible controls, validation, submit/recovery
   outcomes, and destructive-action guards.
4. **Source contracts** are reserved for cross-cutting invariants which cannot
   be observed safely in JSDOM: route ownership, transport-only `fetch`,
   permission binding, no browser credentials, and API endpoint separation.
5. **E2E** validates browser-to-backend journeys separately. It is intentionally
   excluded from this suite because it requires a functioning backend stack.

## What not to do

- Do not use snapshots as the primary assertion for product UI.
- Do not assert Tailwind class strings or component internals.
- Do not add a cache/store merely to make tests easier.
- Do not chase 100% coverage: it encourages tests for unreachable defensive
  code and reduces review signal. Coverage gates are a floor, not a definition
  of quality.

## Required assertions for new code

- A new public transport/schema branch needs a unit test for successful and
  rejected input.
- A new async hook needs tests for happy path, failure/retry, and stale or
  cancelled work where relevant.
- A form or dialog needs a role/label-based interaction test for its highest
  risk action and blocked/error state.
- A new feature boundary needs a contract test only when runtime testing cannot
  prove the invariant.

## Local commands

```sh
npm run test
npm run test:coverage
npm run test:contracts
npm run verify
```
