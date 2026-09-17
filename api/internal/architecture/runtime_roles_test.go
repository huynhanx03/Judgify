package architecture_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

func TestAPIRoleHasNoDocker(t *testing.T) {
	roots := loadPackages(t, "./cmd/server")
	visited := make(map[string]struct{})
	violations := make([]string, 0)

	var visit func(*packages.Package)
	visit = func(pkg *packages.Package) {
		if _, ok := visited[pkg.PkgPath]; ok {
			return
		}
		visited[pkg.PkgPath] = struct{}{}

		for importedPath, importedPkg := range pkg.Imports {
			if isForbiddenAPIRuntimeImport(importedPath) {
				violations = append(violations, formatViolation("api-role", pkg.PkgPath, importedPath))
			}
			visit(importedPkg)
		}
	}

	for _, root := range roots {
		visit(root)
	}

	assertViolationBaseline(t, "api-role", violations)
}

func isForbiddenAPIRuntimeImport(importedPath string) bool {
	return importedPath == modulePath+"/internal/judge/executor" ||
		importedPath == "github.com/huynhanx03/go-common/pkg/mq/forge" ||
		strings.HasPrefix(importedPath, "github.com/docker/docker")
}

func TestWorkerRoleHasNoProductHTTPRouter(t *testing.T) {
	roots := loadPackages(t, "./cmd/judge-worker")
	visited := make(map[string]struct{})
	violations := make([]string, 0)

	var visit func(*packages.Package)
	visit = func(pkg *packages.Package) {
		if _, ok := visited[pkg.PkgPath]; ok {
			return
		}
		visited[pkg.PkgPath] = struct{}{}
		for importedPath, importedPkg := range pkg.Imports {
			if strings.Contains(importedPath, "/adapters/driver/http") ||
				importedPath == "github.com/gin-gonic/gin" {
				violations = append(violations, formatViolation("worker-role", pkg.PkgPath, importedPath))
			}
			visit(importedPkg)
		}
	}
	for _, root := range roots {
		visit(root)
	}
	if len(violations) != 0 {
		t.Fatalf("worker role imports product HTTP runtime:\n%s", strings.Join(violations, "\n"))
	}
}

func TestSignalsFatalAndGoroutinesStayOutOfComposition(t *testing.T) {
	root := moduleRoot(t)
	paths := []string{
		filepath.Join(root, "internal", "infrastructure"),
		filepath.Join(root, "internal", "di"),
	}

	for _, directory := range paths {
		err := filepath.WalkDir(directory, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
			if err != nil {
				return err
			}
			ast.Inspect(file, func(node ast.Node) bool {
				switch typed := node.(type) {
				case *ast.GoStmt:
					t.Errorf("composition starts goroutine in %s:%d", path, typed.Pos())
				case *ast.SelectorExpr:
					if typed.Sel.Name == "Fatal" || typed.Sel.Name == "Fatalf" {
						t.Errorf("composition terminates process in %s:%d", path, typed.Pos())
					}
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatalf("scan %s: %v", directory, err)
		}
	}

	directories := []string{filepath.Join(root, "internal")}
	legacyGlobal := filepath.Join(root, "global")
	if _, err := os.Stat(legacyGlobal); err == nil {
		directories = append(directories, legacyGlobal)
	} else if !os.IsNotExist(err) {
		t.Fatalf("inspect legacy global directory: %v", err)
	}
	for _, directory := range directories {
		err := filepath.WalkDir(directory, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if strings.Contains(string(data), "os/signal") || strings.Contains(string(data), "signal.Notify") {
				t.Errorf("operating-system signal ownership outside main: %s", path)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("scan signals in %s: %v", directory, err)
		}
	}
}

func TestNoAmbientMutableDependencyOwnership(t *testing.T) {
	root := moduleRoot(t)
	forbiddenNames := map[string]struct{}{
		"GlobalContainer": {},
		"Config":          {},
		"LoggerZap":       {},
		"EntClient":       {},
		"LocalCache":      {},
	}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == "generate" || entry.Name() == "third_party" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imported := range file.Imports {
			if strings.Trim(imported.Path.Value, "\"") == modulePath+"/global" {
				t.Errorf("ambient global dependency import in %s", path)
			}
		}
		file, err = parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		for _, declaration := range file.Decls {
			general, ok := declaration.(*ast.GenDecl)
			if !ok || general.Tok != token.VAR {
				continue
			}
			for _, spec := range general.Specs {
				value, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for _, name := range value.Names {
					if _, forbidden := forbiddenNames[name.Name]; forbidden {
						t.Errorf("mutable dependency singleton %s in %s", name.Name, path)
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scan ambient dependency ownership: %v", err)
	}
}
