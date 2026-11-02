package architecture_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/huynhanx03/judgify/internal/ent/schemaextras"
)

// rowLockViolation records a single site where a row lock targets an
// append-only table.
type rowLockViolation struct {
	location string
	clause   string
	table    string
}

// TestNoRowLockOnAppendOnlyTable fails if any raw SQL statement in the
// internal/ tree takes a row lock (FOR UPDATE, FOR NO KEY UPDATE, FOR SHARE,
// FOR KEY SHARE) on an append-only table.
//
// Append-only rows can never be updated or deleted — row locks on them are both
// privileged (PostgreSQL requires UPDATE for every locking clause) and
// semantically vacuous. Runtime roles do not hold UPDATE on these tables, so
// the statement fails in production even though PREPARE succeeds and sqlmock
// never checks privileges.
func TestNoRowLockOnAppendOnlyTable(t *testing.T) {
	appendOnly := make(map[string]struct{})
	for _, table := range schemaextras.AppendOnlyTables() {
		appendOnly[table] = struct{}{}
	}
	if len(appendOnly) == 0 {
		t.Fatal("schemaextras.AppendOnlyTables() returned nothing; the catalog is broken")
	}

	root := moduleRoot(t)
	internalDir := filepath.Join(root, "internal")
	generateDir := filepath.Join(internalDir, "ent", "generate")

	var violations []rowLockViolation
	statementsScanned := 0

	err := filepath.WalkDir(internalDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if strings.HasPrefix(path, generateDir) {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}

		// Collect package-level string constants so we can resolve named SQL
		// references and handle the constant+literal concatenation pattern used
		// throughout the codebase.
		pkgConstants := fileStringConstants(file)

		ast.Inspect(file, func(node ast.Node) bool {
			lit, ok := node.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			text, err := strconv.Unquote(lit.Value)
			if err != nil {
				return true
			}
			if !rowLockPattern.MatchString(text) {
				return true
			}

			pos := fset.Position(lit.Pos())
			location := filepath.Base(pos.Filename) + ":" + strconv.Itoa(pos.Line)
			statementsScanned++
			checkRowLock(text, location, appendOnly, &violations)
			return true
		})

		// Also walk string constant declarations at the file level in case a
		// constant contains a lock keyword — the Inspect above catches them
		// already as BasicLit nodes, but we also look for any += append
		// pattern such as `statement += " FOR SHARE OF t"`.
		for _, decl := range file.Decls {
			funcDecl, ok := decl.(*ast.FuncDecl)
			if !ok || funcDecl.Body == nil {
				continue
			}
			ast.Inspect(funcDecl.Body, func(node ast.Node) bool {
				assign, ok := node.(*ast.AssignStmt)
				if !ok || assign.Tok.String() != "+=" {
					return true
				}
				for _, rhs := range assign.Rhs {
					text, ok := resolveStringExpr(rhs, pkgConstants)
					if !ok || !rowLockPattern.MatchString(text) {
						continue
					}
					pos := fset.Position(assign.Pos())
					location := filepath.Base(pos.Filename) + ":" + strconv.Itoa(pos.Line)
					statementsScanned++
					// For an appended fragment, the fragment IS the locking
					// clause. Parse it directly.
					checkRowLock(text, location, appendOnly, &violations)
				}
				return true
			})
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk internal sources: %v", err)
	}

	if statementsScanned == 0 {
		t.Fatal("no SQL statements with row-locking clauses were found; the collector is broken")
	}

	for _, v := range violations {
		t.Errorf("%s: %q locks append-only table %q — remove the row lock (runtime roles lack UPDATE privilege)",
			v.location, v.clause, v.table)
	}
}

// rowLockPattern matches any locking-read keyword sequence.
var rowLockPattern = regexp.MustCompile(`(?i)\bFOR\s+(UPDATE|SHARE|NO\s+KEY\s+UPDATE|KEY\s+SHARE)\b`)

// forClausePattern matches the full FOR ... [OF ...] [SKIP LOCKED|NOWAIT] tail.
// Group 1 = lock type words ("UPDATE", "SHARE", "NO KEY UPDATE", "KEY SHARE").
// Group 2 = OF list (may be empty/absent).
var forClausePattern = regexp.MustCompile(`(?i)\bFOR\s+((?:NO\s+KEY\s+)?UPDATE|(?:KEY\s+)?SHARE)\s*(?:OF\s+([\w\s,]+?))?(?:\s+(?:SKIP\s+LOCKED|NOWAIT))?(?:\s*$|\s*;|\s+LIMIT|\s+OFFSET|\s+\z)`)

// fromAliasPattern matches FROM/JOIN table references and extracts the table
// name and optional alias.
//
// It handles:
//
//	FROM tablename
//	FROM tablename alias
//	FROM tablename AS alias
//	JOIN tablename alias / JOIN tablename AS alias
//
// The negative lookahead for SQL keywords (ON, WHERE, USING) is emulated in
// code because Go's RE2 engine does not support lookaheads.
var fromAliasPattern = regexp.MustCompile(`(?i)\b(?:FROM|JOIN)\s+(\w+)(?:\s+AS\s+(\w+)|\s+(\w+))?`)

// sqlKeywords is the set of words that follow a table name in a FROM/JOIN
// clause and are not aliases (e.g. ON, WHERE, USING, LEFT, RIGHT, INNER,
// OUTER, LATERAL, SET, LIMIT, ORDER).
var sqlKeywords = map[string]struct{}{
	"on": {}, "where": {}, "using": {}, "left": {}, "right": {},
	"inner": {}, "outer": {}, "cross": {}, "lateral": {}, "set": {},
	"limit": {}, "order": {}, "group": {}, "having": {}, "union": {},
	"and": {}, "or": {}, "not": {}, "in": {}, "as": {}, "true": {},
	"false": {}, "null": {}, "skip": {}, "locked": {}, "nowait": {},
	"for": {}, "with": {}, "join": {}, "from": {},
}

// checkRowLock parses sql for FOR ... [OF ...] clauses and records a violation
// for each locking clause that targets an append-only table.
func checkRowLock(sql, location string, appendOnly map[string]struct{}, violations *[]rowLockViolation) {
	// Build alias→table map from FROM/JOIN clauses.
	aliasToTable := make(map[string]string)
	for _, m := range fromAliasPattern.FindAllStringSubmatch(sql, -1) {
		table := strings.ToLower(m[1])
		alias := ""
		switch {
		case m[2] != "":
			// AS alias form — always valid.
			alias = strings.ToLower(m[2])
		case m[3] != "":
			// Bare word after table name — only treat as alias if it is not a
			// SQL keyword (Go's RE2 does not support negative lookahead).
			candidate := strings.ToLower(m[3])
			if _, isKeyword := sqlKeywords[candidate]; !isKeyword {
				alias = candidate
			}
		}
		if alias == "" {
			alias = table
		}
		aliasToTable[alias] = table
		// Also index the bare table name → itself so a bare FOR UPDATE on a
		// query without aliases resolves correctly.
		aliasToTable[table] = table
	}

	for _, m := range forClausePattern.FindAllStringSubmatch(sql, -1) {
		clause := strings.TrimSpace(m[0])
		// Keep the clause short for the error message.
		if len(clause) > 60 {
			clause = clause[:60] + "..."
		}

		ofList := strings.TrimSpace(m[2])

		if ofList == "" {
			// Bare FOR ... — locks every table reachable via aliasToTable.
			for alias, table := range aliasToTable {
				if _, bad := appendOnly[table]; bad {
					// Avoid double-reporting when alias == table.
					if alias != table {
						continue
					}
					*violations = append(*violations, rowLockViolation{
						location: location,
						clause:   clause,
						table:    table,
					})
				}
			}
			// If aliasToTable is empty (e.g. a bare fragment "+= FOR UPDATE"),
			// we cannot resolve the target; do not report a false positive.
		} else {
			// FOR ... OF a, b — locks only the named aliases.
			for _, raw := range strings.Split(ofList, ",") {
				alias := strings.TrimSpace(strings.ToLower(raw))
				table, known := aliasToTable[alias]
				if !known {
					// Fall back: treat the identifier itself as a table name.
					table = alias
				}
				if _, bad := appendOnly[table]; bad {
					*violations = append(*violations, rowLockViolation{
						location: location,
						clause:   clause,
						table:    table,
					})
				}
			}
		}
	}
}

// fileStringConstants returns a map of package-level string constant names to
// their literal values for one parsed file.
func fileStringConstants(file *ast.File) map[string]string {
	constants := make(map[string]string)
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || (gen.Tok != token.CONST && gen.Tok != token.VAR) {
			continue
		}
		for _, spec := range gen.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for i, name := range vs.Names {
				if i >= len(vs.Values) {
					continue
				}
				if text, ok := resolveStringExpr(vs.Values[i], constants); ok {
					constants[name.Name] = text
				}
			}
		}
	}
	return constants
}

// resolveStringExpr evaluates a Go AST expression to a string constant value,
// following identifiers in the supplied constants map and concatenation.
func resolveStringExpr(expr ast.Expr, constants map[string]string) (string, bool) {
	switch node := expr.(type) {
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
		left, ok := resolveStringExpr(node.X, constants)
		if !ok {
			return "", false
		}
		right, ok := resolveStringExpr(node.Y, constants)
		if !ok {
			return "", false
		}
		return left + right, true
	case *ast.ParenExpr:
		return resolveStringExpr(node.X, constants)
	}
	return "", false
}
