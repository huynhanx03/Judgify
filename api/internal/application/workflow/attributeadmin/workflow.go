// Package attributeadmin binds reviewed attribute-definition lifecycle
// evidence to the exact authenticated administrative command. Authorization
// remains a route concern and durable mutation remains owned by Identity.
package attributeadmin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/huynhanx03/judgify/internal/application/workflow/admincommand"
	"github.com/huynhanx03/judgify/internal/identity/attributemigration"
	identitydto "github.com/huynhanx03/judgify/internal/identity/contract"
	"github.com/huynhanx03/judgify/internal/platform/operation"
	"github.com/huynhanx03/judgify/internal/ports"
)

const attributeConfirmationResource = "identity.attribute_definition"

var ErrInvalidWorkflow = errors.New("attribute admin: invalid workflow")

type Config struct {
	RecentAuthenticationTTL time.Duration
}

type Request struct {
	Action              ports.AttributeLifecycleAction
	DefinitionID        uuid.UUID
	CandidateRevisionID uuid.UUID
	CommandID           uuid.UUID
	ExpectedVersion     int64
	Reason              string
	ConfirmationToken   string
	ReviewHash          string
	ActorID             uuid.UUID
	SessionID           uuid.UUID
}

type lifecycleService interface {
	PreviewLifecycle(context.Context, ports.PreviewAttributeLifecycleCommand) (ports.AttributeLifecyclePreview, error)
	ApplyLifecycle(context.Context, ports.ApplyAttributeLifecycleCommand) (*identitydto.AttributeLifecycleReceipt, error)
}

type confirmationProtector interface {
	IssueWithClaims(admincommand.ConfirmationExpectation) (string, admincommand.ConfirmationClaims, error)
	Verify(string, admincommand.ConfirmationExpectation) (admincommand.ConfirmationClaims, error)
}

type Workflow struct {
	lifecycle     lifecycleService
	confirmations confirmationProtector
	config        Config
	now           func() time.Time
}

func New(
	lifecycle lifecycleService,
	confirmations confirmationProtector,
	config Config,
) (*Workflow, error) {
	if lifecycle == nil || confirmations == nil ||
		config.RecentAuthenticationTTL < time.Minute ||
		config.RecentAuthenticationTTL > time.Hour {
		return nil, ErrInvalidWorkflow
	}
	return &Workflow{
		lifecycle:     lifecycle,
		confirmations: confirmations,
		config:        config,
		now:           func() time.Time { return time.Now().UTC() },
	}, nil
}

func (workflow *Workflow) Preview(
	ctx context.Context,
	request Request,
) (*identitydto.AttributeLifecyclePreviewResponse, error) {
	request, err := workflow.prepare(request, false)
	if err != nil {
		return nil, err
	}
	preview, err := workflow.lifecycle.PreviewLifecycle(ctx, ports.PreviewAttributeLifecycleCommand{
		Action:              request.Action,
		DefinitionID:        request.DefinitionID,
		CandidateRevisionID: request.CandidateRevisionID,
		ExpectedVersion:     request.ExpectedVersion,
	})
	if err != nil {
		return nil, err
	}
	if !validPreview(preview, request) {
		return nil, ErrInvalidWorkflow
	}
	reviewHash, err := ports.HashAttributeLifecyclePreview(preview)
	if err != nil || reviewHash == ([sha256.Size]byte{}) {
		return nil, ErrInvalidWorkflow
	}
	expectation, err := confirmationExpectation(request, reviewHash)
	if err != nil {
		return nil, err
	}
	token, claims, err := workflow.confirmations.IssueWithClaims(expectation)
	if err != nil {
		return nil, err
	}
	return &identitydto.AttributeLifecyclePreviewResponse{
		Preview: identitydto.AttributeLifecyclePreview{
			Action:            string(preview.Action),
			DefinitionID:      preview.DefinitionID,
			ExpectedVersion:   preview.ExpectedVersion,
			ResultingVersion:  preview.ResultingVersion,
			CurrentRevision:   cloneRevision(preview.CurrentRevision),
			CandidateRevision: cloneRevision(preview.CandidateRevision),
			SourceValueCount:  preview.SourceValueCount,
			TargetValueCount:  preview.TargetValueCount,
			RequiresMigration: preview.RequiresMigration,
		},
		ConfirmationToken: token,
		ReviewHash:        hex.EncodeToString(reviewHash[:]),
		ExpiresAt:         claims.ExpiresAt.UTC(),
	}, nil
}

