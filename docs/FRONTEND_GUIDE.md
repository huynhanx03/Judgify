# Judgify Frontend — Developer Guide

> **Đọc file này trước khi code bất cứ thứ gì.**
> Mục đích: tránh viết lại code đã có, dùng đúng pattern, tránh duplicate.

---

## Nguyên tắc bắt buộc

1. **Dùng Shadcn/UI** cho tất cả UI primitive (button, input, dialog, sheet, badge, table, ...). Không tự viết lại.
2. **Không hardcode class màu difficulty** — luôn dùng `getDifficultyStyle(level)` từ `constants/styles.ts`.
3. **Không viết fetch trực tiếp** — mọi API call phải qua `apiClient` trong `lib/api-client.ts`.
4. **Không dùng `alert` / `toast` trực tiếp** — dùng `notify` từ `lib/toast.ts`.
5. **Không hardcode string UI** — mọi text hiển thị lấy từ `TEXT` trong `constants/text.ts`.
6. **Ghép class Tailwind** luôn dùng `cn()` từ `lib/utils.ts`.
7. **Loading spinner** dùng `<LoadingSpinner />` từ `components/loading-spinner.tsx` — không copy-paste pattern `Loader2` lại.
8. **Admin CRUD page mới** — theo đúng pattern `usePaginatedCRUD + DataTableShell + AdminDataTable + Dialog`. Xem mục 7.

---

## 1. Routing & Layout

```
/                        → app/page.tsx (landing / redirect)

── (auth) layout ─────────────────────────────────────────────
/login                   → app/(auth)/login/page.tsx
/register                → app/(auth)/register/page.tsx
/forgot-password         → app/(auth)/forgot-password/page.tsx
/reset-password          → app/(auth)/reset-password/page.tsx

── (main) layout — có Header ──────────────────────────────────
/arena                   → app/(main)/arena/page.tsx
/arena/[id]              → app/(main)/arena/[id]/page.tsx
/contest                 → app/(main)/contest/page.tsx
/materials               → app/(main)/materials/page.tsx
/materials/[id]          → app/(main)/materials/[id]/page.tsx
/ranking                 → app/(main)/ranking/page.tsx
/profile                 → app/(main)/profile/page.tsx
/about                   → app/(main)/about/page.tsx

── admin layout — có AdminShell ───────────────────────────────
/admin/login             → app/admin/login/page.tsx
/admin                   → app/admin/(dashboard)/page.tsx
/admin/users             → app/admin/(dashboard)/users/page.tsx
/admin/roles             → app/admin/(dashboard)/roles/page.tsx
/admin/problems          → app/admin/(dashboard)/problems/page.tsx
/admin/problems/create   → app/admin/(dashboard)/problems/create/page.tsx
/admin/problems/[id]/edit → app/admin/(dashboard)/problems/[id]/edit/page.tsx
/admin/tags              → app/admin/(dashboard)/tags/page.tsx
/admin/difficulties      → app/admin/(dashboard)/difficulties/page.tsx
/admin/elements          → app/admin/(dashboard)/elements/page.tsx
/admin/rarities          → app/admin/(dashboard)/rarities/page.tsx
/admin/traits            → app/admin/(dashboard)/traits/page.tsx
/admin/levels            → app/admin/(dashboard)/levels/page.tsx
/admin/ranks             → app/admin/(dashboard)/ranks/page.tsx
```

---

## 2. Shared Components (`components/`)

| Component | Import | Dùng khi |
|---|---|---|
| `<LoadingSpinner />` | `@/components/loading-spinner` | Cần màn loading toàn trang / section. Truyền `className="h-[xxx]"` để đổi chiều cao wrapper. **Không dùng `Loader2` inline nữa.** |
| `<ConfirmDialog />` | `@/components/confirm-dialog` | Hỏi xác nhận trước khi xóa. Nhận `open`, `onOpenChange`, `onConfirm`. |
| `<PaginationControls />` | `@/components/pagination-controls` | Hiển thị phân trang. Nhận `PaginationMeta` từ API. |
| `<ProtectedContent />` | `@/components/protected-content` | Ẩn content nếu chưa login. |
| `<ThemeToggle />` | `@/components/theme-toggle` | Nút toggle dark/light. |

