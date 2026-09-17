// Command atlas-migration-footer emits the deterministic PostgreSQL objects
// that Atlas OSS cannot represent in its relational desired-state diff.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/huynhanx03/judgify/internal/ent/schemaextras"
)

func main() {
	schema := flag.String("schema", "public", "PostgreSQL schema containing Judgify tables")
	appendTo := flag.String("append-to", "", "append the footer to a generated migration instead of stdout")
	flag.Parse()

	ddl, err := schemaextras.JudgifyMigrationFooterDDL(*schema)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "atlas-migration-footer: %v\n", err)
		os.Exit(1)
	}
	output := os.Stdout
	if *appendTo != "" {
		file, openErr := os.OpenFile(*appendTo, os.O_APPEND|os.O_WRONLY, 0)
		if openErr != nil {
			_, _ = fmt.Fprintf(os.Stderr, "atlas-migration-footer: open migration: %v\n", openErr)
			os.Exit(1)
		}
		defer func() {
			if closeErr := file.Close(); closeErr != nil {
				_, _ = fmt.Fprintf(os.Stderr, "atlas-migration-footer: close migration: %v\n", closeErr)
				os.Exit(1)
			}
		}()
		if _, writeErr := fmt.Fprintln(file, "\n"+ddl); writeErr != nil {
			_, _ = fmt.Fprintf(os.Stderr, "atlas-migration-footer: append DDL: %v\n", writeErr)
			os.Exit(1)
		}
		return
	}
	if _, err := fmt.Fprintln(output, ddl); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "atlas-migration-footer: write DDL: %v\n", err)
		os.Exit(1)
	}
}
