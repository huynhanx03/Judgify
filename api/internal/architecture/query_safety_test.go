package architecture_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDynamicEntQueriesAlwaysDeclareColumnAllowlists(t *testing.T) {
	root := filepath.Join(moduleRoot(t), "internal")
	var violations []string

	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") || strings.Contains(path, string(filepath.Separator)+"ent"+string(filepath.Separator)+"generate"+string(filepath.Separator)) {
			return nil
		}

		set := token.NewFileSet()
		file, err := parser.ParseFile(set, path, nil, 0)
		if err != nil {
			return err
		}
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			switch selector.Sel.Name {
			case "ApplyFilters", "ApplySort", "ApplyQueryFilters", "ApplyQueryWindow", "ApplyQueryOptions":
				if len(call.Args) < 3 {
					violations = append(violations, set.Position(call.Pos()).String()+": "+selector.Sel.Name+" requires an explicit column allowlist")
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("scan dynamic queries: %v", err)
	}
	if len(violations) > 0 {
		t.Fatalf("unbounded dynamic Ent query contracts:\n%s", strings.Join(violations, "\n"))
	}
}
