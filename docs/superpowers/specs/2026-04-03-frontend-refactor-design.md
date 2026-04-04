# Frontend Refactor — Design Spec

**Date:** 2026-04-03
**Branch:** feature/website
**Scope:** auth, arena, profile, submission modules

---

## Problem

Five areas with repeated/bloated code:

1. **Loading spinner** — same 4-line `Loader2` pattern copied 22+ times across the codebase
2. **Difficulty colors** — three separate inline definitions (`arena-client.tsx`, `difficulty-badge.tsx`, `stats-panel.tsx`) for the same color mapping
3. **`arena-client.tsx` (412 lines)** — one file doing fetch, filter state, debounce, sort, pagination, and all render
4. **`submission-history.tsx` (269 lines)** — `SubmissionDetail` component + date utils + config constants bundled in one file
5. **`RegisterFlow.tsx` (291 lines)** — two visually distinct sections (personal info card + trait gacha card) inline in one component; logic and markup tangled together

---

## Solution

**Extract shared primitives. Split large files by responsibility. Zero behavior changes.**

---

## 1. `<LoadingSpinner>` Component

**File:** `website/components/loading-spinner.tsx`

Replaces the repeated inline pattern across all pages/components.

```tsx
interface LoadingSpinnerProps {
  className?: string   // override wrapper height/padding if needed
}
```

Default renders:
```tsx
<div className="flex items-center justify-center py-12">
  <Loader2 className="h-8 w-8 animate-spin text-primary" />
</div>
```

Callers that need a different height pass `className="h-[50vh]"` etc.

**Replace in:** all admin pages, arena-client, submission-history, profile page, RegisterFlow trait section.

---

## 2. `DIFFICULTY_STYLES` Shared Constant

**File:** `website/constants/styles.ts`

Single source of truth for difficulty level colors, keyed by level number (1/2/3/4/5).

```ts
export const DIFFICULTY_STYLES: Record<number, {
  text: string      // e.g. "text-emerald-500"
  bg: string        // e.g. "bg-emerald-500/10"
  border: string    // e.g. "border-emerald-500/20"
  stroke: string    // hex for SVG (stats-panel donut)
  active: string    // combined active state for arena filter buttons
  hover: string     // hover border for arena filter buttons
}> = { ... }

export const DIFFICULTY_FALLBACK = { ... }
```

**Replace in:**
- `difficulty-badge.tsx` — remove `LEVEL_STYLES`, use `DIFFICULTY_STYLES[level].text`
- `stats-panel.tsx` — remove `DIFF_COLORS`, use `DIFFICULTY_STYLES[level]`
- `arena-client.tsx` — remove inline `colors` object, derive from loaded difficulties using `DIFFICULTY_STYLES[diff.level]`

---

## 3. `lib/format.ts` Date Utilities

**File:** `website/lib/format.ts`

Move `formatTime` and `formatDateTime` out of `submission-history.tsx` into shared lib.

```ts
export function formatTime(dateStr: string): string  // "DD/MM HH:MM"
export function formatDateTime(dateStr: string): string  // "DD/MM/YYYY HH:MM:SS"
```

**Replace in:** `submission-history.tsx` (remove inline definitions, import from lib).

---

## 4. Arena Client Split

**Files:**
- Create: `website/modules/arena/hooks/use-arena-filters.ts`
- Create: `website/modules/arena/components/arena-filter-bar.tsx`
- Modify: `website/app/(main)/arena/arena-client.tsx`

### `useArenaFilters` hook

Owns all filter/sort/search/page state. Exports state + handlers. No fetch logic.

```ts
interface UseArenaFiltersReturn {
  // state
  searchTerm: string
  debouncedSearch: string
  difficultyFilter: Difficulty | "all"
  tagFilters: number[]
  sortField: SortField
  sortDirection: SortDirection
  currentPage: number
  // handlers
  setSearchTerm: (v: string) => void
  setDifficultyFilter: (v: Difficulty | "all") => void
  toggleTag: (id: number) => void
  removeTag: (id: number) => void
  setSort: (field: SortField) => void
  toggleSortDirection: () => void
  setPage: (p: number) => void
  clearAll: () => void
  hasActiveFilters: boolean
}
```

