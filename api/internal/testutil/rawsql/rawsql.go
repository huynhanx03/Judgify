// Package rawsql collects the raw SQL a package hands to database/sql so tests
// can prepare each statement against a real PostgreSQL.
//
// PostgreSQL resolves parameter types when a statement is prepared, so a query
// that deduces two different types for one placeholder fails with 42P08 before
// a single row is touched. sqlmock never runs the planner, so unit tests cannot
// see that class of defect at all: it reached this repository and broke every
// contest lifecycle transition against a real database.
//
// Preparing every raw statement needs no fixture data, so the guard stays cheap
// while covering each statement that exists now or is added later.
package rawsql

import (
	"context"
	"database/sql"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

// DSNEnvironment names the variable holding the PostgreSQL DSN the guard
// prepares against. The guard skips when it is unset.
const DSNEnvironment = "JUDGIFY_TEST_DATABASE_URL"

// Statement is a SQL string the package hands to a query method.
type Statement struct {
	Name string
	SQL  string
}

// Site is a query call whose SQL argument is built at run time and so cannot be
// recovered by reading the source. Reporting these keeps the guard honest about
// what it does not cover.
type Site struct {
	Name string
	Why  string
}

// Collection is everything the collector learned about one package.
type Collection struct {
	Statements []Statement
	Unresolved []Site
}

// CallSites is the number of query calls found, resolved or not.
func (collection Collection) CallSites() int {
	return len(collection.Statements) + len(collection.Unresolved)
}

// queryMethods are the database/sql methods that take SQL as their second
// argument. Ent's generated clients expose the same shape.
var queryMethods = map[string]bool{
	"QueryContext":    true,
	"ExecContext":     true,
	"QueryRowContext": true,
}

// Collect reads the non-test Go sources in dir and recovers every SQL string
// they hand to a query method.
//
// Three forms reach a query call, and all three must be followed or the guard
// silently under-reports: a literal at the call site, a package-level constant
// referenced by name, and a constant concatenated with a literal tail. A fourth
// form matters just as much: packages wrap QueryContext in a local helper that
// forwards a string parameter, which moves the literal to the helper's callers.
// Those wrappers are detected here so their call sites are collected too.
func Collect(t *testing.T, dir string) Collection {
	t.Helper()

	entries, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		t.Fatalf("rawsql: glob %s: %v", dir, err)
	}
	sort.Strings(entries)

	fileSet := token.NewFileSet()
	files := make([]*ast.File, 0, len(entries))
	for _, entry := range entries {
		if strings.HasSuffix(entry, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fileSet, entry, nil, 0)
		if err != nil {
			t.Fatalf("rawsql: parse %s: %v", entry, err)
		}
		files = append(files, file)
	}

	constants := stringConstants(files)
	forwarders := sqlForwarders(files)

	collection := Collection{}
	for _, file := range files {
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			index, ok := sqlArgumentIndex(call, forwarders)
			if !ok || index >= len(call.Args) {
				return true
			}
			name := position(fileSet, call.Args[index].Pos())
			text, ok := resolve(call.Args[index], constants)
			if !ok {
				collection.Unresolved = append(collection.Unresolved, Site{
					Name: name,
					Why:  describe(call.Args[index]),
				})
				return true
			}
			if strings.TrimSpace(text) == "" {
				return true
			}
			collection.Statements = append(collection.Statements, Statement{Name: name, SQL: text})
			return true
		})
	}
	return collection
}

// Statements returns only the SQL the collector could recover.
func Statements(t *testing.T, dir string) []Statement {
	t.Helper()
	return Collect(t, dir).Statements
}