### Shadcn/UI primitives có sẵn (`components/ui/`)

`Avatar` · `Badge` · `Button` · `Card` · `Dialog` · `DropdownMenu` · `Input` · `Label` · `Select` · `Separator` · `Sheet` · `Switch` · `Table` · `Tabs` · `Textarea` · `Tooltip`

> **Lưu ý `Select` trong `Dialog`:** Radix Select render vào `document.body` qua portal → bị Dialog overlay che. Nếu cần dropdown trong Dialog, **dùng native `<select>` HTML** (đã áp dụng trong trait-dialog, tag-dialog).

---

## 3. Màu sắc & Style system

### Difficulty colors — `constants/styles.ts`

**Không bao giờ hardcode màu difficulty.** Luôn dùng:

```ts
import { getDifficultyStyle } from "@/constants/styles";

const style = getDifficultyStyle(difficulty.level);
// style.text    → "text-emerald-500"
// style.bg      → "bg-emerald-500/10"
// style.border  → "border-emerald-500/20"
// style.stroke  → "#10b981" (hex cho SVG)
// style.active  → class đầy đủ cho filter button đang active
// style.hover   → class hover cho filter button inactive
```

| Level | Màu | Ý nghĩa |
|---|---|---|
| 1 | emerald-500 `#10b981` | Easy |
| 2 | amber-500 `#f59e0b` | Medium |
| 3 | rose-500 `#f43f5e` | Hard |
| 4 | purple-500 `#a855f7` | Extra 1 |
| 5 | cyan-500 `#06b6d4` | Extra 2 |
| fallback | slate-400 `#6b7280` | Không xác định |

### CSS tokens (globals.css)

| Token | Giá trị tham khảo (dark) | Dùng cho |
|---|---|---|
| `--primary` | amber-500 | CTA buttons, active states, brand color |
| `--secondary` | violet-500 | Secondary actions |
| `--background` | zinc-950 | Page background |
| `--card` | zinc-900 | Card, popover background |
| `--muted` | zinc-800 | Subtle backgrounds |
| `--destructive` | rose-600 | Delete, danger |
| `--chart-1..5` | amber, violet, emerald, sky, rose | Charts |

### Utility classes tùy chỉnh

```css
.glass           /* bg-background/60 backdrop-blur-xl border border-border/50 */
.glass-card      /* bg-card/60 backdrop-blur-xl border border-border */
.glow-amber      /* box-shadow amber glow nhẹ */
.glow-amber-strong /* box-shadow amber glow mạnh */
.glow-text-amber /* text-shadow amber glow */
.heading-gaming  /* font-heading uppercase tracking-wider */
.font-playfair   /* Playfair Display — dùng cho tiêu đề lớn kiểu fantasy */
```

---

## 4. Utilities & Hooks (`lib/` · `hooks/`)

### `lib/api-client.ts`
API wrapper duy nhất. Tự động đính `Authorization` header, auto-refresh token khi 401.

```ts
import { apiClient, ApiError } from "@/lib/api-client";

// Sử dụng
const data = await apiClient.get<MyType>("/endpoint");
await apiClient.post("/endpoint", body);

// Catch lỗi
try { ... } catch (err) {
  if (err instanceof ApiError) notify.error(err.message);
}
```

### `lib/toast.ts`
```ts
import { notify } from "@/lib/toast";

notify.success("Lưu thành công");
notify.error("Có lỗi xảy ra");
notify.warning("Thiếu thông tin");
notify.info("...");
```

### `lib/format.ts`
```ts
import { formatTime, formatDateTime } from "@/lib/format";

formatTime("2024-01-15T10:30:00Z")      // "15/01 10:30"
formatDateTime("2024-01-15T10:30:00Z")  // "15/01/2024 10:30:00"
```

### `lib/utils.ts`
```ts
import { cn } from "@/lib/utils";

cn("base-class", condition && "conditional-class", "another-class")
```

### `lib/jwt.ts`
```ts
import { decodeJwt, isTokenExpired } from "@/lib/jwt";
```

