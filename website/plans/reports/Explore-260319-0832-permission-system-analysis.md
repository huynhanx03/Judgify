# Permission System Exploration Report
**Date:** 2026-03-19  
**Scope:** /Users/lap14687/personal/github.com/huynhanx03/Judgify/api  
**Thoroughness:** Very thorough

---

## Executive Summary

The permission system uses **RBAC (Role-Based Access Control)** with three core entities:
- **Resources**: Protected API resources (problem, user, role, permission, test_case, etc.)
- **Roles**: Users assigned to roles; roles organized in hierarchy via Nested Set Model
- **Permissions**: Links roles to resources with scope bitmasks (CREATE, READ, UPDATE, DELETE)

Permission checking is done at the **HTTP middleware level** with cached role permissions and a resource ID mapper.

---

## Core Data Model

### 1. Resource Entity
**File:** `/Users/lap14687/personal/github.com/huynhanx03/Judgify/api/internal/ent/schema/resource.go`

```
Resource
├── id (PK)
├── key (unique) - string identifier (e.g., "problem", "user", "role")
├── description (nullable)
├── created_at
└── updated_at
```

**All Resource Keys** (`/Users/lap14687/personal/github.com/huynhanx03/Judgify/api/pkg/permissions/constants.go`):
- `generation`
- `user`
- `role`
- `permission`
- `resource`
- `attribute_definition`
- `billing`
- `payment`
- `invoices`
- `plans`
- `subscriptions`
- `problem`
- `test_case`
- `tag`
- `element`
- `trait`
- `user_trait`
- `user_element_exp`
- `level`
- `rank`
- `user_stats`

### 2. Role Entity
**File:** `/Users/lap14687/personal/github.com/huynhanx03/Judgify/api/internal/ent/schema/role.go`

```
Role
├── id (PK)
├── name (unique, not empty)
├── level (default: 0) - role hierarchy level (lower = more privileged)
├── parent_id (default: -1) - parent role ID for hierarchy
├── lft (default: 0) - Nested Set left value
├── rgt (default: 0) - Nested Set right value
├── created_at
├── updated_at
└── [edges]
    ├── users (one-to-many) - users assigned this role
    └── permissions (one-to-many) - permissions granted to this role
```

**Hierarchy Model:** Uses **Nested Set Model** for efficient descendant queries.
- `lft` (left) and `rgt` (right) bounds define hierarchy
- Query: Find all descendants where `lft >= parent.lft AND rgt <= parent.rgt`
- Example: Admin(0) > Teacher(1) > Student(2)

### 3. Permission Entity
**File:** `/Users/lap14687/personal/github.com/huynhanx03/Judgify/api/internal/ent/schema/permission.go`

```
Permission
├── id (PK)
├── role_id (FK) - required
├── resource_id (FK) - required
├── scopes (int, default: 0) - bitmask of allowed operations
├── description (nullable)
├── created_at
├── updated_at
└── [unique index] (role_id, resource_id)
```

**Scopes Bitmask:**
```
PermissionScopeCreate = 1  (0001)
PermissionScopeRead   = 2  (0010)
PermissionScopeUpdate = 4  (0100)
PermissionScopeDelete = 8  (1000)
```

Example: scopes=7 means CREATE|READ|UPDATE (1+2+4)

### 4. User → Role Assignment
**File:** `/Users/lap14687/personal/github.com/huynhanx03/Judgify/api/internal/ent/schema/user.go`

```
User
├── id (PK)
├── username (unique, not empty)
├── role_id (FK) - required, one user = one role
├── created_at
├── updated_at
└── [edges]
    ├── role (many-to-one)
    └── credentials, attributes, federated_identities, ...
```

---

## Relationship & Data Flow

```
User
  │
  └─→ role_id → Role
                  ├─ Permissions → [permission records]
                  │                  ├─ role_id (self-ref)
                  │                  └─ resource_id → Resource
                  │
                  └─ Hierarchy (via Nested Set: lft, rgt)
                     └─ Includes all ancestor & descendant permissions
```

**Permission Check Flow:**
1. User makes HTTP request with JWT token
2. Middleware extracts `user_id` from context
3. Load User → get `role_id`
4. Query role permissions:
   - Fetch role's `lft, rgt` values
   - Query all roles where `lft >= parent.lft AND rgt <= parent.rgt` (descendants)
   - Load all permissions for these role IDs
   - Build map: `resourceID → (scope bitmask)`
   - **Cache locally** with TTL
