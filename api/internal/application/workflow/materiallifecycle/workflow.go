// Package materiallifecycle coordinates reviewed Material lifecycle commands.
// Authorization remains at the HTTP route and durable state remains owned by
// Material; this workflow only binds preview evidence to the exact command.
package materiallifecycle

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
	materialdto "github.com/huynhanx03/judgify/internal/material/contract"
	materialentity "github.com/huynhanx03/judgify/internal/material/domain"
	"github.com/huynhanx03/judgify/internal/ports"
)

const materialConfirmationResource = "material"

var (
	ErrInvalidWorkflow = errors.New("material lifecycle: invalid workflow")
)

type Config struct {
	RecentAuthenticationTTL time.Duration
}

type Request struct {
	Action            ports.MaterialLifecycleAction
	MaterialID        uuid.UUID
	TargetRevisionID  uuid.UUID
	CommandID         uuid.UUID
	ExpectedVersion   int64
	Reason            string
	ConfirmationToken string
	ActorID           uuid.UUID
	SessionID         uuid.UUID
}

type editorialService interface {
	PreviewLifecycle(
		context.Context,
		ports.PreviewMaterialLifecycleCommand,
	) (ports.MaterialLifecyclePreview, error)
	ApplyLifecycle(
		context.Context,
		ports.ApplyMaterialLifecycleCommand,
	) (*materialdto.MaterialLifecycleReceipt, error)
}

type confirmationProtector interface {
	IssueWithClaims(
		admincommand.ConfirmationExpectation,
	) (string, admincommand.ConfirmationClaims, error)
	Verify(
		string,
		admincommand.ConfirmationExpectation,
	) (admincommand.ConfirmationClaims, error)
}

type Workflow struct {
	editorial     editorialService
	confirmations confirmationProtector
	config        Config
	now           func() time.Time
}

func New(
	editorial editorialService,
	confirmations confirmationProtector,
	config Config,
) (*Workflow, error) {
	if editorial == nil || confirmations == nil ||
		config.RecentAuthenticationTTL < time.Minute ||
		config.RecentAuthenticationTTL > time.Hour {
		return nil, ErrInvalidWorkflow
	}
	return &Workflow{
		editorial:     editorial,
		confirmations: confirmations,
		config:        config,
		now:           func() time.Time { return time.Now().UTC() },
	}, nil
}

func (workflow *Workflow) Preview(
	ctx context.Context,
	request Request,
) (*materialdto.MaterialLifecyclePreviewResponse, error) {
	request, expectation, err := workflow.prepare(request, false)
	if err != nil {
		return nil, err
	}
	preview, err := workflow.editorial.PreviewLifecycle(
		ctx,
		ports.PreviewMaterialLifecycleCommand{
			Action:           request.Action,
			MaterialID:       request.MaterialID,
			TargetRevisionID: request.TargetRevisionID,
			ExpectedVersion:  request.ExpectedVersion,
		},
	)
	if err != nil {
		return nil, err
	}
	if err := validatePreview(preview, request); err != nil {
		return nil, err
	}
	token, claims, err := workflow.confirmations.IssueWithClaims(expectation)
	if err != nil {
		return nil, err
	}
	return previewResponse(preview, token, claims), nil
}

