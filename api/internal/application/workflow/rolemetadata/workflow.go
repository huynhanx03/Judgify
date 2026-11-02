// Package rolemetadata binds reviewed role metadata mutations to the exact
// actor, browser session, idempotency command, optimistic version and payload.
package rolemetadata

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/huynhanx03/judgify/internal/application/workflow/admincommand"
	dto "github.com/huynhanx03/judgify/internal/identity/contract"
)

const (
	confirmationResource  = "role"
	metadataUpdateAction  = "update"
	metadataArchiveAction = "archive"
)

var ErrInvalidWorkflow = errors.New("role metadata: invalid workflow")

type Config struct {
	RecentAuthenticationTTL time.Duration
}

type Request struct {
	RoleID            uuid.UUID
	CommandID         uuid.UUID
	ExpectedVersion   int64
	Reason            string
	Name              *string
	Description       *string
	ConfirmationToken string
	ActorID           uuid.UUID
	SessionID         uuid.UUID
}

type commands interface {
	PreviewMetadata(
		context.Context,
		uuid.UUID,
		string,
		*dto.UpdateRoleRequest,
	) (*dto.RoleMetadataPreview, error)
	ApplyMetadata(
		context.Context,
		uuid.UUID,
		string,
		*dto.UpdateRoleRequest,
		[sha256.Size]byte,
	) (*dto.RoleMutationReceipt, error)
}

type confirmations interface {
	IssueWithClaims(
		admincommand.ConfirmationExpectation,
	) (string, admincommand.ConfirmationClaims, error)
	Verify(
		string,
		admincommand.ConfirmationExpectation,
	) (admincommand.ConfirmationClaims, error)
}

type Workflow struct {
	roles         commands
	confirmations confirmations
	config        Config
	now           func() time.Time
}

func New(
	roles commands,
	confirmations confirmations,
	config Config,
) (*Workflow, error) {
	if roles == nil || confirmations == nil ||
		config.RecentAuthenticationTTL < time.Minute ||
		config.RecentAuthenticationTTL > time.Hour {
		return nil, ErrInvalidWorkflow
	}
	return &Workflow{
		roles:         roles,
		confirmations: confirmations,
		config:        config,
		now:           func() time.Time { return time.Now().UTC() },
	}, nil
}

func (workflow *Workflow) PreviewUpdate(
	ctx context.Context,
	request Request,
) (*dto.RoleMetadataPreviewResponse, error) {
	return workflow.preview(ctx, metadataUpdateAction, request)
}

func (workflow *Workflow) PreviewArchive(
	ctx context.Context,
	request Request,
) (*dto.RoleMetadataPreviewResponse, error) {
	return workflow.preview(ctx, metadataArchiveAction, request)
}

func (workflow *Workflow) preview(
	ctx context.Context,
	action string,
	request Request,
) (*dto.RoleMetadataPreviewResponse, error) {
	request, expectation, err := workflow.prepare(ctx, action, request, false)
	if err != nil {
		return nil, err
	}
	preview, err := workflow.roles.PreviewMetadata(
		ctx,
		request.RoleID,
		action,
		metadataRequest(request, false),
	)
	if err != nil {
		return nil, err
	}
	if !validPreview(preview, action, request) {
		return nil, ErrInvalidWorkflow
	}
	token, claims, err := workflow.confirmations.IssueWithClaims(expectation)
	if err != nil {
		return nil, err
	}
	return &dto.RoleMetadataPreviewResponse{
		Preview:           *preview,
		ConfirmationToken: token,
		ExpiresAt:         claims.ExpiresAt.UTC(),
	}, nil
}

func (workflow *Workflow) ApplyUpdate(
	ctx context.Context,
	request Request,
) (*dto.RoleMutationReceipt, error) {
	return workflow.apply(ctx, metadataUpdateAction, request)
}

func (workflow *Workflow) ApplyArchive(
	ctx context.Context,
	request Request,
) (*dto.RoleMutationReceipt, error) {
	return workflow.apply(ctx, metadataArchiveAction, request)
}

func (workflow *Workflow) apply(
	ctx context.Context,
	action string,
	request Request,
) (*dto.RoleMutationReceipt, error) {
	request, expectation, err := workflow.prepare(ctx, action, request, true)
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
	receipt, err := workflow.roles.ApplyMetadata(
		ctx,
		request.RoleID,
		action,
		metadataRequest(request, true),
		expectation.RequestHash,
	)
	if err != nil {
		return nil, err
	}
	if !validReceipt(receipt, action, request) {
		return nil, ErrInvalidWorkflow
	}
	return receipt, nil
}

