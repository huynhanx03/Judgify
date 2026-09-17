# PostgreSQL PREPARE Regression Guard — Full Codebase Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Hoist the existing PostgreSQL PREPARE regression guard into a shared helper and extend it to all 11 remaining packages that contain raw SQL statements, then run every statement against a real migrated database, diagnose and fix every preparation failure.

**Architecture:** A new non-test package `internal/testutil/rawsql` exports `Statements` (AST-based collector) and `PrepareAll` (connects and PREPAREs). Each guarded package gets a tiny `raw_sql_postgres_test.go` that calls `PrepareAll`. The contest package's existing guard is rewritten to delegate to the helper. All SQL defects found by the guard are fixed in the production source files.

**Tech Stack:** Go 1.26, `go/ast` + `go/parser`, `database/sql`, `github.com/lib/pq`, Docker postgres:16, `golangci-lint`.

**Spec:** This file is the spec. The triggering bug and the complete task list are documented here.

## Global Constraints

- Module path: `github.com/huynhanx03/judgify`
- `lib/pq` is already in go.mod; no new dependencies
- All test files must live in the same package as the package they test (not `_test` external packages), unless the existing convention in that directory already uses the external form — match what is already there
- `golangci-lint run --timeout=5m` must produce 0 issues after every task
- `gofmt -l ./cmd ./internal ./pkg` must be silent
- Do NOT add comments unless the file already carries explanatory comments about a non-obvious invariant
- Do NOT `git commit`, `git stage`, `git reset`, or `git checkout`
- Do NOT touch `postgres` or `pgvector` containers, `test/e2e/stack.sh`, or `test/operations/`
- Skip when `JUDGIFY_TEST_DATABASE_URL` is unset; fail when zero statements are discovered
- Container name for this task: `judgify-prepare-guard`

---

### Task 1: Create `internal/testutil/rawsql` shared helper package

**Files:**
- Create: `internal/testutil/rawsql/rawsql.go`

**Interfaces:**
- Produces:
  ```go
  package rawsql

  type Statement struct {
      Name string  // "filename.go:line"
      SQL  string
  }

  // Statements walks dir's non-test Go sources and returns every string
  // literal passed as Args[1] to any QueryContext or ExecContext call.
  // t.Fatal is called if the glob or parse fails.
  func Statements(t *testing.T, dir string) []Statement

  // PrepareAll opens a database/sql connection to dsn, PREPAREs every
  // statement collected from dir, and runs each as a sub-test.
  // It skips when dsn == "" and fatals when zero statements are found.
  func PrepareAll(t *testing.T, dsn, dir string)
  ```

- [ ] **Step 1: Write `internal/testutil/rawsql/rawsql.go`**

