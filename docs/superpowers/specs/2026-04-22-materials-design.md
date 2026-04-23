# Materials Feature Design Spec

## Overview

Learning materials system cho Judgify. Teacher/Admin tạo bài viết markdown, student đọc. Hỗ trợ public và group-scoped visibility.

## Requirements Summary

- **Access**: Admin + Teacher CRUD, Student read-only
- **Visibility**: Public (mọi user) + Group-scoped (member của group)
- **Content**: Markdown only (text, code, LaTeX, KaTeX). No image upload.
- **Categories**: Fixed, admin CRUD. No icon, no sort order — auto sort by name.
- **Workflow**: Draft → Published. Teacher/Admin tự publish, không cần review.
- **Tracking**: view_count (increment mỗi click), estimated_read_time (BE auto-calculate)

---

## Database Schema

### `material_categories` table

Ent schema: `api/internal/ent/schema/material_category.go`

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| id | int | PK, auto | |
| name | string | unique, not empty, max 100 | Tên category (VD: "DSA", "Graph") |
| description | string | optional, max 500 | Mô tả ngắn |
| created_at | timestamp | auto | TimeMixin |
| updated_at | timestamp | auto | TimeMixin |
| deleted_at | timestamp | nullable | SoftDeleteMixin |

Edges: `materials` (one-to-many → Material)

### `materials` table

Ent schema: `api/internal/ent/schema/material.go`

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| id | int | PK, auto | |
| title | string | not empty, max 255 | Tiêu đề bài viết |
| description | string | optional, max 1000 | Mô tả ngắn |
| content | string | optional, type text | Markdown content |
| difficulty_id | int | FK → difficulties | Liên kết bảng difficulties có sẵn |
| author_id | int | FK → users | Người tạo |
| category_id | int | FK → material_categories | Category |
| status | enum | draft/published, default draft | Workflow state |
| visibility | enum | public/group, default public | Ai được xem |
| group_id | int | nullable, FK → groups | Null = public, set = group-scoped (deferred until groups exist) |
| view_count | int | default 0 | Đếm số lần xem |
| estimated_read_time | int | default 0 | Phút đọc ước tính, BE auto-calculate |
| created_at | timestamp | auto | TimeMixin |
| updated_at | timestamp | auto | TimeMixin |
| deleted_at | timestamp | nullable | SoftDeleteMixin |

Edges:
- `category` (many-to-one → MaterialCategory)
- `author` (many-to-one → User)
- `difficulty` (many-to-one → Difficulty)
- `tags` (many-to-many → Tag, like problem_tags)
- `group` (many-to-one → Group, nullable, deferred)

### Join table: `material_tags`

Edge-based (Ent auto-creates join table). Material ↔ Tag many-to-many.

---

## Backend Architecture

Follows existing domain pattern (problem/contest). Domain: `material`.

### Directory Structure

```
api/internal/material/
├── core/
│   ├── entity/
│   │   ├── material_category.go
│   │   └── material.go
│   ├── dto/
│   │   ├── material_category.go
│   │   └── material.go
│   ├── mapper/
│   │   ├── material_category.go    (entity ↔ DTO)
│   │   └── material.go             (entity ↔ DTO)
│   └── service/
│       ├── material_category.go
│       └── material.go
├── ports/
│   └── material.go                 (repository + service interfaces)
├── adapters/
│   ├── driver/
│   │   ├── db/
│   │   │   ├── mapper/
│   │   │   │   ├── material_category.go  (Ent model → entity)
│   │   │   │   └── material.go           (Ent model → entity)
│   │   │   ├── builder/
│   │   │   │   ├── material_category.go
│   │   │   │   └── material.go
│   │   │   ├── material_category.go      (repository impl)
│   │   │   └── material.go               (repository impl)
│   │   └── http/
│   │       ├── material_category.go      (handler)
│   │       ├── material.go               (handler)
│   │       └── handler.go                (route registration)
├── di/
│   └── di.go
└── constant/
    └── material.go
```

### Ports (Interfaces)