5. Check: `(cachedPerms[resourceID] & requiredScope) == requiredScope`

---

## DTOs & Request/Response Contracts

### Permission DTOs
**File:** `/Users/lap14687/personal/github.com/huynhanx03/Judgify/api/internal/identity/core/dto/permission.go`

**CreatePermissionRequest:**
```go
{
  role_id: int (required),
  resource_id: int (required),
  description: string? (optional, max 255),
  scopes: int // Bitmask (e.g., 1+2+4=7)
}
```

**UpdatePermissionRequest:**
```go
{
  // URI param: id
  description: string? (omitempty, max 255),
  scopes: int? (optional)
}
```

**PermissionResponse:**
```go
{
  id: int,
  role_id: int,
  resource_id: int,
  description: string?,
  scopes: int
}
```

### Role DTOs
**File:** `/Users/lap14687/personal/github.com/huynhanx03/Judgify/api/internal/identity/core/dto/role.go`

**CreateRoleRequest:**
```go
{
  name: string (required, 2-50 chars),
  level: int (0-100, default 0),
  parent_id: int? (optional, for hierarchy)
}
```

**UpdateRoleRequest:**
```go
{
  // URI param: id
  name: string? (omitempty, 2-50),
  level: int? (omitempty, 0-100),
  parent_id: int?
}
```

**RoleResponse:**
```go
{
  id: int,
  name: string,
  level: int
}
```

### Resource DTOs
**File:** `/Users/lap14687/personal/github.com/huynhanx03/Judgify/api/internal/identity/core/dto/resource.go`

**CreateResourceRequest:**
```go
{
  key: string (required, 2-50 chars, unique),
  description: string? (optional, max 255)
}
```

**UpdateResourceRequest:**
```go
{
  // URI param: id
  key: string? (omitempty, 2-50),
  description: string? (omitempty, max 255)
}
```

**ResourceResponse:**
```go
{
  id: int,
  key: string,
  description: string?
}
```

---

## Permission Checking Architecture

### Middleware: PermissionChecker
**File:** `/Users/lap14687/personal/github.com/huynhanx03/Judgify/api/pkg/common/http/middlewares/permission.go`

**Dependencies:**
- `UserRepository` - load user by ID
- `RoleRepository` - load role + descendants
- `PermissionRepository` - load permissions by role IDs
- `LocalCache` - in-memory cache with cost tracking

**Two Middleware Methods:**

#### 1. RequirePermission(resourceKey, requiredScope)
- Resolves: `UserID → User.RoleID → Role (cached) → Permission map → bitmask check`
- Caches role permissions keyed by: `"role_permissions:{roleID}"`
- Cache cost: `CacheCostRolePermissions`
- Check logic: `(scopeMask & requiredScope) == requiredScope`
- Returns 403 Forbidden if denied

#### 2. RequireRole(maxLevel)
- Simple level check: `user.role.level <= maxLevel`
- Lower level = more privileged
- Caches role level keyed by: `"role:{roleID}"`
- Example usage: admin-only endpoints check `maxLevel=0`

**Important:** Both rely on Nested Set queries to include descendant role permissions!

### Resource ID Mapper
**File:** `/Users/lap14687/personal/github.com/huynhanx03/Judgify/api/pkg/permissions/mapper.go`

Maps resource keys (strings) to IDs (ints) for permission checking:
- Initialized on app startup via `setupResourceMapping()` in `initialized.go`
- Loads all resources from DB into in-memory map: `map[key]int`
- Thread-safe with RWMutex
- Fallback resolver function if key not cached
- Called in middleware: `permissions.GetResourceID(resourceKey) → ID`

**File:** `/Users/lap14687/personal/github.com/huynhanx03/Judgify/api/internal/infrastructure/initialized.go`
```go
func setupResourceMapping() {
  resources := SELECT * FROM resource
  for r := range resources {
    mapping[r.Key] = r.ID
  }
  permissions.SetResourceMap(mapping)
}
```

---

## Service & Repository Layer

### Permission Service
**File:** `/Users/lap14687/personal/github.com/huynhanx03/Judgify/api/internal/identity/core/service/permission.go`