Debounce (400ms) + page-reset-on-filter-change both live here. Fix the cleanup-on-unmount memory leak (`clearTimeout` in return).

### `ArenaFilterBar` component

Receives filter state + handlers + reference data (tags, difficulties) as props. Renders search input + filter Sheet + active filter badges. Zero fetch logic.

```tsx
interface ArenaFilterBarProps {
  filters: UseArenaFiltersReturn
  tags: Tag[]
  difficulties: DifficultyResponse[]
}
```

### `ArenaClient` after refactor (~80 lines)

```
useArenaFilters() → filter state
useEffect → fetch problems when filters change
render: <ArenaFilterBar /> + <ProblemTable /> + pagination
```

---

## 5. Submission History Split

**Files:**
- Create: `website/modules/problem/submission-detail.tsx`
- Modify: `website/modules/problem/submission-history.tsx`
- Modify: `website/lib/format.ts` (already created above)

### `submission-detail.tsx`

Extract `SubmissionDetail` function component + `STATUS_CONFIG` + `LANG_LABELS` into its own file. `submission-history.tsx` imports and uses it.

`STATUS_CONFIG` and `LANG_LABELS` move with `SubmissionDetail` since they are only used there.

### `submission-history.tsx` after refactor (~80 lines)

Only contains: `SubmissionHistory` component + Sheet wrapper that renders `<SubmissionDetail>`.

---

## 6. RegisterFlow Split

**Files:**
- Create: `website/modules/auth/sections/personal-info-section.tsx`
- Create: `website/modules/auth/sections/trait-selection-section.tsx`
- Modify: `website/modules/auth/RegisterFlow.tsx`

### `PersonalInfoSection`

Receives form state + `updateField` + `showPassword` + `setShowPassword` as props. Renders the "Phàm Trần" Card with all input fields.

```tsx
interface PersonalInfoSectionProps {
  form: { username: string; password: string; first_name: string; last_name: string; gender: number; birthday: string }
  updateField: (field: string, value: string | number) => void
  showPassword: boolean
  onTogglePassword: () => void
}
```

### `TraitSelectionSection`

Receives all trait state + handlers as props. Renders the "Thiên Mệnh" Card.

```tsx
interface TraitSelectionSectionProps {
  rolledRootBone: TraitResponse | null
  rolledTalents: TraitResponse[]
  selectedTalents: number[]
  isRolling: boolean
  onRoll: () => void
  onToggleTalent: (id: number) => void
  onOpenCodex: () => void
}
```

### `RegisterFlow.tsx` after refactor (~90 lines)

State + handlers only. Renders header + `<PersonalInfoSection>` + `<TraitSelectionSection>` + submit button + `<TraitCodexModal>`.

---

## File Checklist

**Create:**
- [ ] `website/components/loading-spinner.tsx`
- [ ] `website/constants/styles.ts`
- [ ] `website/lib/format.ts`
- [ ] `website/modules/arena/hooks/use-arena-filters.ts`
- [ ] `website/modules/arena/components/arena-filter-bar.tsx`
- [ ] `website/modules/problem/submission-detail.tsx`
- [ ] `website/modules/auth/sections/personal-info-section.tsx`
- [ ] `website/modules/auth/sections/trait-selection-section.tsx`

**Modify:**
- [ ] `website/components/loading-spinner.tsx` → used in all pages
- [ ] `website/modules/arena/components/difficulty-badge.tsx`
- [ ] `website/modules/profile/stats-panel.tsx`
- [ ] `website/app/(main)/arena/arena-client.tsx`
- [ ] `website/modules/problem/submission-history.tsx`
- [ ] `website/modules/auth/RegisterFlow.tsx`
- [ ] All admin pages using loading spinner (8 pages)

---

## Out of Scope

- `cultivation-panel.tsx` radar chart SVG — domain-specific, no duplication
- `profile/page.tsx` — layout only, no logic duplication
- `useAsyncData` hook — fetch patterns simple enough, not worth abstracting
- Any behavior changes — pure structural refactor only
