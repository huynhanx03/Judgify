package releasemanifest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestBuildApprovedUsesOnePinnedImageAndOnlyExplicitCapabilityPairs(t *testing.T) {
	t.Parallel()
	draft, err := NewDraft("judge-release.2026.07.29.1")
	if err != nil {
		t.Fatalf("NewDraft() error = %v", err)
	}
	draft.Capabilities = []CapabilitySelection{
		{RuntimeKey: "cpp17", CheckerKey: "exact"},
		{RuntimeKey: "python3", CheckerKey: "token"},
	}
	const imageDigest = "sha256:90caa209195cca93d8c1a9dae708e2a4ecf6b402349a4b3ff899c2d7297157ac"
	manifest, summary, err := BuildApproved(context.Background(), draft, Approval{
		SandboxImageDigest: imageDigest,
		ApprovedBy:         uuid.MustParse("d5d312c4-770b-47b9-a5ea-73f51b1498d9"),
		AuditID:            uuid.MustParse("115a9ed4-bc0e-43f3-b4fb-bd728c85033b"),
		ApprovedAt:         time.Date(2026, time.July, 29, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("BuildApproved() error = %v", err)
	}
	if summary.CapabilityCount != 2 || len(manifest.Capabilities) != 2 {
		t.Fatalf("approved capabilities = %d, want explicit 2", len(manifest.Capabilities))
	}
	for _, runtime := range manifest.Runtimes {
		if runtime.ImageDigest != imageDigest {
			t.Fatalf("runtime %q image = %q", runtime.RuntimeKey, runtime.ImageDigest)
		}
	}
	if _, err := Verify(context.Background(), manifest, imageDigest); err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
}

func TestBuildApprovedRejectsEditedCodeOwnedEvidenceAndImplicitCapabilities(t *testing.T) {
	t.Parallel()
	draft, err := NewDraft("judge-release.2026.07.29.2")
	if err != nil {
		t.Fatalf("NewDraft() error = %v", err)
	}
	approval := Approval{
		SandboxImageDigest: "sha256:90caa209195cca93d8c1a9dae708e2a4ecf6b402349a4b3ff899c2d7297157ac",
		ApprovedBy:         uuid.New(), AuditID: uuid.New(),
		ApprovedAt: time.Date(2026, time.July, 29, 12, 0, 0, 0, time.UTC),
	}
	if _, _, err := BuildApproved(context.Background(), draft, approval); !errors.Is(err, ErrInvalidDraft) {
		t.Fatalf("BuildApproved() empty capability error = %v, want %v", err, ErrInvalidDraft)
	}
	draft.Capabilities = []CapabilitySelection{{RuntimeKey: "cpp17", CheckerKey: "exact"}}
	draft.Runtimes[0].CompileCommandID = "cpp17.compile.edited"
	if _, _, err := BuildApproved(context.Background(), draft, approval); !errors.Is(err, ErrInvalidDraft) {
		t.Fatalf("BuildApproved() edited evidence error = %v, want %v", err, ErrInvalidDraft)
	}
}