func (workflow *Workflow) Apply(
	ctx context.Context,
	request Request,
) (*identitydto.AttributeLifecycleReceipt, error) {
	request, err := workflow.prepare(request, true)
	if err != nil {
		return nil, err
	}
	reviewHash, err := parseReviewHash(request.ReviewHash)
	if err != nil {
		return nil, err
	}
	expectation, err := confirmationExpectation(request, reviewHash)
	if err != nil {
		return nil, err
	}
	if err := admincommand.RequireRecentAuthentication(
		ctx,
		request.ActorID,
		request.SessionID,
		workflow.now().UTC(),
		workflow.config.RecentAuthenticationTTL,
	); err != nil {
		return nil, err
	}
	if _, err := workflow.confirmations.Verify(request.ConfirmationToken, expectation); err != nil {
		return nil, err
	}
	receipt, err := workflow.lifecycle.ApplyLifecycle(ctx, ports.ApplyAttributeLifecycleCommand{
		Action:              request.Action,
		DefinitionID:        request.DefinitionID,
		CandidateRevisionID: request.CandidateRevisionID,
		ExpectedVersion:     request.ExpectedVersion,
		CommandID:           request.CommandID,
		RequestHash:         expectation.RequestHash,
		ReviewedPreviewHash: reviewHash,
		Reason:              request.Reason,
	})
	if err != nil {
		return nil, err
	}
	if !validReceipt(receipt, request) {
		return nil, ErrInvalidWorkflow
	}
	return receipt, nil
}

func (workflow *Workflow) prepare(
	request Request,
	requireConfirmation bool,
) (Request, error) {
	if workflow == nil || workflow.lifecycle == nil || workflow.confirmations == nil ||
		!request.Action.Valid() || request.DefinitionID == uuid.Nil ||
		(request.Action == ports.AttributeLifecycleActivate) !=
			(request.CandidateRevisionID != uuid.Nil) ||
		(!requireConfirmation && (request.ConfirmationToken != "" || request.ReviewHash != "")) {
		return Request{}, ErrInvalidWorkflow
	}
	meta, err := admincommand.NormalizeCommandMeta(admincommand.CommandMeta{
		ActorID:           request.ActorID,
		SessionID:         request.SessionID,
		ExpectedVersion:   request.ExpectedVersion,
		IdempotencyKey:    request.CommandID,
		Reason:            request.Reason,
		ConfirmationToken: request.ConfirmationToken,
	}, admincommand.CommandMetaPolicy{
		RequireIdempotency:  true,
		RequireConfirmation: requireConfirmation,
	})
	if err != nil {
		return Request{}, err
	}
	request.Reason = meta.Reason
	request.ConfirmationToken = meta.ConfirmationToken
	if requireConfirmation {
		if _, err := parseReviewHash(request.ReviewHash); err != nil {
			return Request{}, err
		}
	}
	return request, nil
}

func confirmationExpectation(
	request Request,
	reviewHash [sha256.Size]byte,
) (admincommand.ConfirmationExpectation, error) {
	if reviewHash == ([sha256.Size]byte{}) {
		return admincommand.ConfirmationExpectation{}, ErrInvalidWorkflow
	}
	reviewHashText := hex.EncodeToString(reviewHash[:])
	document, err := json.Marshal(struct {
		Action              string    `json:"action"`
		DefinitionID        uuid.UUID `json:"definition_id"`
		CandidateRevisionID uuid.UUID `json:"candidate_revision_id,omitempty"`
		CommandID           uuid.UUID `json:"command_id"`
		ExpectedVersion     int64     `json:"expected_version"`
		Reason              string    `json:"reason"`
		ReviewHash          string    `json:"review_hash"`
	}{
		Action:              string(request.Action),
		DefinitionID:        request.DefinitionID,
		CandidateRevisionID: request.CandidateRevisionID,
		CommandID:           request.CommandID,
		ExpectedVersion:     request.ExpectedVersion,
		Reason:              request.Reason,
		ReviewHash:          reviewHashText,
	})
	if err != nil {
		return admincommand.ConfirmationExpectation{}, ErrInvalidWorkflow
	}
	hash, err := admincommand.ConfirmationRequestHash(
		"identity.attribute_definition."+string(request.Action)+".v1",
		document,
	)
	if err != nil || hash == ([sha256.Size]byte{}) {
		return admincommand.ConfirmationExpectation{}, ErrInvalidWorkflow
	}
	return admincommand.ConfirmationExpectation{
		ActorID:         request.ActorID,
		SessionID:       request.SessionID,
		IdempotencyKey:  request.CommandID,
		Action:          "identity.attribute_definition." + string(request.Action),
		Resource:        attributeConfirmationResource,
		TargetID:        request.DefinitionID.String(),
		ExpectedVersion: request.ExpectedVersion,
		RequestHash:     hash,
	}, nil
}