```go
package rawsql

import (
	"context"
	"database/sql"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

// Statement is a raw SQL string found in a package's non-test sources.
type Statement struct {
	Name string // "filename.go:line"
	SQL  string
}

// Statements walks the non-test Go sources in dir and returns every string
// literal supplied as the SQL argument (Args[1]) to QueryContext or
// ExecContext. t.Fatal is called on any parse error.
func Statements(t *testing.T, dir string) []Statement {
	t.Helper()

	entries, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		t.Fatalf("rawsql: glob %s: %v", dir, err)
	}
	fileSet := token.NewFileSet()
	statements := make([]Statement, 0, len(entries))
	for _, entry := range entries {
		if strings.HasSuffix(entry, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fileSet, entry, nil, 0)
		if err != nil {
			t.Fatalf("rawsql: parse %s: %v", entry, err)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || len(call.Args) < 2 {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if selector.Sel.Name != "QueryContext" && selector.Sel.Name != "ExecContext" {
				return true
			}
			literal, ok := call.Args[1].(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				return true
			}
			text, err := strconv.Unquote(literal.Value)
			if err != nil || strings.TrimSpace(text) == "" {
				return true
			}
			position := fileSet.Position(literal.Pos())
			statements = append(statements, Statement{
				Name: filepath.Base(position.Filename) + ":" + strconv.Itoa(position.Line),
				SQL:  text,
			})
			return true
		})
	}
	return statements
}

// PrepareAll opens a connection to dsn, skips when dsn is empty, fatals
// when zero statements are discovered in dir, then PREPAREs every statement
// in a sub-test. It mirrors the behaviour of the original contest guard.
func PrepareAll(t *testing.T, dsn, dir string) {
	t.Helper()

	if dsn == "" {
		t.Skipf("JUDGIFY_TEST_DATABASE_URL is not configured")
	}

	statements := Statements(t, dir)
	if len(statements) == 0 {
		t.Fatal("rawsql: no raw SQL statements were discovered; the collector is broken")
	}

	database, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("rawsql: sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := database.PingContext(ctx); err != nil {
		t.Fatalf("rawsql: database.PingContext() error = %v", err)
	}

	for _, stmt := range statements {
		stmt := stmt
		t.Run(stmt.Name, func(t *testing.T) {
			prepared, err := database.PrepareContext(ctx, stmt.SQL)
			if err != nil {
				t.Fatalf("%s does not prepare on PostgreSQL: %v", stmt.Name, err)
			}
			_ = prepared.Close()
		})
	}
}
```

- [ ] **Step 2: Verify it compiles**

```bash
cd /Users/jerry/nhanhmc/coding/Judgify/api && go build ./internal/testutil/rawsql/...
```

Expected: no output, exit 0.

- [ ] **Step 3: Run gofmt and golangci-lint**

```bash
cd /Users/jerry/nhanhmc/coding/Judgify/api && gofmt -l ./internal/testutil/rawsql/ && golangci-lint run --timeout=5m ./internal/testutil/rawsql/...
```

Expected: silent.

---

### Task 2: Rewrite the contest package guard to delegate to the shared helper

**Files:**
- Modify: `internal/contest/adapters/driven/db/raw_sql_postgres_test.go` (full rewrite)

**Interfaces:**
- Consumes: `rawsql.PrepareAll(t, dsn, dir string)` from Task 1

The existing file is 115 lines. We replace it with a ~15-line wrapper. The behaviour is identical: skip when `JUDGIFY_TEST_DATABASE_URL` is unset, fail when zero statements found, PREPARE every literal SQL arg to QueryContext/ExecContext.

- [ ] **Step 1: Overwrite the file**

Replace the entire content of `internal/contest/adapters/driven/db/raw_sql_postgres_test.go` with:

```go
package db

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/huynhanx03/judgify/internal/testutil/rawsql"
)

// PostgreSQL resolves parameter types when a statement is prepared, so a query
// that deduces two different types for one placeholder fails with 42P08 before
// a single row is touched. sqlmock never runs the planner, so unit tests cannot
// see that class of defect at all: it reached this repository and broke every
// contest lifecycle transition against a real database.
//
// Preparing every raw statement in the package needs no fixture data, so it
// stays cheap while covering each statement that exists now or is added later.
func TestEveryRawStatement(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	rawsql.PrepareAll(t, os.Getenv("JUDGIFY_TEST_DATABASE_URL"), filepath.Dir(file))
}
```

- [ ] **Step 2: Verify package builds and test compiles**

```bash
cd /Users/jerry/nhanhmc/coding/Judgify/api && go test -run TestEveryRawStatement -count=1 ./internal/contest/adapters/driven/db/...
```

Expected: `--- SKIP: TestEveryRawStatement` (because DSN is unset).

- [ ] **Step 3: Run gofmt and golangci-lint**

```bash
cd /Users/jerry/nhanhmc/coding/Judgify/api && gofmt -l ./internal/contest/adapters/driven/db/ && golangci-lint run --timeout=5m ./internal/contest/adapters/driven/db/...
```

Expected: silent.

---

### Task 3: Add the guard to `internal/submission/adapters/driven/db`

