package executor

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"

	problementity "github.com/huynhanx03/judgify/internal/problem/domain"
)

func TestStagingPublishesVerifiedPrivateArtifactsAndReleasesQuota(t *testing.T) {
	t.Parallel()

	stager := newTestStager(t, 1<<20)
	attemptID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	profile := testArtifactProfile()
	artifact, err := stager.Stage(context.Background(), StageRequest{
		AttemptID: attemptID,
		Archive: artifactArchive(t, []artifactArchiveEntry{
			{name: "artifact/", mode: 0o700, kind: tar.TypeDir},
			{name: "artifact/main", mode: 0o755, kind: tar.TypeReg, body: []byte("ELF")},
		}),
		RuntimeProfile:   profile,
		ManifestChecksum: profile.ManifestChecksum,
	})
	if err != nil {
		t.Fatalf("Stage() error = %v", err)
	}
	if artifact.ContentChecksum == "" || len(artifact.Files) != 1 ||
		artifact.Files[0].Path != "main" || !artifact.Files[0].Executable {
		t.Fatalf("Stage() = %+v", artifact)
	}
	files, err := artifact.OpenFiles()
	if err != nil {
		t.Fatalf("OpenFiles() error = %v", err)
	}
	value, err := io.ReadAll(files[0])
	closeFiles(files)
	if err != nil || !bytes.Equal(value, []byte("ELF")) {
		t.Fatalf("artifact content = %q, error = %v", value, err)
	}
	info, err := os.Stat(filepath.Join(stager.rootPath, attemptID.String(), "main"))
	if err != nil {
		t.Fatalf("stat staged artifact: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("staged mode = %v, want 0600", info.Mode())
	}
	if err := artifact.Release(context.Background()); err != nil {
		t.Fatalf("Release() error = %v", err)
	}
	if err := artifact.Release(context.Background()); err != nil {
		t.Fatalf("idempotent Release() error = %v", err)
	}
	stager.mu.Lock()
	reservedBytes, reservedInodes := stager.reservedBytes, stager.reservedInodes
	stager.mu.Unlock()
	if reservedBytes != 0 || reservedInodes != 0 {
		t.Fatalf("quota leaked: bytes=%d inodes=%d", reservedBytes, reservedInodes)
	}
}

func TestStagingRejectsHostileArchiveShapes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		entries []artifactArchiveEntry
	}{
		{name: "absolute", entries: []artifactArchiveEntry{{name: "/artifact/main", kind: tar.TypeReg, body: []byte("x")}}},
		{name: "traversal", entries: []artifactArchiveEntry{{name: "artifact/../escape", kind: tar.TypeReg, body: []byte("x")}}},
		{name: "symlink", entries: []artifactArchiveEntry{{name: "artifact/main", kind: tar.TypeSymlink, link: "/etc/passwd"}}},
		{name: "hardlink", entries: []artifactArchiveEntry{{name: "artifact/main", kind: tar.TypeLink, link: "artifact/other"}}},
		{name: "character device", entries: []artifactArchiveEntry{{name: "artifact/main", kind: tar.TypeChar}}},
		{name: "fifo", entries: []artifactArchiveEntry{{name: "artifact/main", kind: tar.TypeFifo}}},
		{name: "whiteout", entries: []artifactArchiveEntry{{name: "artifact/.wh.main", kind: tar.TypeReg, body: []byte("x")}}},
		{name: "unexpected file", entries: []artifactArchiveEntry{{name: "artifact/secret", kind: tar.TypeReg, body: []byte("x")}}},
		{name: "duplicate", entries: []artifactArchiveEntry{
			{name: "artifact/main", kind: tar.TypeReg, body: []byte("x")},
			{name: "artifact/main", kind: tar.TypeReg, body: []byte("x")},
		}},
		{name: "file directory collision", entries: []artifactArchiveEntry{
			{name: "artifact/main", kind: tar.TypeReg, body: []byte("x")},
			{name: "artifact/main/child", kind: tar.TypeReg, body: []byte("x")},
		}},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			stager := newTestStager(t, 1<<20)
			profile := testArtifactProfile()
			_, err := stager.Stage(context.Background(), StageRequest{
				AttemptID:        uuid.New(),
				Archive:          artifactArchive(t, test.entries),
				RuntimeProfile:   profile,
				ManifestChecksum: profile.ManifestChecksum,
			})
			if !errors.Is(err, ErrInvalidArtifact) {
				t.Fatalf("Stage() error = %v, want ErrInvalidArtifact", err)
			}
		})
	}
}