### `contexts/auth-context.tsx`
```ts
import { useAuth } from "@/contexts/auth-context";

const { user, isAuthenticated, isLoading, login, logout } = useAuth();
```
- `user` — `UserProfile | null` (cached, background revalidated)
- `isAuthenticated` — từ JWT trong localStorage (instant, không wait fetch)

---

## 5. Text Constants (`constants/text.ts`)

**Mọi string hiển thị** đều trong `TEXT`. Không hardcode string tiếng Việt trực tiếp vào component.

| Key | Chứa gì |
|---|---|
| `TEXT.COMMON` | LOADING, ERROR, SAVE, CANCEL, DELETE, EDIT, CREATE, SEARCH, FILTER, NO_DATA, PAGE, PREVIOUS, NEXT |
| `TEXT.NAV` | ARENA, MATERIALS, CONTEST, RANKING, PROFILE, LOGOUT |
| `TEXT.AUTH` | Toàn bộ login, register 2 bước, trait gacha, forgot/reset password |
| `TEXT.ARENA` | Hero, table columns, filter panel, sort, empty state |
| `TEXT.PROBLEM` | Problem detail tabs, submission form, history table, detail sheet |
| `TEXT.PROFILE` | Profile sections, cultivation info, edit/save |
| `TEXT.STATS` | Stats panel: solved, AC rate, difficulty breakdown |
| `TEXT.RANKING` | Tab names, leaderboard titles |
| `TEXT.ADMIN` | Toàn bộ admin — mỗi entity có TITLE, SUBTITLE, CREATE, SEARCH_PLACEHOLDER, COL_*, EMPTY, DIALOG_*, FILTER_*, TOAST_* |
| `TEXT.THEME` | LIGHT, DARK, SYSTEM, TOGGLE |

---

## 6. Services (`services/`)

> **Quy tắc: 1 domain = 1 file.** Mỗi service là 1 object export với short method names.
> Domain nằm trong tên service, không lặp trong method. Vd: `userService.find()`, không `userService.findUsers()`.

```ts
// Pattern chuẩn — mọi service đều theo dạng này
export const xxxService = {
  async find(query?: QueryOptions): Promise<Paginated<Xxx>> { ... },
  async getById(id: number): Promise<Xxx> { ... },
  async create(data: ...): Promise<Xxx> { ... },
  async update(id: number, data: ...): Promise<Xxx> { ... },
  async delete(id: number): Promise<void> { ... },
};
```

| Service | File | Covers |
|---|---|---|
| Auth | `auth.service.ts` | login, register, logout, forgot/reset password |
| User | `user.service.ts` | getProfile, updateProfile, admin find/create/updateRole/delete |
| Role | `role.service.ts` | roles CRUD + permissions CRUD + resources CRUD |
| Problem | `problem.service.ts` | public list/detail + admin CRUD + test cases CRUD |
| Submission | `submission.service.ts` | submit, getMyByProblem, getById |
| Tag | `tag.service.ts` | getAll + admin CRUD |
| Difficulty | `difficulty.service.ts` | getAll + admin CRUD |
| Contest | `contest.service.ts` | public list/detail/register + admin CRUD |
| Material | `material.service.ts` | public list/detail + admin CRUD + categories CRUD |
| Cultivation | `cultivation.service.ts` | traits, gacha, user traits |
| Element | `element.service.ts` | getAll + admin CRUD |
| Level | `level.service.ts` | getAll + admin CRUD |
| Rarity | `rarity.service.ts` | getAll + admin CRUD |
| Rank | `rank.service.ts` | getAll + admin CRUD |
| Ranking | `ranking.service.ts` | topByRating, topByExp |

---

## 7. Admin CRUD Pattern

Mọi trang admin CRUD đều theo đúng 3 lớp sau. **Không phá pattern này.**

### Lớp 1 — `usePaginatedCRUD<T>` hook

```ts
import { usePaginatedCRUD } from "@/modules/admin/hooks/use-paginated-crud";

const service = useMemo(() => ({
  find:   xxxService.find.bind(xxxService),    // bắt buộc
  create: xxxService.create.bind(xxxService),   // nếu có dialog
  update: xxxService.update.bind(xxxService),   // nếu có dialog
  delete: xxxService.delete.bind(xxxService),
}), []);

const crud = usePaginatedCRUD<MyEntity>({ service, searchKey: "name" });
```

