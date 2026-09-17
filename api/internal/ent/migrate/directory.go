package migration

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	atlasmigrate "ariga.io/atlas/sql/migrate"
)

const (
	maxMigrationFiles        = 1_000
	maxMigrationFileBytes    = 8 << 20
	maxMigrationTotalBytes   = 64 << 20
	maxMigrationStatements   = 50_000
	maxMigrationChecksumSize = 1 << 20
)

var (
	// Embedded migrations make the operator artifact self-contained. The
	// checked-in atlas.sum remains authoritative and is validated before any
	// database connection is opened.
	//
	//go:embed migrations/*.sql migrations/atlas.sum
	embeddedMigrations embed.FS

	errNoMigrationFiles = errors.New("migration: no checked-in SQL files")
)

func loadEmbeddedDirectory(ctx context.Context) (*atlasmigrate.MemDir, error) {
	source, err := fs.Sub(embeddedMigrations, "migrations")
	if err != nil {
		return nil, fmt.Errorf("migration: open embedded directory: %w", err)
	}
	return loadDirectory(ctx, source)
}

func loadDirectory(ctx context.Context, source fs.FS) (*atlasmigrate.MemDir, error) {
	if ctx == nil || source == nil {
		return nil, ErrInvalidConfiguration
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	entries, err := fs.ReadDir(source, ".")
	if err != nil {
		return nil, fmt.Errorf("migration: read checked-in directory: %w", err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	directory := &atlasmigrate.MemDir{}
	versions := make(map[string]string, len(entries))
	fileCount := 0
	totalBytes := 0
	hasChecksum := false
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if entry.IsDir() {
			return nil, fmt.Errorf("migration: nested directory %q is not allowed", entry.Name())
		}
		name := entry.Name()
		if filepath.Base(name) != name || !fs.ValidPath(name) {
			return nil, fmt.Errorf("migration: invalid file name %q", name)
		}

		limit := maxMigrationFileBytes
		switch {
		case name == atlasmigrate.HashFileName:
			hasChecksum = true
			limit = maxMigrationChecksumSize
		case filepath.Ext(name) == ".sql":
			fileCount++
			if fileCount > maxMigrationFiles {
				return nil, fmt.Errorf("migration: SQL file count exceeds %d", maxMigrationFiles)
			}
		default:
			return nil, fmt.Errorf("migration: unsupported checked-in file %q", name)
		}

		data, err := fs.ReadFile(source, name)
		if err != nil {
			return nil, fmt.Errorf("migration: read %q: %w", name, err)
		}
		if len(data) > limit {
			return nil, fmt.Errorf("migration: %q exceeds %d bytes", name, limit)
		}
		totalBytes += len(data)
		if totalBytes > maxMigrationTotalBytes {
			return nil, fmt.Errorf("migration: checked-in directory exceeds %d bytes", maxMigrationTotalBytes)
		}
		if err := directory.WriteFile(name, data); err != nil {
			return nil, fmt.Errorf("migration: stage %q: %w", name, err)
		}
	}

	if fileCount == 0 {
		return nil, errNoMigrationFiles
	}
	if !hasChecksum {
		return nil, atlasmigrate.ErrChecksumNotFound
	}
	if err := atlasmigrate.Validate(directory); err != nil {
		return nil, fmt.Errorf("migration: validate atlas.sum: %w", err)
	}

	files, err := directory.Files()
	if err != nil {
		return nil, fmt.Errorf("migration: list checked-in SQL: %w", err)
	}
	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		version := file.Version()
		if err := atlasmigrate.CheckVersion(version); err != nil {
			return nil, fmt.Errorf("migration: invalid version in %q: %w", file.Name(), err)
		}
		if checkpoint, ok := file.(atlasmigrate.CheckpointFile); ok && checkpoint.IsCheckpoint() {
			return nil, fmt.Errorf(
				"migration: checkpoint file %q is not allowed in the immutable linear chain",
				file.Name(),
			)
		}
		if directives, ok := file.(interface{ Directive(string) []string }); ok {
			modes := directives.Directive("txmode")
			if len(modes) > 1 || len(modes) == 1 && strings.TrimSpace(modes[0]) != "file" {
				return nil, fmt.Errorf(
					"migration: %q requests unsupported transaction mode %q; only per-file transactions are allowed",
					file.Name(),
					strings.Join(modes, ","),
				)
			}
		}
		if previous, exists := versions[version]; exists {
			return nil, fmt.Errorf(
				"migration: duplicate version %q in %q and %q",
				version,
				previous,
				file.Name(),
			)
		}
		versions[version] = file.Name()
		statements, err := file.Stmts()
		if err != nil {
			return nil, redactedCause(
				fmt.Sprintf("migration: parse %q: SQL content redacted", file.Name()),
				err,
			)
		}
		if len(statements) > maxMigrationStatements {
			return nil, fmt.Errorf(
				"migration: %q has %d statements; limit is %d",
				file.Name(),
				len(statements),
				maxMigrationStatements,
			)
		}
	}
	return directory, nil
}

func loadDriverStatements(
	ctx context.Context,
	directory atlasmigrate.Dir,
	driver atlasmigrate.Driver,
) (map[string][]string, error) {
	if ctx == nil || directory == nil || driver == nil {
		return nil, ErrInvalidConfiguration
	}
	files, err := directory.Files()
	if err != nil {
		return nil, fmt.Errorf("migration: list SQL for PostgreSQL validation: %w", err)
	}
	statements := make(map[string][]string, len(files))
	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		parsed, err := atlasmigrate.FileStmts(driver, file)
		if err != nil {
			return nil, redactedCause(
				fmt.Sprintf("migration: PostgreSQL parse %q: SQL content redacted", file.Name()),
				err,
			)
		}
		if len(parsed) > maxMigrationStatements {
			return nil, fmt.Errorf(
				"migration: %q has %d PostgreSQL statements; limit is %d",
				file.Name(),
				len(parsed),
				maxMigrationStatements,
			)
		}
		statements[file.Name()] = parsed
	}
	return statements, nil
}
