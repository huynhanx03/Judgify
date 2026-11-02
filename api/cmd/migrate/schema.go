package main

import (
	"context"
	"fmt"
	"io"

	"github.com/huynhanx03/judgify/internal/ent/desiredschema"
)

func writeAtlasSchema(ctx context.Context, output io.Writer) error {
	state, err := desiredschema.Load(ctx)
	if err != nil {
		return err
	}
	ddl, err := state.RelationalDDL(ctx)
	if err != nil {
		return err
	}
	if _, err := io.WriteString(output, ddl); err != nil {
		return fmt.Errorf("write desired schema DDL: %w", err)
	}
	return nil
}