**Operations:**
- `Find(opts)` - paginated list
- `Get(id)` - single permission
- `Create(req)` - new permission (invalidates cache)
- `Update(id, req)` - edit scopes/description (invalidates cache)
- `Delete(id)` - remove permission (invalidates cache)

**Cache Invalidation:** Every mutation calls `cacheService.InvalidatePermissionConfig(ctx)`

### Permission Repository
**File:** `/Users/lap14687/personal/github.com/huynhanx03/Judgify/api/internal/identity/adapters/driven/db/permission.go`

**Operations:**
- `Find(opts)` - filtered/paginated query
- `Get(id)` - by ID
- `Create(e)` - insert
- `Update(e)` - update by ID
- `Delete(id)` - soft delete (via mixin)
- `FindByRoleIDs(roleIDs)` - **key method for permission aggregation**
- `Exists(id)` - check existence

### Role Repository - Hierarchy Support
**File:** `/Users/lap14687/personal/github.com/huynhanx03/Judgify/api/internal/identity/adapters/driven/db/role.go`

**Key Method:** `FindDescendants(lft, rgt)`
```go
SELECT * FROM role
WHERE lft >= parent.lft AND rgt <= parent.rgt
```
Returns all roles in subtree (includes the node itself).

### Resource Repository
**File:** `/Users/lap14687/personal/github.com/huynhanx03/Judgify/api/internal/identity/adapters/driven/db/resource.go`

Standard CRUD + `FindByIDs(ids)` for bulk lookup.

---

## HTTP Route Protection Examples

### Identity Module
**File:** `/Users/lap14687/personal/github.com/huynhanx03/Judgify/api/internal/identity/adapters/driver/http/handler.go`

```go
// Group-level permission check (applied to all routes in group)
roles := r.Group("/roles", 
  permChecker.RequirePermission(ResourceKeyRole, PermissionScopeRead))
{
  roles.POST("/find", ...) // Inherits READ check
  roles.GET("/:id", ...)   // Inherits READ check
  roles.POST("", 
    permChecker.RequirePermission(ResourceKeyRole, PermissionScopeCreate), ...)
  roles.PUT("/:id", 
    permChecker.RequirePermission(ResourceKeyRole, PermissionScopeUpdate), ...)
  roles.DELETE("/:id", 
    permChecker.RequirePermission(ResourceKeyRole, PermissionScopeDelete), ...)
}

// Permissions, Resources, Attribute Definitions follow same pattern
```

### Problem Module
**File:** `/Users/lap14687/personal/github.com/huynhanx03/Judgify/api/internal/problem/adapters/driver/http/handler.go`

```go
problems := r.Group("/problems")
{
  problems.POST("/find", ...) // No permission check (public)
  problems.GET("/:id", ...)   // No permission check (public)
  problems.POST("", 
    permChecker.RequirePermission(ResourceKeyProblem, PermissionScopeCreate), ...)
  problems.PUT("/:id", 
    permChecker.RequirePermission(ResourceKeyProblem, PermissionScopeUpdate), ...)
  problems.DELETE("/:id", 
    permChecker.RequirePermission(ResourceKeyProblem, PermissionScopeDelete), ...)
}
```

---

## Mappers & Entity Conversion

### Core Mapper
**File:** `/Users/lap14687/personal/github.com/huynhanx03/Judgify/api/internal/identity/core/mapper/permission.go`

- `ToPermissionResponse(entity)` → DTO for API response
- `ToPermissionEntityFromCreate(request)` → entity for storage

### DB Mapper
**File:** `/Users/lap14687/personal/github.com/huynhanx03/Judgify/api/internal/identity/adapters/driven/db/mapper/permission.go`

- `ToPermissionEntity(entModel)` → domain entity from Ent-generated model
- `ToPermissionModel(entity)` → Ent model for DB ops

### Builder
**File:** `/Users/lap14687/personal/github.com/huynhanx03/Judgify/api/internal/identity/adapters/driven/db/builder/permission.go`

- `BuildCreatePermission(ctx, entity)` → Ent CreateBuilder with all fields
- `BuildUpdatePermission(ctx, entity)` → Ent UpdateBuilder

---

## Complete Permission Check Sequence

### Request → Response

**Input:** HTTP request to protected endpoint
```
POST /api/problems (with JWT)
Authorization: Bearer {token}
```

