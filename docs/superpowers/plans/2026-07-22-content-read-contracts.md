# Public/Admin Content Read Contracts Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:test-driven-development and execute each task with a RED/GREEN/refactor cycle. Do not commit in this workspace.

**Goal:** Separate public and administrative reads so unpublished or restricted content never leaks, while administrators can manage drafts and the public problem page receives only safe sample test cases.

**Architecture:** Public HTTP handlers call explicit public service/repository methods whose database predicates enforce publication and visibility. Protected `/admin/...` read routes require the resource `read` capability and call unrestricted administrative reads. The frontend exposes distinct public/admin service methods so call sites cannot accidentally cross the boundary.

**Tech Stack:** Go, Gin, Ent, GoMock, Testify, Next.js, React, TypeScript, Node test runner.

## Global Constraints

- Strict TDD: every behavior test is written and observed failing before production code changes.
- Do not commit.
- Do not modify realtime or submission files.
- Defer contest files until the contest-correctness agent reports completion.
- Preserve UUIDs as strings in the frontend.
- Keep gender values canonical: `0 = male`, `1 = female`, `2 = other`.

---

### Task 1: Public problem samples and administrative problem reads

**Files:**
- Modify: `api/internal/problem/core/dto/test_case.go`
- Modify: `api/internal/problem/ports/problem.go`
- Modify: `api/internal/problem/ports/test_case.go`
- Modify: `api/internal/problem/core/service/problem.go`
- Modify: `api/internal/problem/core/service/test_case.go`
- Modify: `api/internal/problem/adapters/driven/db/test_case.go`
- Modify: `api/internal/problem/adapters/driver/http/problem.go`
- Modify: `api/internal/problem/adapters/driver/http/test_case.go`
- Modify: `api/internal/problem/adapters/driver/http/handler.go`
- Test: `api/internal/problem/core/service/problem_test.go`
- Test: `api/internal/problem/core/service/test_case_test.go`
- Test: `api/internal/problem/adapters/driver/http/handler_test.go`
- Generated: `api/internal/problem/mocks/mock_problem.go`
- Generated: `api/internal/problem/mocks/mock_test_case.go`

**Interfaces:**
- `ProblemService.FindAdmin(ctx, opts)` and `ProblemService.GetAdmin(ctx, id)` use repository `Find`/`Get`.
- `TestCaseService.FindPublicSamples(ctx, problemID)` first calls `ProblemRepository.GetPublished`, then `TestCaseRepository.FindVisibleByProblemID`.
- `SampleTestCaseResponse` contains only `input`, `expected_output`, and `order_index`.

- [ ] Write service tests proving admin reads call unrestricted repositories and public samples reject unpublished problems and contain only repository-visible samples.
- [ ] Run focused tests and observe compile/assertion failures caused by missing methods.
- [ ] Implement the minimal DTO, ports, service and Ent predicate:

```go
func (r *TestCaseRepository) FindVisibleByProblemID(ctx context.Context, problemID uuid.UUID) ([]*entity.TestCase, error) {
    return r.findByProblemID(ctx, problemID, testcase.IsHidden(false))
}
```

- [ ] Add route tests proving `/problems/:id/samples` is public while `/admin/problems/find` and `/admin/problems/:id` require `problem:read`.
- [ ] Implement handlers/routes, regenerate problem mocks, and run focused tests green.

### Task 2: Frontend problem contract

**Files:**
- Modify: `website/constants/api/problem.ts`
- Modify: `website/services/problem.service.ts`
- Modify: `website/types/submission.ts`
- Modify: `website/app/(main)/arena/[id]/page.tsx`
- Modify: `website/app/admin/(dashboard)/problems/page.tsx`
- Modify: `website/app/admin/(dashboard)/problems/[id]/edit/page.tsx`
- Test: `website/tests/content-read-contracts.test.mjs`

**Interfaces:**
- `problemService.getPublicSamples(id)` returns `SampleTestCaseResponse[]`.
- `problemService.findAdmin(query)` and `problemService.getAdminById(id)` use protected `/admin/problems` reads.