**Files:**
- Create: `internal/submission/adapters/driven/db/raw_sql_postgres_test.go`

The package name in this directory is `db` (check with `head -1` of any non-test file).

- [ ] **Step 1: Verify package name**

```bash
head -1 /Users/jerry/nhanhmc/coding/Judgify/api/internal/submission/adapters/driven/db/submission.go
```

Expected: `package db`

- [ ] **Step 2: Create the file**

```go
package db

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/huynhanx03/judgify/internal/testutil/rawsql"
)

func TestEveryRawStatement(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	rawsql.PrepareAll(t, os.Getenv("JUDGIFY_TEST_DATABASE_URL"), filepath.Dir(file))
}
```

- [ ] **Step 3: Verify it skips without DSN**

```bash
cd /Users/jerry/nhanhmc/coding/Judgify/api && go test -run TestEveryRawStatement -count=1 ./internal/submission/adapters/driven/db/...
```

Expected: `--- SKIP`.

- [ ] **Step 4: Run gofmt and golangci-lint**

```bash
cd /Users/jerry/nhanhmc/coding/Judgify/api && gofmt -l ./internal/submission/adapters/driven/db/ && golangci-lint run --timeout=5m ./internal/submission/adapters/driven/db/...
```

---

### Task 4: Add the guard to `internal/notification/adapters/driven/db`

**Files:**
- Create: `internal/notification/adapters/driven/db/raw_sql_postgres_test.go`

- [ ] **Step 1: Verify package name**

```bash
head -1 /Users/jerry/nhanhmc/coding/Judgify/api/internal/notification/adapters/driven/db/campaign.go
```

- [ ] **Step 2: Create the file** (use same template as Task 3, matching the package name found in Step 1)

```go
package db

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/huynhanx03/judgify/internal/testutil/rawsql"
)

func TestEveryRawStatement(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	rawsql.PrepareAll(t, os.Getenv("JUDGIFY_TEST_DATABASE_URL"), filepath.Dir(file))
}
```

- [ ] **Step 3: Verify it skips without DSN**

```bash
cd /Users/jerry/nhanhmc/coding/Judgify/api && go test -run TestEveryRawStatement -count=1 ./internal/notification/adapters/driven/db/...
```

- [ ] **Step 4: Run gofmt and golangci-lint**

```bash
cd /Users/jerry/nhanhmc/coding/Judgify/api && gofmt -l ./internal/notification/adapters/driven/db/ && golangci-lint run --timeout=5m ./internal/notification/adapters/driven/db/...
```

---

### Task 5: Add the guard to `internal/cultivation/adapters/driven/db`

**Files:**
- Create: `internal/cultivation/adapters/driven/db/raw_sql_postgres_test.go`

- [ ] **Step 1: Verify package name**

```bash
head -1 /Users/jerry/nhanhmc/coding/Judgify/api/internal/cultivation/adapters/driven/db/progression.go
```

- [ ] **Step 2: Create the file**

```go
package db

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/huynhanx03/judgify/internal/testutil/rawsql"
)

func TestEveryRawStatement(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	rawsql.PrepareAll(t, os.Getenv("JUDGIFY_TEST_DATABASE_URL"), filepath.Dir(file))
}
```

- [ ] **Step 3: Verify it skips without DSN**

```bash
cd /Users/jerry/nhanhmc/coding/Judgify/api && go test -run TestEveryRawStatement -count=1 ./internal/cultivation/adapters/driven/db/...
```

- [ ] **Step 4: Run gofmt and golangci-lint**

```bash
cd /Users/jerry/nhanhmc/coding/Judgify/api && gofmt -l ./internal/cultivation/adapters/driven/db/ && golangci-lint run --timeout=5m ./internal/cultivation/adapters/driven/db/...
```

---

### Task 6: Add the guard to `internal/material/adapters/driven/db`