```go
// ports/material.go

type MaterialCategoryRepository interface {
    FindAll(ctx context.Context) ([]*entity.MaterialCategory, error)
    Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.MaterialCategory], error)
    Get(ctx context.Context, id int) (*entity.MaterialCategory, error)
    Create(ctx context.Context, e *entity.MaterialCategory) error
    Update(ctx context.Context, e *entity.MaterialCategory) error
    Delete(ctx context.Context, id int) error
    Exists(ctx context.Context, id int) (bool, error)
}

type MaterialRepository interface {
    Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.Material], error)
    Get(ctx context.Context, id int) (*entity.Material, error)
    Create(ctx context.Context, e *entity.Material) error
    Update(ctx context.Context, e *entity.Material) error
    Delete(ctx context.Context, id int) error
    Exists(ctx context.Context, id int) (bool, error)
    IncrementViewCount(ctx context.Context, id int) error
}

type MaterialCategoryService interface {
    FindAll(ctx context.Context) ([]*dto.MaterialCategoryResponse, error)
    Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*dto.MaterialCategoryResponse], error)
    Get(ctx context.Context, id int) (*dto.MaterialCategoryResponse, error)
    Create(ctx context.Context, req *dto.CreateMaterialCategoryRequest) (*dto.MaterialCategoryResponse, error)
    Update(ctx context.Context, id int, req *dto.UpdateMaterialCategoryRequest) (*dto.MaterialCategoryResponse, error)
    Delete(ctx context.Context, id int) error
}

type MaterialService interface {
    Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*dto.MaterialResponse], error)
    Get(ctx context.Context, id int) (*dto.MaterialResponse, error)
    Create(ctx context.Context, req *dto.CreateMaterialRequest) (*dto.MaterialResponse, error)
    Update(ctx context.Context, id int, req *dto.UpdateMaterialRequest) (*dto.MaterialResponse, error)
    Delete(ctx context.Context, id int) error
}
```

### DTOs

```go
// dto/material_category.go

type MaterialCategoryResponse struct {
    ID           int    `json:"id"`
    Name         string `json:"name"`
    Description  string `json:"description,omitempty"`
    ArticleCount int    `json:"article_count"`
}

type CreateMaterialCategoryRequest struct {
    Name        string `json:"name" validate:"required,min=1,max=100"`
    Description string `json:"description" validate:"omitempty,max=500"`
}

type UpdateMaterialCategoryRequest struct {
    ID          int     `json:"-" uri:"id"`
    Name        *string `json:"name" validate:"omitempty,min=1,max=100"`
    Description *string `json:"description" validate:"omitempty,max=500"`
}

type FindAllMaterialCategoriesRequest struct{}
```

```go
// dto/material.go

type MaterialResponse struct {
    ID                int                    `json:"id"`
    Title             string                 `json:"title"`
    Description       string                 `json:"description"`
    Content           string                 `json:"content,omitempty"`
    Difficulty        *dto.DifficultyResponse `json:"difficulty"`
    Category          *MaterialCategoryResponse `json:"category"`
    Tags              []*dto.TagResponse     `json:"tags"`
    AuthorID          int                    `json:"author_id"`
    Status            string                 `json:"status"`
    Visibility        string                 `json:"visibility"`
    ViewCount         int                    `json:"view_count"`
    EstimatedReadTime int                    `json:"estimated_read_time"` // minutes
    CreatedAt         time.Time              `json:"created_at"`
    UpdatedAt         time.Time              `json:"updated_at"`
}

type CreateMaterialRequest struct {
    Title       string `json:"title" validate:"required,min=1,max=255"`
    Description string `json:"description" validate:"omitempty,max=1000"`
    Content     string `json:"content"`
    DifficultyID int   `json:"difficulty_id" validate:"required"`
    CategoryID  int    `json:"category_id" validate:"required"`
    TagIDs      []int  `json:"tag_ids"`
    Status      string `json:"status" validate:"omitempty,oneof=draft published"`
    Visibility  string `json:"visibility" validate:"omitempty,oneof=public group"`
}

type UpdateMaterialRequest struct {
    ID           int     `json:"-" uri:"id"`
    Title        *string `json:"title" validate:"omitempty,min=1,max=255"`
    Description  *string `json:"description" validate:"omitempty,max=1000"`
    Content      *string `json:"content"`
    DifficultyID *int    `json:"difficulty_id"`
    CategoryID   *int    `json:"category_id"`
    TagIDs       *[]int  `json:"tag_ids"`
    Status       *string `json:"status" validate:"omitempty,oneof=draft published"`
    Visibility   *string `json:"visibility" validate:"omitempty,oneof=public group"`
}

type GetMaterialRequest struct {
    ID int `uri:"id" validate:"required"`
}

type DeleteMaterialRequest struct {
    ID int `uri:"id" validate:"required"`
}
```

### API Endpoints

