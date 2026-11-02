# Reusable Scoped Casbin Implementation Plan

**Goal:** Add a reusable scoped Casbin foundation in go-common and migrate Judgify's current single-scope policy data without broadening permissions.

**Architecture:** go-common defines the scoped request/model and canonical Ent schema mixin. Judgify supplies an explicit system scope, migrates persisted rows, and keeps app-specific transactional writes, validation, revisions, audit, outbox, and reconciliation.

**Tech Stack:** Go, Casbin v3, Ent, PostgreSQL, SQL migrations.

**Spec:** `docs/superpowers/specs/2026-10-18-reusable-scoped-casbin-design.md`

## Constraints

- One fixed shared scoped model; role and permission rows remain application-owned data.
- Missing required scope denies authorization.
- Applications own Ent generation and migrations; shared libraries never run DDL.
- Judgify policy, revision, audit, and outbox writes stay in one application transaction.
- No commit.

## Tasks

1. Add validated scope and a versioned shared Casbin RBAC-with-domain model in go-common, preserving legacy model compatibility.
2. Add canonical Casbin Ent schema mixin and use it from Judgify's schema.
3. Migrate Judgify model, grouping/policy rows, role-validation positions, authorization requests, and default system scope.
4. Preserve transactional mutation, revision reconciliation, and cross-instance invalidation behavior; review existing event payloads and add scope/revision invalidation if safe within current event contracts.
5. Run formatting and static compilation checks only if permitted by the active task instructions; report any verification not performed.