Hook expose: `crud.data`, `crud.pagination`, `crud.isLoading`, `crud.isSaving`, `crud.search`, `crud.onSearch`, `crud.onPageChange`, `crud.setFilter(key, value, type)`, `crud.openCreate`, `crud.openEdit(item)`, `crud.closeDialog`, `crud.handleSave(input)`, `crud.confirmDelete(id)`, `crud.cancelDelete`, `crud.handleDelete`, `crud.dialogOpen`, `crud.editing`, `crud.deleteId`.

### Lớp 2 — `DataTableShell` (layout)

```tsx
import { DataTableShell } from "@/modules/admin/data-table-shell";

<DataTableShell
  title={TEXT.ADMIN.XXX.TITLE}
  subtitle={TEXT.ADMIN.XXX.SUBTITLE}
  createLabel={TEXT.ADMIN.XXX.CREATE}
  onCreateClick={crud.openCreate}
  searchValue={crud.search}
  onSearch={crud.onSearch}
  searchPlaceholder={TEXT.ADMIN.XXX.SEARCH_PLACEHOLDER}
  extraFilters={<select .../>}   {/* optional */}
  table={<AdminDataTable ... />}
  dialog={<XxxDialog ... />}     {/* optional */}
  confirmDialog={<ConfirmDialog ... />}
/>
```

### Lớp 3 — `AdminDataTable<T>` (table)

```tsx
import { AdminDataTable, type AdminColumn } from "@/modules/admin/admin-data-table";

const columns: AdminColumn<MyEntity>[] = [
  { key: "id",      label: "ID",   render: (r) => <span>{r.id}</span> },
  { key: "name",    label: "Tên",  render: (r) => <span>{r.name}</span> },
  { key: "actions", label: "",     render: (r) => <div>...</div> },
];

<AdminDataTable
  columns={columns}
  data={crud.data}
  keyExtractor={(r) => r.id}
  emptyMessage={TEXT.ADMIN.XXX.EMPTY}
  pagination={crud.pagination}
  onPageChange={crud.onPageChange}
/>
```

### Dialog pattern

- Dialog nhận `rarities` / `elements` / ... làm **props** (từ parent đã fetch sẵn) — không tự fetch lại.
- Dùng native `<select>` HTML (không dùng Shadcn `Select`) nếu dialog đặt trong `Dialog` để tránh z-index conflict.
- File dialog để trong `modules/admin/dialogs/`.

---

## 8. Modules inventory

### Arena (`modules/arena/`)

| File | Vai trò |
|---|---|
| `hooks/use-arena-filters.ts` | Toàn bộ filter/sort/search/pagination state. Dùng khi cần filter list. |
| `components/arena-filter-bar.tsx` | UI filter bar (search + Sheet + active badges). Nhận `filters` từ hook + `tags` + `difficulties`. |
| `components/difficulty-badge.tsx` | Badge hiển thị độ khó. Nhận `difficulty?: DifficultyResponse`. |
| `components/problem-table.tsx` | Bảng problem list với link, difficulty, tags, tỉ lệ AC. |
| `problem-description-panel.tsx` | Panel hiện title, badges, markdown, test cases trong trang problem detail. |

### Contest (`modules/contest/`)

| File | Vai trò |
|---|---|
| `contest-hero-section.tsx` | Hero section trang contest list. |
| `contest-card.tsx` | Card hiển thị 1 contest. |
| `contest-standings-table.tsx` | Bảng xếp hạng ICPC realtime (SSE). Nhận `standings[]` + `isConnected`. |
| `contest-rating-table.tsx` | Bảng rating changes (old/new/delta). Nhận `ratingChanges[]`. |

### Auth (`modules/auth/`)