func (workflow *Workflow) Apply(
	ctx context.Context,
	request Request,
) (*materialdto.MaterialLifecycleReceipt, error) {
	request, expectation, err := workflow.prepare(request, true)
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
	if _, err := workflow.confirmations.Verify(
		request.ConfirmationToken,
		expectation,
	); err != nil {
		return nil, err
	}
	receipt, err := workflow.editorial.ApplyLifecycle(
		ctx,
		ports.ApplyMaterialLifecycleCommand{
			Action:           request.Action,
			MaterialID:       request.MaterialID,
			TargetRevisionID: request.TargetRevisionID,
			ExpectedVersion:  request.ExpectedVersion,
			CommandID:        request.CommandID,
			RequestHash:      expectation.RequestHash,
			Reason:           request.Reason,
		},
	)
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
) (Request, admincommand.ConfirmationExpectation, error) {
	if workflow == nil || workflow.editorial == nil ||
		workflow.confirmations == nil || !request.Action.Valid() ||
		request.MaterialID == uuid.Nil ||
		(!lifecycleRequiresTargetRevision(request.Action) !=
			(request.TargetRevisionID == uuid.Nil)) {
		return Request{}, admincommand.ConfirmationExpectation{}, ErrInvalidWorkflow
	}
	meta, err := admincommand.NormalizeCommandMeta(
		admincommand.CommandMeta{
			ActorID:           request.ActorID,
			SessionID:         request.SessionID,
			ExpectedVersion:   request.ExpectedVersion,
			IdempotencyKey:    request.CommandID,
			Reason:            request.Reason,
			ConfirmationToken: request.ConfirmationToken,
		},
		admincommand.CommandMetaPolicy{
			RequireIdempotency:  true,
			RequireConfirmation: requireConfirmation,
		},
	)
	if err != nil {
		return Request{}, admincommand.ConfirmationExpectation{}, err
	}
	request.Reason = meta.Reason
	request.ConfirmationToken = meta.ConfirmationToken
	document, err := canonicalRequestDocument(request)
	if err != nil {
		return Request{}, admincommand.ConfirmationExpectation{}, err
	}
	hash, err := admincommand.ConfirmationRequestHash(
		"material."+string(request.Action)+".v1",
		document,
	)
	if err != nil || hash == ([sha256.Size]byte{}) {
		return Request{}, admincommand.ConfirmationExpectation{}, ErrInvalidWorkflow
	}
	return request, admincommand.ConfirmationExpectation{
		ActorID:         meta.ActorID,
		SessionID:       meta.SessionID,
		IdempotencyKey:  meta.IdempotencyKey,
		Action:          "material." + string(request.Action),
		Resource:        materialConfirmationResource,
		TargetID:        request.MaterialID.String(),
		ExpectedVersion: meta.ExpectedVersion,
		RequestHash:     hash,
	}, nil
}

func canonicalRequestDocument(request Request) ([]byte, error) {
	return json.Marshal(struct {
		Action           string    `json:"action"`
		MaterialID       uuid.UUID `json:"material_id"`
		TargetRevisionID uuid.UUID `json:"target_revision_id,omitempty"`
		ExpectedVersion  int64     `json:"expected_version"`
		IdempotencyKey   uuid.UUID `json:"idempotency_key"`
		Reason           string    `json:"reason"`
	}{
		Action:           string(request.Action),
		MaterialID:       request.MaterialID,
		TargetRevisionID: request.TargetRevisionID,
		ExpectedVersion:  request.ExpectedVersion,
		IdempotencyKey:   request.CommandID,
		Reason:           request.Reason,
	})
}

func validatePreview(
	preview ports.MaterialLifecyclePreview,
	request Request,
) error {
	if preview.Action != request.Action ||
		preview.MaterialID != request.MaterialID ||
		preview.Before.Version != request.ExpectedVersion ||
		preview.After.Version != request.ExpectedVersion+1 ||
		preview.Before.Slug != preview.After.Slug ||
		preview.Before.Visibility != preview.After.Visibility ||
		!validLifecycleProjection(preview.Before) ||
		!validLifecycleProjection(preview.After) {
		return ErrInvalidWorkflow
	}
	if request.Action == ports.MaterialLifecycleArchive {
		if preview.TargetRevisionID != nil ||
			preview.After.Status != "archived" ||
			!sameLifecycleArtifact(preview.Before, preview.After) {
			return ErrInvalidWorkflow
		}
		return nil
	}
	if request.Action == ports.MaterialLifecycleSubmitReview {
		if preview.TargetRevisionID != nil || preview.After.Status != "in_review" ||
			!sameLifecycleArtifact(preview.Before, preview.After) {
			return ErrInvalidWorkflow
		}
		return nil
	}
	if request.Action == ports.MaterialLifecycleReturnToDraft {
		if preview.TargetRevisionID != nil || preview.After.Status != "draft" ||
			!sameLifecycleArtifact(preview.Before, preview.After) {
			return ErrInvalidWorkflow
		}
		return nil
	}
	if preview.TargetRevisionID == nil ||
		*preview.TargetRevisionID != request.TargetRevisionID ||
		preview.After.RevisionID == nil ||
		*preview.After.RevisionID != request.TargetRevisionID ||
		preview.After.RevisionNumber < 1 ||
		strings.TrimSpace(preview.After.Title) == "" ||
		preview.After.Status != "published" {
		return ErrInvalidWorkflow
	}
	if uuidPointersEqual(preview.Before.RevisionID, preview.After.RevisionID) &&
		!sameLifecycleArtifact(preview.Before, preview.After) {
		return ErrInvalidWorkflow
	}
	return nil
}