#### Public Routes

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/material-categories` | FindAllCategories | List all categories (FindAll pattern, no pagination) |
| POST | `/materials/find` | FindMaterials | List articles with filters/pagination |
| GET | `/materials/:id` | GetMaterial | Detail + increment view_count |

#### Protected Routes (Admin only — Categories)

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| POST | `/material-categories` | CreateCategory | |
| PUT | `/material-categories/:id` | UpdateCategory | |
| DELETE | `/material-categories/:id` | DeleteCategory | |

#### Protected Routes (Teacher+ — Articles)

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| POST | `/materials` | CreateMaterial | |
| PUT | `/materials/:id` | UpdateMaterial | |
| DELETE | `/materials/:id` | DeleteMaterial | |

### Business Logic

**estimated_read_time**: Auto-calculated on create/update.
- Formula: `len(content) / 1000` (chars per minute, ~200 words/min average, ~5 chars/word in Vietnamese)
- Minimum 1 minute
- Only calculated if content is non-empty

**view_count**: Atomic increment on every GET `/materials/:id`.
- Uses Ent's `.Add(view_count, 1)` or raw SQL `UPDATE materials SET view_count = view_count + 1 WHERE id = ?`

**Find filters** (via QueryOptions):
- `category_id` — filter by category
- `difficulty_id` — filter by difficulty
- `status` — filter by status (public only sees "published")
- `search` — search in title/description (ILIKE)
- `tag_ids` — filter by tags

---

## Frontend Changes

### Types Update (`types/material.ts`)

```typescript
export interface MaterialCategory {
  id: number;
  name: string;
  description?: string;
  article_count: number;
}

export interface MaterialArticle {
  id: number;
  title: string;
  description: string;
  content?: string;
  difficulty: DifficultyResponse;
  category: MaterialCategory;
  tags: Tag[];
  author_id: number;
  status: "draft" | "published";
  visibility: "public" | "group";
  view_count: number;
  estimated_read_time: number;
  created_at: string;
  updated_at: string;
}
```

### Service Update (`services/material.service.ts`)

Replace mock with real `apiClient` calls:
- `getCategories()` → `GET /material-categories`
- `getArticles(filters)` → `POST /materials/find`
- `getArticleById(id)` → `GET /materials/:id`
- Admin CRUD methods for categories and articles

### Public Pages (minor changes)

- `/materials` — swap mock → API. Add view_count, estimated_read_time display.
- `/materials/[id]` — Add reading time indicator, view count badge.

### Admin Pages (new)

#### `/admin/materials/categories`
- Standard `usePaginatedCRUD` + `DataTableShell` + `AdminDataTable` pattern
- Columns: ID, Name, Description, Article Count, Actions
- Dialog: Create/Edit (name + description)

#### `/admin/materials`
- `usePaginatedCRUD` + `DataTableShell` + `AdminDataTable` pattern
- Columns: ID, Title, Category, Difficulty, Status, Views, Read Time, Actions
- Filters: category, difficulty, status
- Dialog: Create/Edit with markdown editor
  - Title, Description (input)
  - Category (select from FindAll)
  - Difficulty (select from FindAll difficulties)
  - Tags (multi-select from FindAll tags)
  - Status (draft/published toggle)
  - Visibility (public/group select)
  - Content (markdown editor textarea)

### Text Constants

Add `TEXT.MATERIALS` to `constants/text.ts` for all new strings.

---

## Implementation Order

1. **BE Schema** — Ent schemas (material_category, material)
2. **BE Entity + Mapper** — Domain entities + Ent↔entity + entity↔DTO mappers
3. **BE Repository** — Ports + Driven adapters (db)
4. **BE Service** — Business logic (estimated_read_time calc, view_count increment)
5. **BE Handler + Routes** — HTTP handlers + route registration
6. **BE DI** — Wire container + register in global DI
7. **FE Types + Service** — Update types, swap mock to API
8. **FE Public Pages** — Update materials list + detail pages
9. **FE Admin Categories** — CRUD page for categories
10. **FE Admin Materials** — CRUD page + markdown editor for articles

---

## Permission Resources

New entries in `resources` table:
- `material.category.read` — public
- `material.category.create` — admin
- `material.category.update` — admin
- `material.category.delete` — admin
- `material.article.read` — public
- `material.article.create` — teacher+
- `material.article.update` — teacher+ (own) / admin (all)
- `material.article.delete` — teacher+ (own) / admin (all)

## Deferred

- **Group-scoping**: Schema có `visibility` + `group_id` nullable. Khi groups feature được implement, enable group-scoping logic.
- **Image upload**: Không hỗ trợ. Content chỉ markdown text.
- **Bookmarks/Reading progress**: Future enhancement.