**Files:**
- Create: `internal/material/adapters/driven/db/raw_sql_postgres_test.go`

Note: `internal/material/adapters/driven/db/query_safety_test.go` already exists in this package — it tests different things (sqlmock query shape). The new file will be a separate test function in the same package.

- [ ] **Step 1: Verify package name**

```bash
head -1 /Users/jerry/nhanhmc/coding/Judgify/api/internal/material/adapters/driven/db/material_category.go
```

- [ ] **Step 2: Create the file**

```go
package db

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/huynhanx03/judgify/internal/testutil/rawsql"
)

func TestEveryRawStatement(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	rawsql.PrepareAll(t, os.Getenv("JUDGIFY_TEST_DATABASE_URL"), filepath.Dir(file))
}
```

- [ ] **Step 3: Verify it skips without DSN**

```bash
cd /Users/jerry/nhanhmc/coding/Judgify/api && go test -run TestEveryRawStatement -count=1 ./internal/material/adapters/driven/db/...
```

- [ ] **Step 4: Run gofmt and golangci-lint**

```bash
cd /Users/jerry/nhanhmc/coding/Judgify/api && gofmt -l ./internal/material/adapters/driven/db/ && golangci-lint run --timeout=5m ./internal/material/adapters/driven/db/...
```

---

### Task 7: Add the guard to `internal/identity/adapters/driven/db`

**Files:**
- Create: `internal/identity/adapters/driven/db/raw_sql_postgres_test.go`

- [ ] **Step 1: Verify package name**

```bash
head -1 /Users/jerry/nhanhmc/coding/Judgify/api/internal/identity/adapters/driven/db/public_profile.go
```

- [ ] **Step 2: Create the file**

```go
package db

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/huynhanx03/judgify/internal/testutil/rawsql"
)

func TestEveryRawStatement(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	rawsql.PrepareAll(t, os.Getenv("JUDGIFY_TEST_DATABASE_URL"), filepath.Dir(file))
}
```

- [ ] **Step 3: Verify it skips without DSN**

```bash
cd /Users/jerry/nhanhmc/coding/Judgify/api && go test -run TestEveryRawStatement -count=1 ./internal/identity/adapters/driven/db/...
```

- [ ] **Step 4: Run gofmt and golangci-lint**

```bash
cd /Users/jerry/nhanhmc/coding/Judgify/api && gofmt -l ./internal/identity/adapters/driven/db/ && golangci-lint run --timeout=5m ./internal/identity/adapters/driven/db/...
```

---

### Task 8: Add the guard to `internal/realtime/adapters/driven/db`

**Files:**
- Create: `internal/realtime/adapters/driven/db/raw_sql_postgres_test.go`

- [ ] **Step 1: Verify package name**

```bash
head -1 /Users/jerry/nhanhmc/coding/Judgify/api/internal/realtime/adapters/driven/db/replay.go
```

- [ ] **Step 2: Create the file**

```go
package db

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/huynhanx03/judgify/internal/testutil/rawsql"
)

func TestEveryRawStatement(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	rawsql.PrepareAll(t, os.Getenv("JUDGIFY_TEST_DATABASE_URL"), filepath.Dir(file))
}
```

- [ ] **Step 3: Verify it skips without DSN**

```bash
cd /Users/jerry/nhanhmc/coding/Judgify/api && go test -run TestEveryRawStatement -count=1 ./internal/realtime/adapters/driven/db/...
```

- [ ] **Step 4: Run gofmt and golangci-lint**

```bash
cd /Users/jerry/nhanhmc/coding/Judgify/api && gofmt -l ./internal/realtime/adapters/driven/db/ && golangci-lint run --timeout=5m ./internal/realtime/adapters/driven/db/...
```

---

### Task 9: Add the guard to `internal/platform/observability`

**Files:**
- Create: `internal/platform/observability/raw_sql_postgres_test.go`

Package name is `observability` (confirmed from `durable.go`).

- [ ] **Step 1: Verify package name**

