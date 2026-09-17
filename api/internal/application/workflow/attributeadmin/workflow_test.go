package attributeadmin

import (
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/security/authentication"

	"github.com/huynhanx03/judgify/internal/application/workflow/admincommand"
	identitydto "github.com/huynhanx03/judgify/internal/identity/contract"
	"github.com/huynhanx03/judgify/internal/ports"
)

// This catches an activation apply that is no longer bound to the exact
// candidate revision, actor/session, optimistic version, reason, and command.
func TestWorkflowBindsActivationToExactReviewedCommand(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC().Truncate(time.Second)
	request := validAttributeWorkflowRequest()
	preview := attributeLifecyclePreview(request)
	receipt := attributeLifecycleReceipt(request, now)
	lifecycle := &attributeLifecycleStub{preview: preview, receipt: receipt}
	confirmations := &attributeConfirmationStub{
		token:     "v1.attribute.review",
		expiresAt: now.Add(5 * time.Minute),
	}
	workflow, err := New(lifecycle, confirmations, Config{
		RecentAuthenticationTTL: 10 * time.Minute,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	workflow.now = func() time.Time { return now }
	ctx := attributePrincipalContext(request, now.Add(-time.Minute))

	reviewed, err := workflow.Preview(ctx, request)
	if err != nil {
		t.Fatalf("Preview() error = %v", err)
	}
	if reviewed.ConfirmationToken != confirmations.token ||
		reviewed.ReviewHash == "" ||
		reviewed.Preview.CandidateRevision == nil ||
		reviewed.Preview.CandidateRevision.ID != request.CandidateRevisionID ||
		reviewed.Preview.ExpectedVersion != request.ExpectedVersion {
		t.Fatalf("Preview() = %#v", reviewed)
	}
	issued := confirmations.issued
	if issued.ActorID != request.ActorID || issued.SessionID != request.SessionID ||
		issued.IdempotencyKey != request.CommandID ||
		issued.Action != "identity.attribute_definition.activate" ||
		issued.Resource != "identity.attribute_definition" ||
		issued.TargetID != request.DefinitionID.String() ||
		issued.ExpectedVersion != request.ExpectedVersion ||
		issued.RequestHash == ([sha256.Size]byte{}) {
		t.Fatalf("issued expectation = %#v", issued)
	}

	request.ConfirmationToken = reviewed.ConfirmationToken
	request.ReviewHash = reviewed.ReviewHash
	applied, err := workflow.Apply(ctx, request)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if applied.CommandID != request.CommandID ||
		lifecycle.applyCommand.RequestHash != issued.RequestHash ||
		lifecycle.applyCommand.Reason != "Kích hoạt schema đã được duyệt" {
		t.Fatalf("apply mismatch: receipt=%#v command=%#v", applied, lifecycle.applyCommand)
	}
}

// This catches a workflow that verifies a stale review token after mutable
// activation input has changed and still reaches the lifecycle service.
func TestWorkflowRejectsChangedActivationBeforeMutation(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC().Truncate(time.Second)
	request := validAttributeWorkflowRequest()
	lifecycle := &attributeLifecycleStub{preview: attributeLifecyclePreview(request)}
	confirmations := &attributeConfirmationStub{
		token:     "v1.attribute.review",
		expiresAt: now.Add(5 * time.Minute),
	}
	workflow, err := New(lifecycle, confirmations, Config{
		RecentAuthenticationTTL: 10 * time.Minute,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	workflow.now = func() time.Time { return now }
	ctx := attributePrincipalContext(request, now.Add(-time.Minute))
	reviewed, err := workflow.Preview(ctx, request)
	if err != nil {
		t.Fatalf("Preview() error = %v", err)
	}
	confirmations.verifyErr = admincommand.ErrConfirmationMismatch
	request.CandidateRevisionID = uuid.New()
	request.ConfirmationToken = confirmations.token
	request.ReviewHash = reviewed.ReviewHash

	if _, err := workflow.Apply(ctx, request); !errors.Is(err, admincommand.ErrConfirmationMismatch) {
		t.Fatalf("Apply(changed candidate) error = %v", err)
	}
	if lifecycle.applyCalls != 0 {
		t.Fatalf("ApplyLifecycle() calls = %d, want 0", lifecycle.applyCalls)
	}
}

type attributeLifecycleStub struct {
	preview        ports.AttributeLifecyclePreview
	receipt        *identitydto.AttributeLifecycleReceipt
	previewCommand ports.PreviewAttributeLifecycleCommand
	applyCommand   ports.ApplyAttributeLifecycleCommand
	applyCalls     int
}

func (stub *attributeLifecycleStub) PreviewLifecycle(
	_ context.Context,
	command ports.PreviewAttributeLifecycleCommand,
) (ports.AttributeLifecyclePreview, error) {
	stub.previewCommand = command
	return stub.preview, nil
}

func (stub *attributeLifecycleStub) ApplyLifecycle(
	_ context.Context,
	command ports.ApplyAttributeLifecycleCommand,
) (*identitydto.AttributeLifecycleReceipt, error) {
	stub.applyCalls++
	stub.applyCommand = command
	return stub.receipt, nil
}

type attributeConfirmationStub struct {
	token     string
	expiresAt time.Time
	issued    admincommand.ConfirmationExpectation
	verified  admincommand.ConfirmationExpectation
	verifyErr error
}

func (stub *attributeConfirmationStub) IssueWithClaims(
	expectation admincommand.ConfirmationExpectation,
) (string, admincommand.ConfirmationClaims, error) {
	stub.issued = expectation
	return stub.token, admincommand.ConfirmationClaims{
		ConfirmationExpectation: expectation,
		IssuedAt:                stub.expiresAt.Add(-time.Minute),
		ExpiresAt:               stub.expiresAt,
	}, nil
}

func (stub *attributeConfirmationStub) Verify(
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

func validAttributeWorkflowRequest() Request {
	return Request{
		Action:              ports.AttributeLifecycleActivate,
		DefinitionID:        uuid.New(),
		CandidateRevisionID: uuid.New(),
		CommandID:           uuid.New(),
		ExpectedVersion:     4,
		Reason:              "  Kích hoạt schema đã được duyệt  ",
		ActorID:             uuid.New(),
		SessionID:           uuid.New(),
	}
}

func attributeLifecyclePreview(request Request) ports.AttributeLifecyclePreview {
	return ports.AttributeLifecyclePreview{
		Action:           request.Action,
		DefinitionID:     request.DefinitionID,
		ExpectedVersion:  request.ExpectedVersion,
		ResultingVersion: request.ExpectedVersion + 1,
		CandidateRevision: &identitydto.AttributeDefinitionRevisionResponse{
			ID:           request.CandidateRevisionID,
			DefinitionID: request.DefinitionID,
			Revision:     2,
			DataType:     "string",
			Label:        "Biệt danh",
			Visibility:   "public",
			Validation:   []byte(`{}`),
			CreatedAt:    time.Date(2026, time.August, 10, 7, 0, 0, 0, time.UTC),
		},
	}
}

func attributeLifecycleReceipt(request Request, committedAt time.Time) *identitydto.AttributeLifecycleReceipt {
	activeRevisionID := request.CandidateRevisionID
	eventID := request.CommandID
	return &identitydto.AttributeLifecycleReceipt{
		Status:       identitydto.AttributeLifecycleCommitted,
		Action:       string(request.Action),
		CommandID:    request.CommandID,
		EventID:      &eventID,
		DefinitionID: request.DefinitionID,
		Version:      request.ExpectedVersion + 1,
		Definition: &identitydto.AttributeDefinitionResponse{
			ID:               request.DefinitionID,
			Key:              "nickname",
			Kind:             "custom",
			Status:           "active",
			Version:          request.ExpectedVersion + 1,
			DraftRevisionID:  &activeRevisionID,
			ActiveRevisionID: &activeRevisionID,
		},
		ActiveRevision: &identitydto.AttributeDefinitionRevisionResponse{
			ID:           request.CandidateRevisionID,
			DefinitionID: request.DefinitionID,
			Revision:     2,
			DataType:     "string",
			Label:        "Biệt danh",
			Visibility:   "public",
			Validation:   []byte(`{}`),
			CreatedAt:    committedAt.Add(-time.Hour),
		},
		RecordedAt: committedAt,
	}
}

func attributePrincipalContext(request Request, authenticatedAt time.Time) context.Context {
	return authentication.WithPrincipal(context.Background(), authentication.Principal{
		Authenticated:      true,
		Subject:            request.ActorID.String(),
		UserID:             request.ActorID.String(),
		SessionID:          request.SessionID.String(),
		TokenID:            uuid.NewString(),
		AuthenticatedAt:    authenticatedAt,
		AuthenticatedUntil: authenticatedAt.Add(time.Hour),
		SecurityVersion:    1,
	})
}
