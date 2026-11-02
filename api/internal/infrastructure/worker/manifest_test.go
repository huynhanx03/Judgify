package worker

import (
	"errors"
	"testing"
)

func TestDecodeApprovedReleaseManifestRejectsUnknownAndTrailingJSON(t *testing.T) {
	t.Parallel()
	for name, document := range map[string]string{
		"unknown field":  `{"release_id":"judge-release.2026.07.29.1","unexpected":true}`,
		"trailing value": `{"release_id":"judge-release.2026.07.29.1"}{}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := decodeApprovedReleaseManifest([]byte(document)); !errors.Is(err, ErrInvalidWorker) {
				t.Fatalf("decodeApprovedReleaseManifest() error = %v, want %v", err, ErrInvalidWorker)
			}
		})
	}
}

func TestReleaseManifestPathIsAbsoluteRegularFile(t *testing.T) {
	t.Parallel()
	if err := validateReleaseManifestPath("relative/release.json"); !errors.Is(err, ErrInvalidWorker) {
		t.Fatalf("validateReleaseManifestPath() error = %v, want %v", err, ErrInvalidWorker)
	}
}
