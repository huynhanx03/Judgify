package materiallifecycle

import (
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/security/authentication"

	"github.com/huynhanx03/judgify/internal/application/workflow/admincommand"
	materialdto "github.com/huynhanx03/judgify/internal/material/contract"
	entity "github.com/huynhanx03/judgify/internal/material/domain"
	"github.com/huynhanx03/judgify/internal/ports"
)

func TestWorkflowBindsPreviewAndApplyToTheExactReviewedCommand(t *testing.T) {
	t.Parallel()
	actorID := uuid.MustParse("123e4567-e89b-42d3-a456-426614174001")
	sessionID := uuid.MustParse("123e4567-e89b-42d3-a456-426614174002")
	materialID := uuid.MustParse("123e4567-e89b-42d3-a456-426614174003")
	revisionID := uuid.MustParse("123e4567-e89b-42d3-a456-426614174004")
	commandID := uuid.MustParse("123e4567-e89b-42d3-a456-426614174005")
	expiresAt := time.Date(2026, time.August, 9, 8, 5, 0, 0, time.UTC)
	before := workflowProjection(revisionID, 7, entity.MaterialDraft)
	before.Slug = "cay-fenwick"
	after := before
	after.Version = 8
	after.Status = entity.MaterialPublished
	editorial := &editorialWorkflowStub{
		preview: ports.MaterialLifecyclePreview{
			Action:           ports.MaterialLifecyclePublish,
			MaterialID:       materialID,
			TargetRevisionID: &revisionID,
			Before:           before,
			After:            after,
		},
		receipt: &materialdto.MaterialLifecycleReceipt{
			MaterialID:  materialID,
			Action:      string(ports.MaterialLifecyclePublish),
			CommandID:   commandID,
			EventID:     commandID,
			Version:     8,
			After:       projectionResponse(after),
			CommittedAt: expiresAt.Add(-time.Second),
		},
	}
	confirmations := &confirmationWorkflowStub{
		token:     "v1.reviewed.command",
		expiresAt: expiresAt,
	}
	workflow, err := New(editorial, confirmations, Config{
		RecentAuthenticationTTL: 10 * time.Minute,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	request := Request{
		Action:           ports.MaterialLifecyclePublish,
		MaterialID:       materialID,
		TargetRevisionID: revisionID,
		CommandID:        commandID,
		ExpectedVersion:  7,
		Reason:           "  Đã kiểm tra nội dung và công thức  ",
		ActorID:          actorID,
		SessionID:        sessionID,
	}

	ctx := workflowPrincipalContext(
		request,
		expiresAt.Add(-2*time.Minute),
		time.Now().UTC().Add(time.Hour),
	)
	workflow.now = func() time.Time { return expiresAt }
	preview, err := workflow.Preview(ctx, request)
	if err != nil {
		t.Fatalf("Preview() error = %v", err)
	}
	if preview.ConfirmationToken != confirmations.token ||
		!preview.ConfirmationExpiresAt.Equal(expiresAt) ||
		preview.Action != string(ports.MaterialLifecyclePublish) ||
		preview.Before.Version != 7 || preview.After.Version != 8 {
		t.Fatalf("Preview() = %#v", preview)
	}
	issued := confirmations.issued
	if issued.ActorID != actorID || issued.SessionID != sessionID ||
		issued.IdempotencyKey != commandID ||
		issued.Action != "material.publish" ||
		issued.Resource != "material" || issued.TargetID != materialID.String() ||
		issued.ExpectedVersion != 7 ||
		issued.RequestHash == ([sha256.Size]byte{}) {
		t.Fatalf("issued expectation = %#v", issued)
	}
	if editorial.previewCommand.ExpectedVersion != 7 ||
		editorial.previewCommand.TargetRevisionID != revisionID {
		t.Fatalf("preview command = %#v", editorial.previewCommand)
	}

	request.ConfirmationToken = preview.ConfirmationToken
	receipt, err := workflow.Apply(ctx, request)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if receipt.CommandID != commandID || receipt.Version != 8 {
		t.Fatalf("Apply() receipt = %#v", receipt)
	}
	if confirmations.verified != issued ||
		editorial.applyCommand.RequestHash != issued.RequestHash ||
		editorial.applyCommand.Reason != "Đã kiểm tra nội dung và công thức" {
		t.Fatalf(
			"verify/apply mismatch: verify=%#v apply=%#v",
			confirmations.verified,
			editorial.applyCommand,
		)
	}
}

func TestWorkflowRejectsChangedInputBeforeMaterialMutation(t *testing.T) {
	t.Parallel()
	editorial := &editorialWorkflowStub{}
	confirmations := &confirmationWorkflowStub{
		token:     "v1.reviewed.command",
		expiresAt: time.Now().UTC().Add(time.Minute),
	}
	workflow, err := New(editorial, confirmations, Config{
		RecentAuthenticationTTL: 10 * time.Minute,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	request := validWorkflowRequest()
	now := time.Now().UTC()
	workflow.now = func() time.Time { return now }
	ctx := workflowPrincipalContext(
		request,
		now.Add(-time.Minute),
		now.Add(time.Hour),
	)
	if _, err := workflow.Preview(ctx, request); err != nil {
		t.Fatalf("Preview() error = %v", err)
	}
	confirmations.verifyErr = admincommand.ErrConfirmationMismatch
	request.Reason = "Lý do đã bị thay đổi sau preview"
	request.ConfirmationToken = confirmations.token

	if _, err := workflow.Apply(ctx, request); !errors.Is(
		err,
		admincommand.ErrConfirmationMismatch,
	) {
		t.Fatalf("Apply(changed input) error = %v", err)
	}
	if editorial.applyCalls != 0 {
		t.Fatalf("ApplyLifecycle() calls = %d, want 0", editorial.applyCalls)
	}
}

func TestWorkflowRejectsStaleLiveSessionBeforeConfirmationVerification(t *testing.T) {
	t.Parallel()
	editorial := &editorialWorkflowStub{}
	confirmations := &confirmationWorkflowStub{
		token:     "v1.reviewed.command",
		expiresAt: time.Now().UTC().Add(time.Minute),
	}
	workflow, err := New(editorial, confirmations, Config{
		RecentAuthenticationTTL: 10 * time.Minute,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	now := time.Date(2026, time.August, 9, 8, 0, 0, 0, time.UTC)
	workflow.now = func() time.Time { return now }
	request := validWorkflowRequest()
	request.ConfirmationToken = confirmations.token
	ctx := workflowPrincipalContext(
		request,
		now.Add(-11*time.Minute),
		time.Now().UTC().Add(time.Hour),
	)

	if _, err := workflow.Apply(ctx, request); !errors.Is(
		err,
		admincommand.ErrRecentAuthenticationRequired,
	) {
		t.Fatalf("Apply(stale session) error = %v", err)
	}
	if confirmations.verified != (admincommand.ConfirmationExpectation{}) ||
		editorial.applyCalls != 0 {
		t.Fatal("stale session reached confirmation or material mutation")
	}
}

type editorialWorkflowStub struct {
	preview        ports.MaterialLifecyclePreview
	receipt        *materialdto.MaterialLifecycleReceipt
	previewCommand ports.PreviewMaterialLifecycleCommand
	applyCommand   ports.ApplyMaterialLifecycleCommand
	applyCalls     int
}

func (stub *editorialWorkflowStub) PreviewLifecycle(
	_ context.Context,
	command ports.PreviewMaterialLifecycleCommand,
) (ports.MaterialLifecyclePreview, error) {
	stub.previewCommand = command
	if stub.preview.MaterialID == uuid.Nil {
		target := command.TargetRevisionID
		before := workflowProjection(
			target,
			command.ExpectedVersion,
			entity.MaterialDraft,
		)
		after := before
		after.Version = command.ExpectedVersion + 1
		after.Status = entity.MaterialPublished
		stub.preview = ports.MaterialLifecyclePreview{
			Action:           command.Action,
			MaterialID:       command.MaterialID,
			TargetRevisionID: &target,
			Before:           before,
			After:            after,
		}
	}
	return stub.preview, nil
}

func (stub *editorialWorkflowStub) ApplyLifecycle(
	_ context.Context,
	command ports.ApplyMaterialLifecycleCommand,
) (*materialdto.MaterialLifecycleReceipt, error) {
	stub.applyCalls++
	stub.applyCommand = command
	return stub.receipt, nil
}

type confirmationWorkflowStub struct {
	token     string
	expiresAt time.Time
	issued    admincommand.ConfirmationExpectation
	verified  admincommand.ConfirmationExpectation
	verifyErr error
}

func (stub *confirmationWorkflowStub) IssueWithClaims(
	expectation admincommand.ConfirmationExpectation,
) (string, admincommand.ConfirmationClaims, error) {
	stub.issued = expectation
	return stub.token, admincommand.ConfirmationClaims{
		ConfirmationExpectation: expectation,
		IssuedAt:                stub.expiresAt.Add(-time.Minute),
		ExpiresAt:               stub.expiresAt,
	}, nil
}

func (stub *confirmationWorkflowStub) Verify(
	_ string,
	expectation admincommand.ConfirmationExpectation,
) (admincommand.ConfirmationClaims, error) {
	stub.verified = expectation
	if stub.verifyErr != nil {
		return admincommand.ConfirmationClaims{}, stub.verifyErr
	}
	return admincommand.ConfirmationClaims{
		ConfirmationExpectation: expectation,
		ExpiresAt:               stub.expiresAt,
	}, nil
}

func validWorkflowRequest() Request {
	return Request{
		Action:           ports.MaterialLifecyclePublish,
		MaterialID:       uuid.New(),
		TargetRevisionID: uuid.New(),
		CommandID:        uuid.New(),
		ExpectedVersion:  3,
		Reason:           "Đã rà soát nội dung",
		ActorID:          uuid.New(),
		SessionID:        uuid.New(),
	}
}

func workflowProjection(
	revisionID uuid.UUID,
	version int64,
	status entity.MaterialState,
) ports.MaterialLifecycleProjection {
	return ports.MaterialLifecycleProjection{
		Version:                version,
		Status:                 status,
		Visibility:             entity.MaterialPublic,
		Slug:                   "material-slug",
		RevisionID:             &revisionID,
		RevisionNumber:         4,
		Title:                  "Cây Fenwick",
		CategoryID:             uuid.New(),
		DifficultyID:           uuid.New(),
		TagIDs:                 []uuid.UUID{uuid.New()},
		RenderSourceID:         uuid.New(),
		RenderArtifactID:       uuid.New(),
		SourceChecksum:         "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		RendererVersion:        "renderer-v1",
		SanitizerPolicyVersion: "sanitizer-v1",
	}
}

func workflowPrincipalContext(
	request Request,
	authenticatedAt time.Time,
	authenticatedUntil time.Time,
) context.Context {
	return authentication.WithPrincipal(
		context.Background(),
		authentication.Principal{
			Authenticated:      true,
			Subject:            request.ActorID.String(),
			UserID:             request.ActorID.String(),
			SessionID:          request.SessionID.String(),
			TokenID:            uuid.NewString(),
			AuthenticatedAt:    authenticatedAt,
			AuthenticatedUntil: authenticatedUntil,
			SecurityVersion:    1,
		},
	)
}