func validLifecycleProjection(
	projection ports.MaterialLifecycleProjection,
) bool {
	if projection.Version < 1 || !projection.Status.Valid() ||
		!projection.Visibility.Valid() || strings.TrimSpace(projection.Slug) == "" ||
		projection.RevisionID == nil || *projection.RevisionID == uuid.Nil ||
		projection.RevisionNumber < 1 || strings.TrimSpace(projection.Title) == "" ||
		projection.CategoryID == uuid.Nil || projection.DifficultyID == uuid.Nil ||
		projection.RenderSourceID == uuid.Nil ||
		projection.RenderArtifactID == uuid.Nil ||
		strings.TrimSpace(projection.RendererVersion) == "" ||
		strings.TrimSpace(projection.SanitizerPolicyVersion) == "" ||
		len(projection.TagIDs) > 64 {
		return false
	}
	if !validLifecycleChecksum(projection.SourceChecksum) {
		return false
	}
	seen := make(map[uuid.UUID]struct{}, len(projection.TagIDs))
	for _, tagID := range projection.TagIDs {
		if tagID == uuid.Nil {
			return false
		}
		if _, duplicate := seen[tagID]; duplicate {
			return false
		}
		seen[tagID] = struct{}{}
	}
	return true
}

func sameLifecycleArtifact(
	before ports.MaterialLifecycleProjection,
	after ports.MaterialLifecycleProjection,
) bool {
	return uuidPointersEqual(before.RevisionID, after.RevisionID) &&
		before.RevisionNumber == after.RevisionNumber &&
		before.Title == after.Title &&
		before.CategoryID == after.CategoryID &&
		before.DifficultyID == after.DifficultyID &&
		uuidSlicesEqual(before.TagIDs, after.TagIDs) &&
		before.RenderSourceID == after.RenderSourceID &&
		before.RenderArtifactID == after.RenderArtifactID &&
		before.SourceChecksum == after.SourceChecksum &&
		before.RendererVersion == after.RendererVersion &&
		before.SanitizerPolicyVersion == after.SanitizerPolicyVersion
}

func uuidPointersEqual(left, right *uuid.UUID) bool {
	return left != nil && right != nil && *left == *right
}

func uuidSlicesEqual(left, right []uuid.UUID) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func previewResponse(
	preview ports.MaterialLifecyclePreview,
	token string,
	claims admincommand.ConfirmationClaims,
) *materialdto.MaterialLifecyclePreviewResponse {
	return &materialdto.MaterialLifecyclePreviewResponse{
		Action:                string(preview.Action),
		MaterialID:            preview.MaterialID,
		TargetRevisionID:      cloneUUID(preview.TargetRevisionID),
		ExpectedVersion:       preview.Before.Version,
		Before:                projectionResponse(preview.Before),
		After:                 projectionResponse(preview.After),
		ConfirmationToken:     token,
		ConfirmationExpiresAt: claims.ExpiresAt,
	}
}