```bash
head -1 /Users/jerry/nhanhmc/coding/Judgify/api/internal/platform/observability/durable.go
```

Expected: `package observability`

- [ ] **Step 2: Create the file**

```go
package observability

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/huynhanx03/judgify/internal/testutil/rawsql"
)

func TestEveryRawStatement(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	rawsql.PrepareAll(t, os.Getenv("JUDGIFY_TEST_DATABASE_URL"), filepath.Dir(file))
}
```

- [ ] **Step 3: Verify it skips without DSN**

```bash
cd /Users/jerry/nhanhmc/coding/Judgify/api && go test -run TestEveryRawStatement -count=1 ./internal/platform/observability/...
```

- [ ] **Step 4: Run gofmt and golangci-lint**

```bash
cd /Users/jerry/nhanhmc/coding/Judgify/api && gofmt -l ./internal/platform/observability/ && golangci-lint run --timeout=5m ./internal/platform/observability/...
```

---

### Task 10: Add the guard to `internal/judge/operations`

**Files:**
- Create: `internal/judge/operations/raw_sql_postgres_test.go`

Package name is `operations`. This package uses `client.DB(ctx).ActivatedJudgeCapability.QueryContext(ctx, sql)` — `QueryContext` with the SQL as Args[1], so the AST collector picks it up identically to `database/sql` calls.

- [ ] **Step 1: Verify package name**

```bash
head -1 /Users/jerry/nhanhmc/coding/Judgify/api/internal/judge/operations/query.go
```

Expected: `package operations`

- [ ] **Step 2: Create the file**

```go
package operations

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/huynhanx03/judgify/internal/testutil/rawsql"
)

func TestEveryRawStatement(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	rawsql.PrepareAll(t, os.Getenv("JUDGIFY_TEST_DATABASE_URL"), filepath.Dir(file))
}
```

- [ ] **Step 3: Verify it skips without DSN**

```bash
cd /Users/jerry/nhanhmc/coding/Judgify/api && go test -run TestEveryRawStatement -count=1 ./internal/judge/operations/...
```

- [ ] **Step 4: Run gofmt and golangci-lint**

```bash
cd /Users/jerry/nhanhmc/coding/Judgify/api && gofmt -l ./internal/judge/operations/ && golangci-lint run --timeout=5m ./internal/judge/operations/...
```

---

### Task 11: Add the guard to `internal/infrastructure/worker`

**Files:**
- Create: `internal/infrastructure/worker/raw_sql_postgres_test.go`

Package name is `worker`. Note: `workerLeaseReadinessSQL` is a `const` string used as a *variable argument* to `QueryContext`, so the AST collector (which only collects literal strings at the call site) will NOT pick it up automatically. The helper will find the 2 inline literal strings only. The `const` must be verified by manually adding it as a supplemental case, OR the helper must be extended to also scan `const` and `var` top-level string declarations whose names end in `SQL` and then confirm the collector sees the right number.

**Decision:** Extend the guard in this package to also add the const SQL manually. Use a package-level `extraStatements` slice and a local `TestEveryRawStatement` that calls `rawsql.Statements` for inline literals, appends the const SQL, then PREPAREs all of them. This keeps the shared helper simple and the package-level extra explicit.

- [ ] **Step 1: Verify package name and count**

```bash
head -1 /Users/jerry/nhanhmc/coding/Judgify/api/internal/infrastructure/worker/readiness_database.go
grep -n "workerLeaseReadinessSQL\|QueryContext.*\`\|ExecContext.*\`" /Users/jerry/nhanhmc/coding/Judgify/api/internal/infrastructure/worker/readiness_database.go | head -20
```

- [ ] **Step 2: Create the file**