| File | Vai trò |
|---|---|
| `RegisterFlow.tsx` | Orchestrator đăng ký: giữ state + handlers, render 2 section bên dưới. |
| `sections/personal-info-section.tsx` | Form thông tin cá nhân (username, password, tên, giới tính, ngày sinh). Nhận `form` + `updateField` từ parent. |
| `sections/trait-selection-section.tsx` | Gacha chọn thiên phú (root bone + talents). Nhận toàn bộ trait state + handlers từ parent. |
| `LoginForm.tsx` | Form đăng nhập. |
| `AuthContainer.tsx` | Wrapper glassmorphism cho trang auth. |

### Problem (`modules/problem/`)

| File | Vai trò |
|---|---|
| `submission-history.tsx` | Bảng lịch sử submit. Nhận `submissions[]` + `isLoading`. |
| `submission-detail.tsx` | Panel chi tiết 1 submission (sheet). Export `STATUS_CONFIG`, `LANG_LABELS`. |
| `file-submission.tsx` | Form chọn ngôn ngữ + file + submit. |
| `test-case-block.tsx` | Hiển thị test case mẫu có copy. |
| `markdown-renderer.tsx` (`modules/shared/`) | Render markdown + KaTeX cho đề bài. |

### Profile (`modules/profile/`)

| File | Vai trò |
|---|---|
| `stats-panel.tsx` | Donut chart + bảng solved theo độ khó. Dùng `getDifficultyStyle`. |
| `cultivation-panel.tsx` | Thanh progress realm/rank + element EXP + traits hiển thị. |
| `tags-panel.tsx` | Grid tag đã giải nhóm theo element. |

### Cultivation (`modules/cultivation/`)

| File | Vai trò |
|---|---|
| `trait-card.tsx` | Card hiển thị trait với animation. |
| `trait-codex-modal.tsx` | Modal xem tất cả traits. |

---

## 9. Types

| File | Nội dung chính |
|---|---|
| `types/api.ts` | `ApiResponse<T>`, `PaginationMeta`, `Paginated<T>`, `SearchFilter`, `QueryOptions` |
| `types/auth.ts` | `LoginRequest`, `RegisterRequest`, `LoginResponse` |
| `types/user.ts` | `UserProfile`, `CultivationInfo`, `ProblemStats`, `DiffStat`, `TagStat` |
| `types/problem.ts` | `Problem` |
| `types/submission.ts` | `Submission`, `SubmissionStatus` (union), `Language` (union) |
| `types/difficulty.ts` | `DifficultyResponse`, `DIFFICULTY_SLUG` map |
| `types/tag.ts` | `Tag`, `TagElement` |
| `types/cultivation.ts` | `TraitResponse`, `GachaRollResponse`, `RARITY_DISPLAY`, `ELEMENT_DISPLAY`, `getTierColors()` |
| `types/admin.ts` | `Role`, `Resource`, `Permission`, `AdminUser` |

---

## 10. Checklist khi thêm tính năng mới

- [ ] String mới → thêm vào `constants/text.ts` trước
- [ ] Màu difficulty → dùng `getDifficultyStyle`, không hardcode
- [ ] Loading state → dùng `<LoadingSpinner />`, không copy `Loader2`
- [ ] API call mới → thêm vào đúng service file (1 domain = 1 file), gọi qua `apiClient`
- [ ] Toast → dùng `notify`, không dùng sonner trực tiếp
- [ ] Admin entity mới → theo đúng pattern `usePaginatedCRUD + DataTableShell + AdminDataTable + Dialog`
- [ ] Dialog có dropdown → dùng native `<select>` nếu nằm trong Shadcn `Dialog`
- [ ] Class ghép → dùng `cn()`
- [ ] Fetch reference data (rarities, elements, ...) → fetch 1 lần ở parent, truyền xuống dialog qua props
- [ ] Service mới → 1 domain = 1 file, object export pattern, short method names (`find`, `getById`, `create`, `update`, `delete`)
- [ ] API constant mới → thêm vào đúng file trong `constants/api/`, không tạo file mới nếu domain đã có
- [ ] Dialog tách riêng → để trong `modules/admin/dialogs/`, nhận `open`/`onSave`/`onClose`/`isSaving` props
- [ ] Page quá 200 dòng → tách dialog/panel ra file riêng trong `modules/`
