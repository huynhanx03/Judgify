# CI/CD Track Gates Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make pull request CI visibly separate API, website, cross-stack, and final gates while enforcing the security and production checks appropriate to each layer.

**Architecture:** Keep one PR workflow for now, but name jobs by track and add explicit `api-gate`, `website-gate`, `cross-stack-gate`, and `final-gate` jobs. PRs run source tests, dependency scans, migration replay, packaging, SBOM/CVE checks, and E2E; expensive backup, resilience, and release signing remain in scheduled/manual workflows.

**Tech Stack:** GitHub Actions, Go 1.27.1, Node 22.22.0, npm lockfile, Docker Buildx, Syft, Grype, Gitleaks, Playwright.

**Spec:** CI/CD topology agreed in the conversation on 2026-09-11.

## Global Constraints

- Preserve immutable action references and digest-pinned container images.
- Keep workflow permissions read-only.
- Do not publish or deploy from PR CI.
- Required gates must fail closed when a required job fails.
- Do not weaken current vulnerability thresholds to hide existing findings.

### Task 1: Add topology contract coverage

**Files:**
- Modify: `scripts/verify/workflows_test.sh`

- [ ] Add assertions for named API, website, cross-stack, and final gate jobs and their `needs` relationships.
- [ ] Run `scripts/verify/workflows_test.sh` and confirm the new assertions fail against the current flat workflow.

### Task 2: Refactor PR workflow into track gates

**Files:**
- Modify: `.github/workflows/ci.yml`

- [ ] Rename/group jobs as API, Website, and Cross-stack tracks without changing existing command coverage.
- [ ] Add `api-gate`, `website-gate`, `cross-stack-gate`, and `final-gate` jobs using `if: always()` and explicit result checks.
- [ ] Keep only `final-gate` as the branch-protection aggregate check while retaining readable intermediate checks.
- [ ] Add a PR source security job that runs Syft, Grype, and Gitleaks against source artifacts using pinned tool versions or approved runner tools.
- [ ] Keep image CVE scanning attached to the packaging path after images are built; do not push images.
- [ ] Run workflow contract tests and YAML parsing checks.

### Task 3: Harden release/nightly separation

**Files:**
- Modify: `.github/workflows/integration.yml`
- Modify: `.github/workflows/release-evidence.yml`

- [ ] Preserve manual protected-environment controls for backup/restore and resilience.
- [ ] Add a scheduled full image/source security scan path without granting write permissions or deployment credentials.
- [ ] Keep release evidence unsigned and explicit about signing/provenance remaining an approved release action.

### Task 4: Verify and document operational usage

**Files:**
- Modify: `CONTRIBUTING.md`
- Modify: `README.md`

- [ ] Document the four PR checks and which checks are nightly/release-only.
- [ ] Document local commands for API, website, migration, and security verification.
- [ ] Run workflow contracts, Go verification, frontend verification, and targeted security checks; report existing dependency/E2E blockers separately.