func projectionResponse(
	projection ports.MaterialLifecycleProjection,
) materialdto.MaterialLifecycleProjectionResponse {
	return materialdto.MaterialLifecycleProjectionResponse{
		Version:                projection.Version,
		Status:                 string(projection.Status),
		Visibility:             string(projection.Visibility),
		Slug:                   projection.Slug,
		RevisionID:             cloneUUID(projection.RevisionID),
		RevisionNumber:         projection.RevisionNumber,
		Title:                  projection.Title,
		CategoryID:             projection.CategoryID,
		DifficultyID:           projection.DifficultyID,
		TagIDs:                 append([]uuid.UUID(nil), projection.TagIDs...),
		RenderSourceID:         projection.RenderSourceID,
		RenderArtifactID:       projection.RenderArtifactID,
		SourceChecksum:         projection.SourceChecksum,
		RendererVersion:        projection.RendererVersion,
		SanitizerPolicyVersion: projection.SanitizerPolicyVersion,
	}
}

func validReceipt(
	receipt *materialdto.MaterialLifecycleReceipt,
	request Request,
) bool {
	if receipt == nil || receipt.MaterialID != request.MaterialID ||
		receipt.Action != string(request.Action) ||
		receipt.CommandID != request.CommandID ||
		receipt.EventID != request.CommandID ||
		receipt.Version != request.ExpectedVersion+1 ||
		receipt.After.Version != receipt.Version ||
		receipt.CommittedAt.IsZero() ||
		!validLifecycleProjectionResponse(receipt.After) {
		return false
	}
	if request.Action == ports.MaterialLifecycleArchive {
		return receipt.After.Status == "archived"
	}
	if request.Action == ports.MaterialLifecycleSubmitReview {
		return receipt.After.Status == "in_review"
	}
	if request.Action == ports.MaterialLifecycleReturnToDraft {
		return receipt.After.Status == "draft"
	}
	return receipt.After.Status == "published" &&
		receipt.After.RevisionID != nil &&
		*receipt.After.RevisionID == request.TargetRevisionID
}

func lifecycleRequiresTargetRevision(action ports.MaterialLifecycleAction) bool {
	return action == ports.MaterialLifecyclePublish || action == ports.MaterialLifecycleRollback
}

func validLifecycleProjectionResponse(
	projection materialdto.MaterialLifecycleProjectionResponse,
) bool {
	if !validLifecycleChecksum(projection.SourceChecksum) ||
		projection.Version < 1 || projection.RevisionID == nil ||
		*projection.RevisionID == uuid.Nil || projection.RevisionNumber < 1 ||
		!materialentity.MaterialState(projection.Status).Valid() ||
		!materialentity.MaterialVisibility(projection.Visibility).Valid() ||
		strings.TrimSpace(projection.Title) == "" ||
		projection.CategoryID == uuid.Nil || projection.DifficultyID == uuid.Nil ||
		projection.RenderSourceID == uuid.Nil ||
		projection.RenderArtifactID == uuid.Nil ||
		strings.TrimSpace(projection.RendererVersion) == "" ||
		strings.TrimSpace(projection.SanitizerPolicyVersion) == "" ||
		strings.TrimSpace(projection.Slug) == "" || len(projection.TagIDs) > 64 {
		return false
	}
	seen := make(map[uuid.UUID]struct{}, len(projection.TagIDs))
	for _, tagID := range projection.TagIDs {
		if tagID == uuid.Nil {
			return false
		}
		if _, duplicate := seen[tagID]; duplicate {
			return false
		}
		seen[tagID] = struct{}{}
	}
	return true
}

func validLifecycleChecksum(value string) bool {
	checksum, err := hex.DecodeString(value)
	if err != nil || len(checksum) != sha256.Size {
		return false
	}
	for _, current := range checksum {
		if current != 0 {
			return true
		}
	}
	return false
}

func cloneUUID(value *uuid.UUID) *uuid.UUID {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