- [ ] Add a source/behavior contract test asserting public detail never references `getTestCases`, and admin pages use admin methods.
- [ ] Run the test and observe the expected failure.
- [ ] Add centralized endpoints and service methods, switch the three call sites, and run the test green.

### Task 3: Public/material administrative reads and category routes

**Files:**
- Modify: `api/internal/material/ports/material.go`
- Modify: `api/internal/material/core/service/material.go`
- Modify: `api/internal/material/adapters/driven/db/material.go`
- Modify: `api/internal/material/adapters/driver/http/material.go`
- Modify: `api/internal/material/adapters/driver/http/handler.go`
- Test: `api/internal/material/core/service/material_test.go`
- Create: `api/internal/material/adapters/driver/http/handler_test.go`
- Generated: `api/internal/material/mocks/mock_material.go`

**Interfaces:**
- `MaterialRepository.FindPublic/GetPublic` enforce both `status = published` and `visibility = public` in Ent.
- `MaterialService.FindPublic/GetPublic` back public routes; `FindAdmin/GetAdmin` back protected `/admin/materials` routes.
- Protected material-category `POST /material-categories/find` and `GET /material-categories/:id` require `material_category:read`.

- [ ] Write failing service tests proving public and admin calls select different repository methods.
- [ ] Run focused tests and observe missing-method failures.
- [ ] Implement public Ent predicates and service separation; regenerate mocks; run service tests green.
- [ ] Write failing route tests for public/admin material reads and category Find/Get permissions.
- [ ] Implement the route/handler split and run route tests green.

### Task 4: Frontend material contract

**Files:**
- Modify: `website/constants/api/material.ts`
- Modify: `website/services/material.service.ts`
- Modify: `website/app/admin/(dashboard)/materials/page.tsx`
- Modify: `website/app/admin/(dashboard)/materials/categories/page.tsx`
- Test: `website/tests/content-read-contracts.test.mjs`

**Interfaces:**
- Public `find/getById` remain public-only.
- `findAdmin/getAdminById` use `/admin/materials`.
- Category admin methods use protected category Find/Get routes.

- [ ] Extend the failing frontend contract test for material admin/public call-site separation.
- [ ] Observe RED, implement endpoint/service/call-site changes, then observe GREEN.

### Task 5: UUID filters and canonical gender mapping

**Files:**
- Create: `website/constants/gender.ts`
- Modify: `website/modules/auth/sections/personal-info-section.tsx`
- Modify: `website/modules/admin/dialogs/create-user-dialog.tsx`
- Modify: `website/app/admin/(dashboard)/problems/page.tsx`
- Modify: `website/app/admin/(dashboard)/tags/page.tsx`
- Modify: `website/app/admin/(dashboard)/traits/page.tsx`
- Modify: `website/app/admin/(dashboard)/materials/page.tsx`
- Test: `website/tests/frontend-domain-contracts.test.mjs`

**Interfaces:**
- `GENDER_OPTIONS` is the sole label/value source and preserves the backend numeric convention.
- Every UUID filter forwards `event.target.value` without numeric coercion.

- [ ] Write tests that load the gender constant as TypeScript and assert exact values, and scan UUID filter call sites for `Number` coercion.
- [ ] Observe RED, implement the typed constant and string filters, then observe GREEN.

### Task 6: Deferred contest public/admin isolation

**Files:** Determined only after the contest-correctness agent finishes; no files are touched before handoff.

- [ ] Inspect the completed contest state and coordinate ownership.
- [ ] Add failing tests proving public reads exclude `draft` while admin reads include all statuses.
- [ ] Implement only query/read route separation without touching transition, intake, arena, or submission behavior.
- [ ] Run contest focused tests green.

### Task 7: Verification

- [ ] Run focused Go tests for problem and material packages.
- [ ] Run focused tests under `go test -race`.
- [ ] Run `go vet` for changed Go packages.
- [ ] Run frontend contract tests, `npm run verify`, and `npm run build`.
- [ ] Inspect the final diff for generated-only churn and unrelated changes; do not commit.