**Middleware Chain (RequirePermission):**

1. **Extract User from Token**
   - Get `userID` from `context.ContextKeyUserID`
   - If missing → 401 Unauthorized

2. **Load User**
   - `userRepo.Get(ctx, userID)` → User entity
   - If not found → 403 Forbidden

3. **Get Role Permissions** (with caching)
   - Cache key: `"role_permissions:{roleID}"`
   - If cached → return `map[resourceID]scopeMask`
   - If not cached:
     - Load role: `roleRepo.Get(ctx, user.roleID)`
     - Get descendants: `roleRepo.FindDescendants(role.lft, role.rgt)`
     - Collect role IDs from descendants
     - Load permissions: `permissionRepo.FindByRoleIDs(roleIDs)`
     - Build map by OR-ing scopes: `perms[resourceID] |= permission.scopes`
     - Cache result with cost

4. **Resolve Resource ID**
   - `permissions.GetResourceID("problem")` → resourceID (from mapper cache)
   - If not found → 403 Forbidden

5. **Check Scope**
   - `(perms[resourceID] & requiredScope) == requiredScope`
   - If false → 403 Forbidden

6. **Allow Request**
   - `c.Next()` → proceed to handler

**Output:** 200/error with response body

---

## Key Design Insights

### 1. Scope Bitmask Pattern
Allows multiple permissions in single int:
- `scopes=15` (0b1111) = CREATE|READ|UPDATE|DELETE (all)
- `scopes=3` (0b0011) = CREATE|READ
- Bitwise AND checks if required bits are set

### 2. Role Hierarchy via Nested Set
Efficient O(1) descendant query without recursion:
- A role inherits all permissions of its ancestors
- Descendants include the role itself
- Used in `FindDescendants(lft, rgt)`

### 3. Two-Level Caching
- **DB-level cache** in middleware for role permissions
  - TTL-based (cost-based eviction)
  - Keyed by roleID
- **Mapper cache** for resource ID lookups
  - Thread-safe RWMutex
  - Thread-safe RWMutex
  - Loaded on startup

### 4. Permission Aggregation
When checking if a role has permission:
- Include all descendant roles' permissions
- OR their scopes together: `perms[resourceID] |= descendant.permission.scopes`
- Example: Parent role(1) + Child role(1) = combined scopes (1|1=1)

### 5. Cache Invalidation
Every permission mutation calls `invalidatePermissionConfig()`:
- Prevents stale cache until explicitly refreshed
- Ensures fresh permissions on next check

---

## Summary Table

| Entity | PK | Key Fields | Hierarchy | Caching |
|--------|----|----|-----------|---------|
| **Resource** | id | key (string) | None | Startup map + mapper |
| **Role** | id | name, level, lft, rgt | Nested Set (lft/rgt) | Per-request cache |
| **Permission** | id | (role_id, resource_id) unique | Via role hierarchy | Cache invalidation |
| **User** | id | username, role_id | Via role | None (loaded per-request) |

---

## Files & Path Summary

| File | Purpose |
|------|---------|
| `/pkg/permissions/constants.go` | Resource keys + scope bitmask constants |
| `/pkg/permissions/mapper.go` | Resource key → ID mapping (startup + cache) |
| `/internal/ent/schema/{role,resource,permission}.go` | Ent schema definitions |
| `/internal/ent/schema/user.go` | User role FK assignment |
| `/internal/identity/core/entity/{role,resource,permission}.go` | Domain entities |
| `/internal/identity/core/dto/{role,resource,permission}.go` | Request/response DTOs |
| `/internal/identity/core/mapper/permission.go` | Entity ↔ DTO conversion |
| `/internal/identity/core/service/permission.go` | Permission service (CRUD + cache invalidation) |
| `/internal/identity/adapters/driven/db/{role,resource,permission}.go` | Repository implementations |
| `/internal/identity/adapters/driven/db/mapper/` | Ent model ↔ domain entity mappers |
| `/internal/identity/adapters/driven/db/builder/permission.go` | Ent query builders |
| `/pkg/common/http/middlewares/permission.go` | Permission checker middleware |
| `/internal/identity/adapters/driver/http/handler.go` | Route registration with middleware |
| `/internal/infrastructure/initialized.go` | Startup resource mapping |

---

## Unresolved Questions

None — the permission system is fully documented and clear.
