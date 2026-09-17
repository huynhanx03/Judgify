package schemaextras_test

import (
	"reflect"
	"slices"
	"sort"
	"testing"

	"entgo.io/ent/entc"
	"entgo.io/ent/entc/gen"

	commonent "github.com/huynhanx03/go-common/pkg/database/ent"

	"github.com/huynhanx03/judgify/internal/ent/migrate/inventory"
	"github.com/huynhanx03/judgify/internal/ent/schemaextras"
)

func TestAppendOnlyTableCatalogMatchesEntSchemaMixins(t *testing.T) {
	t.Chdir("../../..")

	graph, err := entc.LoadGraph("./internal/ent/schema", &gen.Config{})
	if err != nil {
		t.Fatalf("load Ent schema graph: %v", err)
	}

	nodesBySchema := make(map[string]*gen.Type, len(graph.Nodes))
	for _, node := range graph.Nodes {
		if node == nil {
			t.Fatal("Ent schema graph contains a nil node")
		}
		if previous, duplicate := nodesBySchema[node.Name]; duplicate {
			t.Fatalf("Ent schema graph contains duplicate node %q: %#v and %#v", node.Name, previous, node)
		}
		nodesBySchema[node.Name] = node
	}

	appendOnlyMixinType := concreteType(commonent.AppendOnlyMixin{})
	derivedTables := make([]string, 0)
	tableOwners := make(map[string]string)
	for _, currentSchema := range inventory.CurrentSchemas() {
		schemaType := concreteType(currentSchema)
		if schemaType == nil || schemaType.Name() == "" {
			t.Fatalf("CurrentSchemas() contains an unnamed schema %T", currentSchema)
		}
		schemaName := schemaType.Name()
		node, exists := nodesBySchema[schemaName]
		if !exists {
			t.Fatalf(
				"CurrentSchemas() contains duplicate schema %q or one missing from the Ent graph",
				schemaName,
			)
		}
		delete(nodesBySchema, schemaName)

		appendOnlyMixins := 0
		for _, schemaMixin := range currentSchema.Mixin() {
			if concreteType(schemaMixin) == appendOnlyMixinType {
				appendOnlyMixins++
			}
		}
		if appendOnlyMixins > 1 {
			t.Fatalf("schema %q declares AppendOnlyMixin %d times", schemaName, appendOnlyMixins)
		}
		if appendOnlyMixins == 0 {
			continue
		}

		table := node.Table()
		if table == "" {
			t.Fatalf("append-only schema %q resolves to an empty table name", schemaName)
		}
		if previous, duplicate := tableOwners[table]; duplicate {
			t.Fatalf(
				"append-only schemas %q and %q both resolve to table %q",
				previous,
				schemaName,
				table,
			)
		}
		tableOwners[table] = schemaName
		derivedTables = append(derivedTables, table)
	}
	if len(nodesBySchema) != 0 {
		missingSchemas := make([]string, 0, len(nodesBySchema))
		for schemaName := range nodesBySchema {
			missingSchemas = append(missingSchemas, schemaName)
		}
		sort.Strings(missingSchemas)
		t.Fatalf("CurrentSchemas() is missing Ent graph schemas: %v", missingSchemas)
	}

	sort.Strings(derivedTables)
	catalogTables := schemaextras.AppendOnlyTables()
	sort.Strings(catalogTables)
	if !slices.Equal(derivedTables, catalogTables) {
		t.Fatalf(
			"append-only tables derived from Ent schemas = %v, catalog = %v",
			derivedTables,
			catalogTables,
		)
	}
}

func concreteType(value any) reflect.Type {
	valueType := reflect.TypeOf(value)
	for valueType != nil && valueType.Kind() == reflect.Pointer {
		valueType = valueType.Elem()
	}
	return valueType
}