```go
package worker

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	_ "github.com/lib/pq"

	"github.com/huynhanx03/judgify/internal/testutil/rawsql"
)

func TestEveryRawStatement(t *testing.T) {
	dsn := os.Getenv("JUDGIFY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skipf("JUDGIFY_TEST_DATABASE_URL is not configured")
	}

	_, file, _, _ := runtime.Caller(0)
	dir := filepath.Dir(file)

	statements := rawsql.Statements(t, dir)
	// workerLeaseReadinessSQL is a package-level const passed as a variable
	// argument to QueryContext; the AST collector only sees literal call-site
	// strings, so we add it explicitly here.
	statements = append(statements, rawsql.Statement{
		Name: "readiness_database.go:workerLeaseReadinessSQL",
		SQL:  workerLeaseReadinessSQL,
	})
	if len(statements) == 0 {
		t.Fatal("rawsql: no raw SQL statements were discovered; the collector is broken")
	}

	database, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := database.PingContext(ctx); err != nil {
		t.Fatalf("database.PingContext() error = %v", err)
	}

	for _, stmt := range statements {
		stmt := stmt
		t.Run(stmt.Name, func(t *testing.T) {
			prepared, err := database.PrepareContext(ctx, stmt.SQL)
			if err != nil {
				t.Fatalf("%s does not prepare on PostgreSQL: %v", stmt.Name, err)
			}
			_ = prepared.Close()
		})
	}
}
```

- [ ] **Step 3: Verify it skips without DSN**

```bash
cd /Users/jerry/nhanhmc/coding/Judgify/api && go test -run TestEveryRawStatement -count=1 ./internal/infrastructure/worker/...
```

- [ ] **Step 4: Run gofmt and golangci-lint**

```bash
cd /Users/jerry/nhanhmc/coding/Judgify/api && gofmt -l ./internal/infrastructure/worker/ && golangci-lint run --timeout=5m ./internal/infrastructure/worker/...
```

---

### Task 12: Add the guard to `internal/ent/migrate`

**Files:**
- Create: `internal/ent/migrate/raw_sql_postgres_test.go`

Package name is `migration`. The existing test files in this directory use `package migration`.

- [ ] **Step 1: Verify package name**

```bash
head -1 /Users/jerry/nhanhmc/coding/Judgify/api/internal/ent/migrate/revision_store.go
```

Expected: `package migration`

- [ ] **Step 2: Create the file**

```go
package migration

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/huynhanx03/judgify/internal/testutil/rawsql"
)

func TestEveryRawStatement(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	rawsql.PrepareAll(t, os.Getenv("JUDGIFY_TEST_DATABASE_URL"), filepath.Dir(file))
}
```

- [ ] **Step 3: Verify it skips without DSN**

```bash
cd /Users/jerry/nhanhmc/coding/Judgify/api && go test -run TestEveryRawStatement -count=1 ./internal/ent/migrate/...
```

- [ ] **Step 4: Run gofmt and golangci-lint**

```bash
cd /Users/jerry/nhanhmc/coding/Judgify/api && gofmt -l ./internal/ent/migrate/ && golangci-lint run --timeout=5m ./internal/ent/migrate/...
```

---

### Task 13: Add the guard to `internal/platform/audit`

**Files:**
- Create: `internal/platform/audit/raw_sql_postgres_test.go`

Package name is `audit`.

- [ ] **Step 1: Verify package name**

```bash
head -1 /Users/jerry/nhanhmc/coding/Judgify/api/internal/platform/audit/repository.go
```

Expected: `package audit`

- [ ] **Step 2: Create the file**

```go
package audit

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/huynhanx03/judgify/internal/testutil/rawsql"
)

func TestEveryRawStatement(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	rawsql.PrepareAll(t, os.Getenv("JUDGIFY_TEST_DATABASE_URL"), filepath.Dir(file))
}
```

- [ ] **Step 3: Verify it skips without DSN**

```bash
cd /Users/jerry/nhanhmc/coding/Judgify/api && go test -run TestEveryRawStatement -count=1 ./internal/platform/audit/...
```

- [ ] **Step 4: Run gofmt and golangci-lint**

