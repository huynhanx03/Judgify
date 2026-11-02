package architecture_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/huynhanx03/judgify/pkg/permissions"
)

func TestEveryRouteCapabilityExistsInCanonicalCatalog(t *testing.T) {
	root := moduleRoot(t)
	constants := permissionConstants(t, filepath.Join(root, "pkg", "permissions", "constants.go"))
	internal := filepath.Join(root, "internal")

	err := filepath.WalkDir(internal, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		set := token.NewFileSet()
		file, parseErr := parser.ParseFile(set, path, nil, 0)
		if parseErr != nil {
			return parseErr
		}
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || selector.Sel.Name != "RequirePermission" {
				return true
			}
			position := set.Position(call.Pos())
			if len(call.Args) != 2 {
				t.Errorf("%s: RequirePermission must use exactly resource/action", position)
				return true
			}
			resourceName, resourceOK := selectorName(call.Args[0])
			actionName, actionOK := selectorName(call.Args[1])
			if !resourceOK || !actionOK {
				t.Errorf("%s: route capability must use canonical permission constants", position)
				return true
			}
			resource, resourceExists := constants[resourceName]
			action, actionExists := constants[actionName]
			if !resourceExists || !actionExists {
				t.Errorf("%s: unresolved permission constants %s/%s", position, resourceName, actionName)
				return true
			}
			if !permissions.Contains(resource, action) {
				t.Errorf("%s: route capability %s:%s is absent from canonical catalog", position, resource, action)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("scan route capabilities: %v", err)
	}
}

func permissionConstants(t *testing.T, filename string) map[string]string {
	t.Helper()
	set := token.NewFileSet()
	file, err := parser.ParseFile(set, filename, nil, 0)
	if err != nil {
		t.Fatalf("parse permission constants: %v", err)
	}
	constants := make(map[string]string)
	for _, declaration := range file.Decls {
		general, ok := declaration.(*ast.GenDecl)
		if !ok || general.Tok != token.CONST {
			continue
		}
		for _, specification := range general.Specs {
			values, ok := specification.(*ast.ValueSpec)
			if !ok || len(values.Names) != len(values.Values) {
				continue
			}
			for index, name := range values.Names {
				literal, ok := values.Values[index].(*ast.BasicLit)
				if !ok || literal.Kind != token.STRING {
					continue
				}
				value, unquoteErr := strconv.Unquote(literal.Value)
				if unquoteErr != nil {
					t.Fatalf("unquote %s: %v", name.Name, unquoteErr)
				}
				if _, duplicate := constants[name.Name]; duplicate {
					t.Fatalf("duplicate permission constant %s", name.Name)
				}
				constants[name.Name] = value
			}
		}
	}
	if len(constants) == 0 {
		t.Fatal(fmt.Errorf("no permission constants found in %s", filename))
	}
	return constants
}

func selectorName(expression ast.Expr) (string, bool) {
	selector, ok := expression.(*ast.SelectorExpr)
	if !ok || selector.Sel == nil {
		return "", false
	}
	return selector.Sel.Name, true
}
