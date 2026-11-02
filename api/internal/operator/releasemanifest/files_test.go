package releasemanifest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/huynhanx03/judgify/internal/infrastructure/releasecatalog"
)

func TestWriteSandboxMakefileEmitsEveryRuntimeFieldFromACodeOwnedDraft(t *testing.T) {
	t.Parallel()
	draft, err := NewDraft("judge-release.2026.08.22.1")
	if err != nil {
		t.Fatalf("NewDraft() error = %v", err)
	}
	path := filepath.Join(t.TempDir(), "sandbox.mk")
	if err := WriteSandboxMakefile(path, draft); err != nil {
		t.Fatalf("WriteSandboxMakefile() error = %v", err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	rendered := string(contents)
	for _, runtime := range draft.Runtimes {
		prefix, exists := sandboxMakePrefix(runtime.RuntimeKey)
		if !exists {
			t.Fatalf("runtime %q has no Make prefix", runtime.RuntimeKey)
		}
		// The checksum is the field that made this function unreachable: it is
		// always "sha256:<hex>", so it must survive verbatim.
		want := prefix + "_MANIFEST_CHECKSUM := " + runtime.ManifestChecksum
		if !strings.Contains(rendered, want+"\n") {
			t.Fatalf("sandbox makefile missing %q", want)
		}
		if !strings.HasPrefix(runtime.ManifestChecksum, "sha256:") {
			t.Fatalf("runtime %q checksum = %q, want a sha256 digest", runtime.RuntimeKey, runtime.ManifestChecksum)
		}
	}
}

func TestAuthoredFileModeIsAcceptedByTheExclusiveWriterAndReadableByTheCatalog(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "release.json")
	if err := WriteJSONExclusive(path, map[string]string{"release_id": "r"}, AuthoredFileMode); err != nil {
		t.Fatalf("WriteJSONExclusive(AuthoredFileMode) error = %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	// writeExclusive refuses any world bit and the catalog loader refuses any
	// group or world write bit. The constant has to satisfy both at once, which
	// is the pairing that a 0o444 literal silently broke.
	if perm := info.Mode().Perm(); perm&0o007 != 0 || perm&0o022 != 0 {
		t.Fatalf("published manifest mode = %#o", perm)
	}
	if err := releasecatalog.ValidateFilePath(path); err != nil {
		t.Fatalf("ValidateFilePath() error = %v", err)
	}
}

func TestWriteSandboxMakefileRejectsValuesThatCouldEscapeTheAssignment(t *testing.T) {
	t.Parallel()
	for name, injected := range map[string]string{
		"expansion":      "$(shell id)",
		"comment":        "2026.07.1 # trailing",
		"newline":        "2026.07.1\nEXTRA := 1",
		"continuation":   "2026.07.1\\",
		"assignment":     "2026.07.1=1",
		"nestedOperator": "a:=b",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			draft, err := NewDraft("judge-release.2026.08.22.1")
			if err != nil {
				t.Fatalf("NewDraft() error = %v", err)
			}
			draft.Runtimes[0].ProfileVersion = injected
			path := filepath.Join(t.TempDir(), "sandbox.mk")
			if err := WriteSandboxMakefile(path, draft); err == nil {
				t.Fatalf("WriteSandboxMakefile() accepted %q", injected)
			}
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatalf("rejected draft still produced a file: %v", err)
			}
		})
	}
}