```bash
cd /Users/jerry/nhanhmc/coding/Judgify/api && gofmt -l ./internal/platform/audit/ && golangci-lint run --timeout=5m ./internal/platform/audit/...
```

---

### Task 14: Full suite lint and skip verification

Before spinning up any database, confirm the entire suite compiles and skips cleanly.

- [ ] **Step 1: gofmt**

```bash
cd /Users/jerry/nhanhmc/coding/Judgify/api && gofmt -l ./cmd ./internal ./pkg
```

Expected: silent (no files listed).

- [ ] **Step 2: golangci-lint**

```bash
cd /Users/jerry/nhanhmc/coding/Judgify/api && golangci-lint run --timeout=5m
```

Expected: 0 issues.

- [ ] **Step 3: go test without DSN**

```bash
cd /Users/jerry/nhanhmc/coding/Judgify/api && go test ./... -count=1 2>&1 | grep -E "SKIP|FAIL|ok"
```

Expected: every TestEveryRawStatement line shows `--- SKIP`, no `FAIL`.

---

### Task 15: Spin up Docker, migrate, run guard, diagnose and fix every failure

This is the most important task. Expect failures. Each one is a real production bug.

- [ ] **Step 1: Start the container**

```bash
cd /Users/jerry/nhanhmc/coding/Judgify
docker run --pull never --rm -d --name judgify-prepare-guard \
  -e POSTGRES_USER=postgres -e POSTGRES_PASSWORD=t -e POSTGRES_DB=judgify \
  -p 127.0.0.1::5432 postgres:16
```

- [ ] **Step 2: Wait for PostgreSQL to be ready**

```bash
until docker exec judgify-prepare-guard pg_isready -U postgres; do sleep 1; done
```

Expected: `localhost:5432 - accepting connections`

- [ ] **Step 3: Apply init-roles.sql**

```bash
cd /Users/jerry/nhanhmc/coding/Judgify
PORT=$(docker port judgify-prepare-guard 5432/tcp | awk -F: 'NR==1{print $NF}')
docker exec -i -e POSTGRES_DB=judgify -e JUDGIFY_MIGRATOR_PASSWORD=m \
  -e JUDGIFY_API_PASSWORD=a -e JUDGIFY_WORKER_PASSWORD=w \
  judgify-prepare-guard psql --no-psqlrc --set ON_ERROR_STOP=1 -U postgres -d judgify \
  < deploy/postgres/init-roles.sql
```

Expected: output showing role creation, exit 0.

- [ ] **Step 4: Run the migrator**

```bash
cd /Users/jerry/nhanhmc/coding/Judgify
PORT=$(docker port judgify-prepare-guard 5432/tcp | awk -F: 'NR==1{print $NF}')
cd api && env GO_ENV=local DATABASE_DRIVER=postgres DATABASE_HOST=127.0.0.1 DATABASE_PORT=$PORT \
  DATABASE_USERNAME=judgify_migrator DATABASE_PASSWORD=m DATABASE_DATABASE=judgify \
  LOGGER_FILE_LOG_NAME=/tmp/pg.log go run ./cmd/migrate
```

Expected: migrator exits 0, schema is applied.

- [ ] **Step 5: Run the guard across all packages**

```bash
cd /Users/jerry/nhanhmc/coding/Judgify
PORT=$(docker port judgify-prepare-guard 5432/tcp | awk -F: 'NR==1{print $NF}')
JUDGIFY_TEST_DATABASE_URL="postgres://judgify_migrator:m@127.0.0.1:$PORT/judgify?sslmode=disable" \
  go test -v ./internal/... -run TestEveryRawStatement -count=1 2>&1 | tee /tmp/prepare-guard-run.txt
```

- [ ] **Step 6: Triage every FAIL**

For each failing test, note:
  - The file and line number (from the test name)
  - The exact PostgreSQL error (SQLSTATE code + message)
  - The SQL statement text