// PrepareAll prepares every recoverable statement in dir against dsn. It skips
// when dsn is empty and fails when the collector finds nothing, which would
// mean the collector, not the package, had stopped working.
func PrepareAll(t *testing.T, dsn, dir string) {
	t.Helper()

	if dsn == "" {
		t.Skipf("%s is not configured", DSNEnvironment)
	}

	collection := Collect(t, dir)
	if len(collection.Statements) == 0 {
		t.Fatal("no raw SQL statements were discovered; the collector is broken")
	}
	for _, site := range collection.Unresolved {
		t.Logf("rawsql: %s builds its SQL at run time (%s); not covered", site.Name, site.Why)
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

	for _, statement := range collection.Statements {
		t.Run(statement.Name, func(t *testing.T) {
			prepared, err := database.PrepareContext(ctx, statement.SQL)
			if err != nil {
				t.Fatalf("%s does not prepare on PostgreSQL: %v", statement.Name, err)
			}
			_ = prepared.Close()
		})
	}
}

// sqlArgumentIndex reports which argument of call carries SQL, if any.
func sqlArgumentIndex(call *ast.CallExpr, forwarders map[string]int) (int, bool) {
	switch fun := call.Fun.(type) {
	case *ast.SelectorExpr:
		if queryMethods[fun.Sel.Name] && len(call.Args) >= 2 {
			return 1, true
		}
	case *ast.Ident:
		if index, ok := forwarders[fun.Name]; ok && len(call.Args) > index {
			return index, true
		}
	}
	return 0, false
}

// sqlForwarders finds package functions that pass one of their own string
// parameters straight to a query method, and records which parameter it is.
func sqlForwarders(files []*ast.File) map[string]int {
	forwarders := map[string]int{}
	for _, file := range files {
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Recv != nil || function.Body == nil {
				continue
			}
			names := parameterNames(function.Type.Params)
			ast.Inspect(function.Body, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok || len(call.Args) < 2 {
					return true
				}
				selector, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || !queryMethods[selector.Sel.Name] {
					return true
				}
				identifier, ok := call.Args[1].(*ast.Ident)
				if !ok {
					return true
				}
				if index, ok := names[identifier.Name]; ok {
					forwarders[function.Name.Name] = index
				}
				return true
			})
		}
	}
	return forwarders
}

// parameterNames maps each string parameter's name to its argument position.
func parameterNames(params *ast.FieldList) map[string]int {
	names := map[string]int{}
	if params == nil {
		return names
	}
	index := 0
	for _, field := range params.List {
		identifier, isString := field.Type.(*ast.Ident)
		for _, name := range field.Names {
			if isString && identifier.Name == "string" {
				names[name.Name] = index
			}
			index++
		}
		if len(field.Names) == 0 {
			index++
		}
	}
	return names
}

// stringConstants maps package-level string constant and variable names to
// their literal values.
func stringConstants(files []*ast.File) map[string]string {
	constants := map[string]string{}
	for _, file := range files {
		for _, declaration := range file.Decls {
			general, ok := declaration.(*ast.GenDecl)
			if !ok || (general.Tok != token.CONST && general.Tok != token.VAR) {
				continue
			}
			for _, spec := range general.Specs {
				value, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for position, name := range value.Names {
					if position >= len(value.Values) {
						continue
					}
					if text, ok := resolve(value.Values[position], constants); ok {
						constants[name.Name] = text
					}
				}
			}
		}
	}
	return constants
}

// resolve recovers the compile-time value of a SQL expression, following
// constant references and concatenation.
func resolve(expression ast.Expr, constants map[string]string) (string, bool) {
	switch node := expression.(type) {
	case *ast.BasicLit:
		if node.Kind != token.STRING {
			return "", false
		}
		text, err := strconv.Unquote(node.Value)
		if err != nil {
			return "", false
		}
		return text, true
	case *ast.Ident:
		text, ok := constants[node.Name]
		return text, ok
	case *ast.BinaryExpr:
		if node.Op != token.ADD {
			return "", false
		}
		left, ok := resolve(node.X, constants)
		if !ok {
			return "", false
		}
		right, ok := resolve(node.Y, constants)
		if !ok {
			return "", false
		}
		return left + right, true
	case *ast.ParenExpr:
		return resolve(node.X, constants)
	}
	return "", false
}

// describe names the shape of an unresolvable SQL argument.
func describe(expression ast.Expr) string {
	switch node := expression.(type) {
	case *ast.CallExpr:
		if selector, ok := node.Fun.(*ast.SelectorExpr); ok {
			return selector.Sel.Name + "()"
		}
		return "call"
	case *ast.Ident:
		return "local variable " + node.Name
	case *ast.BinaryExpr:
		return "concatenation with a run-time operand"
	}
	return "dynamic expression"
}

func position(fileSet *token.FileSet, pos token.Pos) string {
	at := fileSet.Position(pos)
	return filepath.Base(at.Filename) + ":" + strconv.Itoa(at.Line)
}