func (workflow *Workflow) prepare(
	ctx context.Context,
	action string,
	request Request,
	requireConfirmation bool,
) (Request, admincommand.ConfirmationExpectation, error) {
	if workflow == nil || workflow.roles == nil || workflow.confirmations == nil ||
		ctx == nil || request.RoleID == uuid.Nil || !validAction(action) {
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
	if !validMetadataInput(action, request.Name, request.Description) {
		return Request{}, admincommand.ConfirmationExpectation{}, ErrInvalidWorkflow
	}

	request.Reason = meta.Reason
	request.ConfirmationToken = meta.ConfirmationToken
	document, err := canonicalRequestDocument(request)
	if err != nil {
		return Request{}, admincommand.ConfirmationExpectation{}, ErrInvalidWorkflow
	}
	hash, err := admincommand.ConfirmationRequestHash(
		"identity.role.metadata."+action+".v1",
		document,
	)
	if err != nil || hash == ([sha256.Size]byte{}) {
		return Request{}, admincommand.ConfirmationExpectation{}, ErrInvalidWorkflow
	}
	return request, admincommand.ConfirmationExpectation{
		ActorID:         meta.ActorID,
		SessionID:       meta.SessionID,
		IdempotencyKey:  meta.IdempotencyKey,
		Action:          "identity.role.metadata." + action,
		Resource:        confirmationResource,
		TargetID:        request.RoleID.String(),
		ExpectedVersion: meta.ExpectedVersion,
		RequestHash:     hash,
	}, nil
}

func canonicalRequestDocument(request Request) ([]byte, error) {
	return json.Marshal(struct {
		ActorID         uuid.UUID `json:"actor_id"`
		SessionID       uuid.UUID `json:"session_id"`
		RoleID          uuid.UUID `json:"role_id"`
		CommandID       uuid.UUID `json:"command_id"`
		ExpectedVersion int64     `json:"expected_version"`
		Reason          string    `json:"reason"`
		Name            *string   `json:"name,omitempty"`
		Description     *string   `json:"description,omitempty"`
	}{
		ActorID:         request.ActorID,
		SessionID:       request.SessionID,
		RoleID:          request.RoleID,
		CommandID:       request.CommandID,
		ExpectedVersion: request.ExpectedVersion,
		Reason:          request.Reason,
		Name:            request.Name,
		Description:     request.Description,
	})
}

func metadataRequest(request Request, includeConfirmation bool) *dto.UpdateRoleRequest {
	confirmationToken := ""
	if includeConfirmation {
		confirmationToken = request.ConfirmationToken
	}
	return &dto.UpdateRoleRequest{
		ID:                request.RoleID,
		CommandID:         request.CommandID,
		ExpectedVersion:   request.ExpectedVersion,
		Reason:            request.Reason,
		Name:              request.Name,
		Description:       request.Description,
		ConfirmationToken: confirmationToken,
	}
}

func validAction(action string) bool {
	return action == metadataUpdateAction || action == metadataArchiveAction
}

func validMetadataInput(action string, name, description *string) bool {
	if action == metadataUpdateAction && name == nil && description == nil {
		return false
	}
	if action == metadataArchiveAction && (name != nil || description != nil) {
		return false
	}
	if name != nil && (strings.TrimSpace(*name) != *name || len(*name) < 2 || len(*name) > 50) {
		return false
	}
	return description == nil ||
		(strings.TrimSpace(*description) == *description && len(*description) <= 255)
}

func validPreview(
	preview *dto.RoleMetadataPreview,
	action string,
	request Request,
) bool {
	if preview == nil || preview.Action != action || preview.RoleID != request.RoleID ||
		preview.ExpectedVersion != request.ExpectedVersion ||
		preview.ResultingVersion != request.ExpectedVersion+1 ||
		!validProjectionPair(preview.Before, preview.After, request) {
		return false
	}

	switch action {
	case metadataUpdateAction:
		return preview.After.Status == "active" &&
			preview.After.Name == resultingName(preview.Before.Name, request.Name) &&
			preview.After.Description == resultingDescription(
				preview.Before.Description,
				request.Description,
			)
	case metadataArchiveAction:
		return preview.After.Status == "archived" &&
			preview.After.Name == preview.Before.Name &&
			preview.After.Description == preview.Before.Description
	default:
		return false
	}
}

func validProjectionPair(
	before dto.RoleResponse,
	after dto.RoleResponse,
	request Request,
) bool {
	return before.ID == request.RoleID && after.ID == request.RoleID &&
		before.Version == request.ExpectedVersion &&
		after.Version == request.ExpectedVersion+1 &&
		before.Key != "" && before.Key == after.Key &&
		before.Status == "active" &&
		!before.IsSystem && !after.IsSystem &&
		!before.IsProtected && !after.IsProtected
}

func validReceipt(
	receipt *dto.RoleMutationReceipt,
	action string,
	request Request,
) bool {
	if receipt == nil || receipt.CommandID != request.CommandID ||
		receipt.EventID != request.CommandID || receipt.Action != action ||
		receipt.Role.ID != request.RoleID ||
		receipt.Role.Version != request.ExpectedVersion+1 ||
		receipt.Role.Key == "" || receipt.Role.IsSystem || receipt.Role.IsProtected ||
		receipt.CommittedAt.IsZero() {
		return false
	}
	if action == metadataArchiveAction {
		return receipt.Role.Status == "archived"
	}
	return receipt.Role.Status == "active" &&
		(request.Name == nil || receipt.Role.Name == *request.Name) &&
		(request.Description == nil || receipt.Role.Description == *request.Description)
}

func resultingName(current string, requested *string) string {
	if requested == nil {
		return current
	}
	return *requested
}

func resultingDescription(current string, requested *string) string {
	if requested == nil {
		return current
	}
	return *requested
}