Then for each failure:

  a. **42P08 inconsistent types for $N** — A parameter appears in two positions where PostgreSQL infers different types (e.g. in a SET clause yielding `character varying` and a CASE comparison yielding `text`). Fix: add `::text` (or the correct type cast) to every use of that placeholder in the statement.

  b. **42703 column does not exist** — The SQL references a column name that is not in the actual schema. Read the surrounding Go code to understand intent, then check the schema (Ent definitions in `internal/ent/schema/` and actual migration SQL in `internal/ent/migrate/migrations/`). Find the correct column name and fix the statement.

  c. **42P01 relation does not exist** — The SQL references a table that does not exist. Same approach: read Go context, check schema, fix.

  d. **42804 type mismatch** — A value of one type is compared to or assigned to a column of a different type. Add the correct cast.

  For each fix: edit the production Go source file containing the SQL. Do NOT change the statement's semantics, only its syntax to satisfy the planner.

- [ ] **Step 7: Re-run after fixes until all pass**

```bash
cd /Users/jerry/nhanhmc/coding/Judgify
PORT=$(docker port judgify-prepare-guard 5432/tcp | awk -F: 'NR==1{print $NF}')
JUDGIFY_TEST_DATABASE_URL="postgres://judgify_migrator:m@127.0.0.1:$PORT/judgify?sslmode=disable" \
  go test -v ./internal/... -run TestEveryRawStatement -count=1 2>&1 | grep -E "PASS|FAIL|SKIP"
```

Expected: every package shows `PASS` or `SKIP` (the ent/generate packages have no test), no `FAIL`.

- [ ] **Step 8: Verify the guard still bites (regression check)**

Pick one fix from Step 6 — e.g. a `::text` cast you added. Temporarily revert it by editing the source file. Re-run the guard for that package:

```bash
cd /Users/jerry/nhanhmc/coding/Judgify
PORT=$(docker port judgify-prepare-guard 5432/tcp | awk -F: 'NR==1{print $NF}')
JUDGIFY_TEST_DATABASE_URL="postgres://judgify_migrator:m@127.0.0.1:$PORT/judgify?sslmode=disable" \
  go test -v ./internal/<package>/... -run TestEveryRawStatement -count=1
```

Expected: `FAIL` with the 42P08 (or appropriate) error. Confirm the error appears, then restore the fix.

- [ ] **Step 9: Final full-suite run**

```bash
cd /Users/jerry/nhanhmc/coding/Judgify
PORT=$(docker port judgify-prepare-guard 5432/tcp | awk -F: 'NR==1{print $NF}')
JUDGIFY_TEST_DATABASE_URL="postgres://judgify_migrator:m@127.0.0.1:$PORT/judgify?sslmode=disable" \
  go test ./internal/... -run TestEveryRawStatement -count=1
```

Expected: all pass.

- [ ] **Step 10: Final lint and format check**

```bash
cd /Users/jerry/nhanhmc/coding/Judgify/api && gofmt -l ./cmd ./internal ./pkg && golangci-lint run --timeout=5m
```

Expected: silent.

---

### Task 16: Clean up

- [ ] **Step 1: Stop and remove the container**

```bash
docker rm -f judgify-prepare-guard
```

- [ ] **Step 2: Verify no container remains**

```bash
docker ps -a | grep judgify-prepare-guard || echo "clean"
```

Expected: `clean`.

---

## Report Checklist (fill in after execution)

At the end, report:

1. **Shared helper API** — package path, exported symbols, brief description
2. **Per-package statement counts** — how many discovered, how many prepared successfully
3. **Every production SQL defect** — for each:
   - File and line
   - PostgreSQL SQLSTATE + error message
   - The original broken SQL fragment
   - The fix applied (minimal diff)
   - Reasoning (why that fix is correct)
4. **Guard-still-bites confirmation** — which statement was reverted, what error appeared
5. **Final state** — all green, lint clean, container removed
