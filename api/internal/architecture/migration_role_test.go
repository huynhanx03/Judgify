package architecture_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

func TestLongRunningRolesDoNotImportMigrationRuntime(t *testing.T) {
	t.Parallel()

	for _, command := range []string{"./cmd/server", "./cmd/judge-worker"} {
		roots := loadPackages(t, command)
		visited := make(map[string]struct{})
		violations := make([]string, 0)
		var visit func(*packages.Package)
		visit = func(pkg *packages.Package) {
			if _, exists := visited[pkg.PkgPath]; exists {
				return
			}
			visited[pkg.PkgPath] = struct{}{}
			for importedPath, importedPackage := range pkg.Imports {
				if importedPath == modulePath+"/internal/ent/migrate" {
					violations = append(violations, formatViolation(command, pkg.PkgPath, importedPath))
				}
				visit(importedPackage)
			}
		}
		for _, root := range roots {
			visit(root)
		}
		if len(violations) > 0 {
			t.Fatalf("%s imports operator-only migration runtime: %v", command, violations)
		}
	}
}

func TestMigrationRuntimeDoesNotImportProductOrLongRunningGraphs(t *testing.T) {
	t.Parallel()

	roots := loadPackages(t, "./internal/ent/migrate")
	visited := make(map[string]struct{})
	violations := make([]string, 0)
	var visit func(*packages.Package)
	visit = func(pkg *packages.Package) {
		if _, exists := visited[pkg.PkgPath]; exists {
			return
		}
		visited[pkg.PkgPath] = struct{}{}
		for importedPath, importedPackage := range pkg.Imports {
			if isForbiddenMigrationRuntimeImport(importedPath) {
				violations = append(violations, formatViolation("migration-role", pkg.PkgPath, importedPath))
			}
			visit(importedPackage)
		}
	}
	for _, root := range roots {
		visit(root)
	}
	if len(violations) > 0 {
		t.Fatalf("migration runtime imports long-running or product graph: %v", violations)
	}
}

func TestProductionMigrationCommandIsOperatorOnly(t *testing.T) {
	t.Parallel()

	roots := loadPackages(t, "./cmd/migrate")
	visited := make(map[string]struct{})
	violations := make([]string, 0)
	importsRunner := false
	var visit func(*packages.Package)
	visit = func(pkg *packages.Package) {
		if _, exists := visited[pkg.PkgPath]; exists {
			return
		}
		visited[pkg.PkgPath] = struct{}{}
		for importedPath, importedPackage := range pkg.Imports {
			if importedPath == modulePath+"/internal/ent/migrate" {
				importsRunner = true
			}
			if isForbiddenMigrationCommandImport(importedPath) {
				violations = append(violations, formatViolation("migration-command", pkg.PkgPath, importedPath))
			}
			visit(importedPackage)
		}
	}
	for _, root := range roots {
		visit(root)
	}
	if !importsRunner {
		t.Fatal("production migrate command does not import the checked-in migration runner")
	}
	if len(violations) > 0 {
		t.Fatalf("production migrate command imports product/long-running graph: %v", violations)
	}
}

func TestMakeMigrationApplyCannotBypassProductionRunner(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile(filepath.Join(moduleRoot(t), "..", "Makefile"))
	if err != nil {
		t.Fatalf("read Makefile: %v", err)
	}
	makefile := string(data)
	if strings.Contains(makefile, "atlas migrate apply") {
		t.Fatal("Makefile bypasses the production runner with atlas migrate apply")
	}
	if !strings.Contains(makefile, "go run ./cmd/migrate") {
		t.Fatal("migrate-apply does not invoke the production migrate artifact")
	}
}

func isForbiddenMigrationRuntimeImport(importedPath string) bool {
	for _, prefix := range []string{
		modulePath + "/internal/contest",
		modulePath + "/internal/cultivation",
		modulePath + "/internal/identity",
		modulePath + "/internal/judge",
		modulePath + "/internal/material",
		modulePath + "/internal/problem",
		modulePath + "/internal/submission",
		modulePath + "/internal/infrastructure",
		modulePath + "/internal/ent/generate",
		"github.com/docker/docker",
		"github.com/gin-gonic/gin",
		"github.com/huynhanx03/go-common/pkg/mq/forge",
	} {
		if importedPath == prefix || strings.HasPrefix(importedPath, prefix+"/") {
			return true
		}
	}
	return false
}

func isForbiddenMigrationCommandImport(importedPath string) bool {
	for _, prefix := range []string{
		modulePath + "/internal/contest",
		modulePath + "/internal/cultivation",
		modulePath + "/internal/identity",
		modulePath + "/internal/judge",
		modulePath + "/internal/material",
		modulePath + "/internal/problem",
		modulePath + "/internal/submission",
		modulePath + "/internal/infrastructure/worker",
		"github.com/docker/docker",
		"github.com/gin-gonic/gin",
		"github.com/huynhanx03/go-common/pkg/mq/forge",
	} {
		if importedPath == prefix || strings.HasPrefix(importedPath, prefix+"/") {
			return true
		}
	}
	return false
}
