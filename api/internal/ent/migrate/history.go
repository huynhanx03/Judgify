package migration

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	atlasmigrate "ariga.io/atlas/sql/migrate"
)

// ErrHistoryDrift means the database revision ledger no longer describes the
// immutable, checksummed migration chain embedded in this operator artifact.
// This is deliberately never repaired at runtime: an operator must inspect and
// reconcile the database before another DDL statement can execute.
var ErrHistoryDrift = errors.New("migration: revision history drift")

// validateRevisionHistory closes safety gaps that are intentionally outside
// Atlas' Pending contract. In particular, Atlas validates atlas.sum and
// partially-applied statement hashes, but a fully-applied database revision
// must also be compared with the embedded file hash before execution resumes.
func validateRevisionHistory(
	ctx context.Context,
	directory atlasmigrate.Dir,
	revisions []*atlasmigrate.Revision,
	loadStatements func(atlasmigrate.File) ([]string, error),
) error {
	if ctx == nil || directory == nil || loadStatements == nil {
		return ErrInvalidConfiguration
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(revisions) > maxMigrationFiles {
		return historyDrift("revision count exceeds %d", maxMigrationFiles)
	}
	files, err := directory.Files()
	if err != nil {
		return fmt.Errorf("migration: list files for history validation: %w", err)
	}
	checksum, err := directory.Checksum()
	if err != nil {
		return fmt.Errorf("migration: checksum files for history validation: %w", err)
	}
	fileIndex := make(map[string]int, len(files))
	for index, file := range files {
		fileIndex[file.Version()] = index
	}

	previousFileIndex := -1
	for index, revision := range revisions {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := validateRevision(revision); err != nil {
			return historyDrift("invalid revision at position %d: %v", index+1, err)
		}
		position, exists := fileIndex[revision.Version]
		if !exists {
			return historyDrift("revision %q has no embedded migration file", revision.Version)
		}
		file := files[position]
		if revision.Description != file.Desc() {
			return historyDrift("revision %q description does not match %q", revision.Version, file.Name())
		}
		if index > 0 && revision.Version <= revisions[index-1].Version {
			return historyDrift("revision versions are not strictly increasing at %q", revision.Version)
		}

		switch revision.Type {
		case atlasmigrate.RevisionTypeBaseline:
			if index != 0 || revision.Applied != 0 || revision.Total != 0 ||
				revision.Hash != "" || len(revision.PartialHashes) != 0 ||
				revision.Error != "" || revision.ErrorStmt != "" {
				return historyDrift("revision %q is an invalid baseline", revision.Version)
			}
		case atlasmigrate.RevisionTypeExecute:
			if err := validateExecutedRevision(file, checksum, revision, loadStatements); err != nil {
				return err
			}
		default:
			return historyDrift("revision %q has unsupported type %s", revision.Version, revision.Type)
		}

		if index == 0 && revision.Type != atlasmigrate.RevisionTypeBaseline && position != 0 {
			return historyDrift("revision history starts at %q instead of the first migration", revision.Version)
		}
		if previousFileIndex >= 0 && position != previousFileIndex+1 {
			return historyDrift("revision history is non-linear before %q", revision.Version)
		}
		if revision.Applied != revision.Total && index != len(revisions)-1 {
			return historyDrift("partially applied revision %q is not the latest revision", revision.Version)
		}
		previousFileIndex = position
	}
	return nil
}

func validateExecutedRevision(
	file atlasmigrate.File,
	checksum atlasmigrate.HashFile,
	revision *atlasmigrate.Revision,
	loadStatements func(atlasmigrate.File) ([]string, error),
) error {
	statements, err := loadStatements(file)
	if err != nil {
		return redactedCause(
			fmt.Sprintf("migration: parse %q for history validation: SQL content redacted", file.Name()),
			err,
		)
	}
	if revision.Total != len(statements) {
		return historyDrift(
			"revision %q statement total is %d; embedded file has %d",
			revision.Version,
			revision.Total,
			len(statements),
		)
	}
	expectedHash, err := checksum.SumByName(file.Name())
	if err != nil {
		return fmt.Errorf("migration: read checksum for %q: %w", file.Name(), err)
	}
	if revision.Hash == "" || revision.Hash != expectedHash {
		return historyDrift("revision %q file checksum does not match the embedded chain", revision.Version)
	}

	partial := revision.Applied != revision.Total
	if !partial {
		if len(revision.PartialHashes) != 0 {
			return historyDrift("completed revision %q retains partial hashes", revision.Version)
		}
		if revision.Error != "" || revision.ErrorStmt != "" {
			return historyDrift("completed revision %q retains failure metadata", revision.Version)
		}
		return nil
	}
	if len(revision.PartialHashes) != revision.Applied {
		return historyDrift(
			"revision %q has %d applied statements but %d partial hashes",
			revision.Version,
			revision.Applied,
			len(revision.PartialHashes),
		)
	}
	hasher := sha256.New()
	for index := 0; index < revision.Applied; index++ {
		_, _ = hasher.Write([]byte(statements[index]))
		expected := "h1:" + base64.StdEncoding.EncodeToString(hasher.Sum(nil))
		if revision.PartialHashes[index] != expected {
			return historyDrift(
				"revision %q applied statement %d checksum does not match",
				revision.Version,
				index+1,
			)
		}
	}
	return nil
}

func historyDrift(format string, args ...any) error {
	message := fmt.Sprintf(format, args...)
	message = strings.ReplaceAll(message, "\n", " ")
	message = strings.ReplaceAll(message, "\r", " ")
	return fmt.Errorf("%w: %s", ErrHistoryDrift, message)
}