func parseReviewHash(value string) ([sha256.Size]byte, error) {
	decoded, err := hex.DecodeString(value)
	if err != nil || len(decoded) != sha256.Size || hex.EncodeToString(decoded) != value {
		return [sha256.Size]byte{}, ErrInvalidWorkflow
	}
	var result [sha256.Size]byte
	copy(result[:], decoded)
	return result, nil
}

func validPreview(
	preview ports.AttributeLifecyclePreview,
	request Request,
) bool {
	if preview.Action != request.Action || preview.DefinitionID != request.DefinitionID ||
		preview.ExpectedVersion != request.ExpectedVersion ||
		preview.ResultingVersion != request.ExpectedVersion+1 ||
		preview.SourceValueCount < 0 || preview.TargetValueCount < 0 {
		return false
	}
	if request.Action == ports.AttributeLifecycleArchive {
		return preview.CurrentRevision != nil &&
			preview.CurrentRevision.DefinitionID == request.DefinitionID &&
			preview.CandidateRevision == nil && preview.TargetValueCount == 0 &&
			!preview.RequiresMigration
	}
	if preview.RequiresMigration !=
		(preview.SourceValueCount > 0 || preview.TargetValueCount > 0) {
		return false
	}
	return preview.CandidateRevision != nil &&
		preview.CandidateRevision.ID == request.CandidateRevisionID &&
		preview.CandidateRevision.DefinitionID == request.DefinitionID &&
		preview.CandidateRevision.Revision > 0 &&
		strings.TrimSpace(preview.CandidateRevision.Label) != "" &&
		preview.CandidateRevision.CreatedAt.Location() == time.UTC
}

func validReceipt(
	receipt *identitydto.AttributeLifecycleReceipt,
	request Request,
) bool {
	if receipt == nil || receipt.Action != string(request.Action) ||
		receipt.CommandID != request.CommandID ||
		receipt.DefinitionID != request.DefinitionID ||
		receipt.RecordedAt.IsZero() ||
		receipt.Definition == nil || receipt.Definition.ID != request.DefinitionID ||
		receipt.Definition.Version != receipt.Version {
		return false
	}
	if receipt.Status == identitydto.AttributeLifecycleMigrationScheduled {
		return request.Action == ports.AttributeLifecycleActivate &&
			receipt.EventID == nil && receipt.Version == request.ExpectedVersion &&
			receipt.Operation != nil && receipt.Operation.ID == request.CommandID &&
			receipt.Operation.Kind == attributemigration.OperationKind &&
			receipt.Operation.ActorUserID == request.ActorID &&
			receipt.Operation.Total > 0 &&
			operation.ValidateProjection(*receipt.Operation) == nil &&
			receipt.ActiveRevision != nil &&
			receipt.ActiveRevision.DefinitionID == request.DefinitionID &&
			receipt.Definition.Status == "active" &&
			receipt.Definition.ActiveRevisionID != nil &&
			*receipt.Definition.ActiveRevisionID == receipt.ActiveRevision.ID &&
			receipt.ActiveRevision.ID != request.CandidateRevisionID
	}
	if receipt.Status != identitydto.AttributeLifecycleCommitted ||
		receipt.EventID == nil || *receipt.EventID != request.CommandID ||
		receipt.Operation != nil || receipt.Version != request.ExpectedVersion+1 {
		return false
	}
	if request.Action == ports.AttributeLifecycleArchive {
		return receipt.Definition.Status == "archived" && receipt.ActiveRevision == nil
	}
	return receipt.Definition.Status == "active" && receipt.ActiveRevision != nil &&
		receipt.ActiveRevision.ID == request.CandidateRevisionID &&
		receipt.ActiveRevision.DefinitionID == request.DefinitionID &&
		receipt.Definition.ActiveRevisionID != nil &&
		*receipt.Definition.ActiveRevisionID == request.CandidateRevisionID
}

func cloneRevision(
	value *identitydto.AttributeDefinitionRevisionResponse,
) *identitydto.AttributeDefinitionRevisionResponse {
	if value == nil {
		return nil
	}
	cloned := *value
	cloned.Validation = append([]byte(nil), value.Validation...)
	return &cloned
}
