// Package userlifecycle binds sensitive administrative account transitions to
// one reviewed target, actor, live session, command identity and optimistic
// security-state version.
package userlifecycle

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/huynhanx03/judgify/internal/application/workflow/admincommand"
	dto "github.com/huynhanx03/judgify/internal/identity/contract"
)

const confirmationResource = "user"

var ErrInvalidWorkflow = errors.New("user lifecycle: invalid workflow")

type Config struct {
	RecentAuthenticationTTL time.Duration
}

type Request struct {
	UserID            uuid.UUID
	CommandID         uuid.UUID
	ExpectedVersion   int64
	Reason            string
	ConfirmationToken string
	ActorID           uuid.UUID
	SessionID         uuid.UUID
}

type commands interface {
	PreviewLifecycle(
		context.Context,
		uuid.UUID,
		string,
		*dto.UserLifecycleCommandRequest,
	) (*dto.UserLifecyclePreview, error)
	ApplyLifecycle(
		context.Context,
		uuid.UUID,
		string,
		*dto.UserLifecycleCommandRequest,
		[sha256.Size]byte,
	) (*dto.UserLifecycleReceipt, error)
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
	users         commands
	confirmations confirmations
	config        Config
	now           func() time.Time
}

func New(
	users commands,
	confirmations confirmations,
	config Config,
) (*Workflow, error) {
	if users == nil || confirmations == nil ||
		config.RecentAuthenticationTTL < time.Minute ||
		config.RecentAuthenticationTTL > time.Hour {
		return nil, ErrInvalidWorkflow
	}
	return &Workflow{
		users:         users,
		confirmations: confirmations,
		config:        config,
		now:           func() time.Time { return time.Now().UTC() },
	}, nil
}

func (workflow *Workflow) Preview(
	ctx context.Context,
	action string,
	request Request,
) (*dto.UserLifecyclePreviewResponse, error) {
	request, expectation, err := workflow.prepare(ctx, action, request, false)
	if err != nil {
		return nil, err
	}
	preview, err := workflow.users.PreviewLifecycle(
		ctx,
		request.UserID,
		action,
		commandRequest(request, false),
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
	return &dto.UserLifecyclePreviewResponse{
		Preview:           *preview,
		ConfirmationToken: token,
		ExpiresAt:         claims.ExpiresAt.UTC(),
	}, nil
}

func (workflow *Workflow) Apply(
	ctx context.Context,
	action string,
	request Request,
) (*dto.UserLifecycleReceipt, error) {
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
	receipt, err := workflow.users.ApplyLifecycle(
		ctx,
		request.UserID,
		action,
		commandRequest(request, true),
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
	if workflow == nil || workflow.users == nil || workflow.confirmations == nil ||
		ctx == nil || request.UserID == uuid.Nil || !validAction(action) ||
		request.ActorID == request.UserID {
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
	document, err := json.Marshal(struct {
		ActorID         uuid.UUID `json:"actor_id"`
		SessionID       uuid.UUID `json:"session_id"`
		UserID          uuid.UUID `json:"user_id"`
		CommandID       uuid.UUID `json:"command_id"`
		Action          string    `json:"action"`
		ExpectedVersion int64     `json:"expected_version"`
		Reason          string    `json:"reason"`
	}{
		ActorID:         request.ActorID,
		SessionID:       request.SessionID,
		UserID:          request.UserID,
		CommandID:       request.CommandID,
		Action:          action,
		ExpectedVersion: request.ExpectedVersion,
		Reason:          request.Reason,
	})
	if err != nil {
		return Request{}, admincommand.ConfirmationExpectation{}, ErrInvalidWorkflow
	}
	hash, err := admincommand.ConfirmationRequestHash(
		"identity.user.lifecycle."+action+".v1",
		document,
	)
	if err != nil || hash == ([sha256.Size]byte{}) {
		return Request{}, admincommand.ConfirmationExpectation{}, ErrInvalidWorkflow
	}
	return request, admincommand.ConfirmationExpectation{
		ActorID:         meta.ActorID,
		SessionID:       meta.SessionID,
		IdempotencyKey:  meta.IdempotencyKey,
		Action:          "identity.user.lifecycle." + action,
		Resource:        confirmationResource,
		TargetID:        request.UserID.String(),
		ExpectedVersion: meta.ExpectedVersion,
		RequestHash:     hash,
	}, nil
}

func commandRequest(
	request Request,
	includeConfirmation bool,
) *dto.UserLifecycleCommandRequest {
	confirmation := ""
	if includeConfirmation {
		confirmation = request.ConfirmationToken
	}
	return &dto.UserLifecycleCommandRequest{
		ID:                request.UserID,
		CommandID:         request.CommandID,
		ExpectedVersion:   request.ExpectedVersion,
		Reason:            request.Reason,
		ConfirmationToken: confirmation,
	}
}

func validAction(action string) bool {
	return action == "suspend" || action == "reactivate" || action == "delete"
}

func validPreview(
	preview *dto.UserLifecyclePreview,
	action string,
	request Request,
) bool {
	if preview == nil || preview.Action != action ||
		preview.UserID != request.UserID ||
		preview.ExpectedVersion != request.ExpectedVersion ||
		preview.ResultingVersion != request.ExpectedVersion+1 ||
		!validProjectionPair(preview.Before, preview.After, request) {
		return false
	}
	if !validTransition(action, preview.Before.Status, preview.After.Status) {
		return false
	}
	switch action {
	case "suspend":
		return preview.Before.SuspendedAt == nil &&
			preview.After.SuspendedAt != nil &&
			!preview.After.SuspendedAt.IsZero()
	case "reactivate":
		return preview.Before.SuspendedAt != nil &&
			!preview.Before.SuspendedAt.IsZero() &&
			preview.After.SuspendedAt == nil
	case "delete":
		return preview.After.SuspendedAt == nil &&
			(preview.Before.Status != "suspended" ||
				(preview.Before.SuspendedAt != nil &&
					!preview.Before.SuspendedAt.IsZero()))
	default:
		return false
	}
}

func validProjectionPair(
	before dto.UserLifecycleProjection,
	after dto.UserLifecycleProjection,
	request Request,
) bool {
	return before.ID == request.UserID && after.ID == request.UserID &&
		before.Username != "" && before.Username == after.Username &&
		before.Version == request.ExpectedVersion &&
		after.Version == request.ExpectedVersion+1
}

func validReceipt(
	receipt *dto.UserLifecycleReceipt,
	action string,
	request Request,
) bool {
	if receipt == nil || receipt.CommandID != request.CommandID ||
		receipt.EventID != request.CommandID || receipt.Action != action ||
		receipt.User.ID != request.UserID || receipt.User.Username == "" ||
		receipt.User.Version != request.ExpectedVersion+1 ||
		receipt.CommittedAt.IsZero() {
		return false
	}
	switch action {
	case "suspend":
		return receipt.User.Status == "suspended" &&
			receipt.User.SuspendedAt != nil && !receipt.User.SuspendedAt.IsZero()
	case "reactivate":
		return receipt.User.Status == "active" && receipt.User.SuspendedAt == nil
	case "delete":
		return receipt.User.Status == "deleted" && receipt.User.SuspendedAt == nil
	default:
		return false
	}
}

func validTransition(action, before, after string) bool {
	switch action {
	case "suspend":
		return before == "active" && after == "suspended"
	case "reactivate":
		return before == "suspended" && after == "active"
	case "delete":
		return (before == "active" || before == "suspended") && after == "deleted"
	default:
		return false
	}
}
