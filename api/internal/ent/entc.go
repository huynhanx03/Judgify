//go:build ignore

package main

import (
	"log"

	"entgo.io/ent/entc"
	"entgo.io/ent/entc/gen"
)

func main() {
	opts := []entc.Option{
		entc.TemplateDir("./template"),
	}

	if err := entc.Generate("./schema", &gen.Config{
		Target:  "./generate",
		Package: "github.com/huynhanx03/judgify/internal/ent/generate",
		Features: []gen.Feature{
			gen.FeatureIntercept,
			gen.FeatureUpsert,
			gen.FeatureExecQuery,
			gen.FeatureVersionedMigration,
			gen.FeatureModifier,
			gen.FeatureLock,
			gen.FeatureBidiEdgeRefs,
		},
	}, opts...); err != nil {
		log.Fatalf("running ent codegen: %v", err)
	}
}