func TestStagingRejectsTruncatedArchiveAndContentTampering(t *testing.T) {
	t.Parallel()

	stager := newTestStager(t, 1<<20)
	profile := testArtifactProfile()
	valid := artifactArchive(t, []artifactArchiveEntry{{
		name: "artifact/main", mode: 0o700, kind: tar.TypeReg, body: []byte("abcdef"),
	}})
	_, err := stager.Stage(context.Background(), StageRequest{
		AttemptID:        uuid.New(),
		Archive:          valid[:520],
		RuntimeProfile:   profile,
		ManifestChecksum: profile.ManifestChecksum,
	})
	if !errors.Is(err, ErrInvalidArtifact) {
		t.Fatalf("truncated Stage() error = %v", err)
	}

	attemptID := uuid.New()
	artifact, err := stager.Stage(context.Background(), StageRequest{
		AttemptID:        attemptID,
		Archive:          valid,
		RuntimeProfile:   profile,
		ManifestChecksum: profile.ManifestChecksum,
	})
	if err != nil {
		t.Fatalf("Stage() error = %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(stager.rootPath, artifact.lease.root, "main"),
		[]byte("ghijkl"),
		0o600,
	); err != nil {
		t.Fatalf("tamper artifact: %v", err)
	}
	if _, err := artifact.OpenFiles(); !errors.Is(err, ErrInvalidArtifact) {
		t.Fatalf("OpenFiles() error = %v, want checksum rejection", err)
	}
	if err := artifact.Release(context.Background()); err != nil {
		t.Fatalf("Release() error = %v", err)
	}
}

func TestStagingReservesBeforePublishAndRejectsWatermarkExhaustion(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	stager, err := NewStager(StagerConfig{
		Root:        root,
		GlobalBytes: 1 << 20, GlobalInodes: 32,
		PerAttemptBytes: 1 << 20, PerAttemptInodes: 32,
		MinimumFreeBytes: math.MaxInt64,
	})
	if err != nil {
		t.Fatalf("NewStager() error = %v", err)
	}
	t.Cleanup(func() { _ = stager.Close() })
	profile := testArtifactProfile()
	_, err = stager.Stage(context.Background(), StageRequest{
		AttemptID: uuid.New(),
		Archive: artifactArchive(t, []artifactArchiveEntry{{
			name: "artifact/main", mode: 0o700, kind: tar.TypeReg, body: []byte("x"),
		}}),
		RuntimeProfile:   profile,
		ManifestChecksum: profile.ManifestChecksum,
	})
	if !errors.Is(err, ErrStagingQuota) {
		t.Fatalf("Stage() error = %v, want ErrStagingQuota", err)
	}
	entries, readErr := os.ReadDir(root)
	if readErr != nil || len(entries) != 0 {
		t.Fatalf("root entries = %v, error = %v; publish occurred without reservation", entries, readErr)
	}
}

type artifactArchiveEntry struct {
	name string
	mode int64
	kind byte
	link string
	body []byte
}

func artifactArchive(t *testing.T, entries []artifactArchiveEntry) []byte {
	t.Helper()
	var output bytes.Buffer
	writer := tar.NewWriter(&output)
	for _, entry := range entries {
		mode := entry.mode
		if mode == 0 {
			mode = 0o700
		}
		header := &tar.Header{
			Name: entry.name, Mode: mode, Typeflag: entry.kind,
			Linkname: entry.link, Uid: 65532, Gid: 65532,
			Size: int64(len(entry.body)), Format: tar.FormatUSTAR,
		}
		if entry.kind == tar.TypeDir || entry.kind == tar.TypeSymlink ||
			entry.kind == tar.TypeLink || entry.kind == tar.TypeChar ||
			entry.kind == tar.TypeFifo {
			header.Size = 0
		}
		if err := writer.WriteHeader(header); err != nil {
			t.Fatalf("WriteHeader(%q): %v", entry.name, err)
		}
		if header.Size != 0 {
			if _, err := writer.Write(entry.body); err != nil {
				t.Fatalf("Write(%q): %v", entry.name, err)
			}
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close archive: %v", err)
	}
	return output.Bytes()
}

func newTestStager(t *testing.T, maximumBytes int64) *Stager {
	t.Helper()
	stager, err := NewStager(StagerConfig{
		Root:        filepath.Join(t.TempDir(), "artifacts"),
		GlobalBytes: maximumBytes, GlobalInodes: 64,
		PerAttemptBytes: maximumBytes, PerAttemptInodes: 64,
	})
	if err != nil {
		t.Fatalf("NewStager() error = %v", err)
	}
	t.Cleanup(func() { _ = stager.Close() })
	return stager
}

func testArtifactProfile() problementity.RuntimeProfile {
	return problementity.RuntimeProfile{
		ManifestChecksum: "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		Artifacts: []problementity.RuntimeArtifact{{
			Path: "main", Executable: true, MaximumBytes: 1 << 20,
			MaximumFiles: 1, Required: true,
		}},
	}
}
