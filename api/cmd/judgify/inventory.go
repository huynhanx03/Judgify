package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/huynhanx03/judgify/internal/ent/migrate/inventory"
)

func runInventory(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("judgify inventory", flag.ContinueOnError)
	flags.SetOutput(stderr)
	format := flags.String("format", "markdown", "output format: markdown or json")
	migrations := flags.String(
		"migrations",
		"internal/ent/migrate/migrations",
		"path to the migration directory",
	)
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		_, _ = fmt.Fprintf(stderr, "judgify inventory: unexpected arguments: %s\n", strings.Join(flags.Args(), " "))
		return 2
	}
	if *format != "markdown" && *format != "json" {
		_, _ = fmt.Fprintf(stderr, "judgify inventory: unsupported format %q\n", *format)
		return 2
	}

	report, err := inventory.Inspect(ctx, os.DirFS(*migrations), inventory.CurrentSchemas())
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "judgify inventory: %v\n", err)
		return 2
	}

	switch *format {
	case "json":
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(report); err != nil {
			_, _ = fmt.Fprintf(stderr, "judgify inventory: encode JSON: %v\n", err)
			return 2
		}
	case "markdown":
		if err := inventory.WriteMarkdown(stdout, report); err != nil {
			_, _ = fmt.Fprintf(stderr, "judgify inventory: encode markdown: %v\n", err)
			return 2
		}
	}

	if err := report.Validate(); err != nil {
		_, _ = fmt.Fprintf(stderr, "judgify inventory: %v\n", err)
		return 1
	}
	return 0
}
