// Package userinvitation binds administrative account invitations to a
// reviewed request, authenticated actor/session and durable command identity.
package userinvitation

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/huynhanx03/judgify/internal/application/workflow/admincommand"
	dto "github.com/huynhanx03/judgify/internal/identity/contract"
)

const (
	confirmationAction       = "create"
	resendConfirmationAction = "resend"
	confirmationResource     = "user-invitation"
	initialUserVersion       = int64(1)
)

var ErrInvalidWorkflow = errors.New("user invitation: invalid workflow")

type Config struct {
	RecentAuthenticationTTL time.Duration
}

type Request struct {
	dto.ProfileSchemaEvidence
	CommandID         uuid.UUID
	Username          string
	RecoveryEmail     string
	Roles             []string
	Reason            string
	ConfirmationToken string
	ActorID           uuid.UUID
	SessionID         uuid.UUID
}

type ResendRequest struct {
	UserID                    uuid.UUID
	CommandID                 uuid.UUID
	ExpectedUserVersion       int64
	ExpectedInvitationVersion uint64
	Reason                    string
	ConfirmationToken         string
	ActorID                   uuid.UUID
	SessionID                 uuid.UUID
}

type commands interface {
	PreviewInvitation(
		context.Context,
		*dto.UserInvitationCommandRequest,
	) (*dto.UserInvitationPreview, error)
	ApplyInvitation(
		context.Context,
		*dto.UserInvitationCommandRequest,
		[sha256.Size]byte,
	) (*dto.UserInvitationReceipt, error)
	PreviewInvitationResend(
		context.Context,
		*dto.UserInvitationResendCommandRequest,
	) (*dto.UserInvitationResendPreview, error)
	ApplyInvitationResend(
		context.Context,
		*dto.UserInvitationResendCommandRequest,
		[sha256.Size]byte,
	) (*dto.UserInvitationResendReceipt, error)
}

func (workflow *Workflow) PreviewResend(
	ctx context.Context,
	request ResendRequest,
) (*dto.UserInvitationResendPreviewResponse, error) {
	request, err := workflow.prepareResend(ctx, request, false)
	if err != nil {
		return nil, err
	}
	preview, err := workflow.commands.PreviewInvitationResend(
		ctx,
		resendCommandRequest(request, false),
	)
	if err != nil {
		return nil, err
	}
	if !validResendPreview(preview, request) {
		return nil, ErrInvalidWorkflow
	}
	request.ExpectedInvitationVersion = preview.ExpectedInvitationVersion
	expectation, err := resendExpectation(request)
	if err != nil {
		return nil, err
	}
	token, claims, err := workflow.confirmations.IssueWithClaims(expectation)
	if err != nil {
		return nil, err
	}
	return &dto.UserInvitationResendPreviewResponse{
		Preview: *preview, ConfirmationToken: token,
		ExpiresAt: claims.ExpiresAt.UTC(),
	}, nil
}

func (workflow *Workflow) ApplyResend(
	ctx context.Context,
	request ResendRequest,
) (*dto.UserInvitationResendReceipt, error) {
	request, err := workflow.prepareResend(ctx, request, true)
	if err != nil {
		return nil, err
	}
	expectation, err := resendExpectation(request)
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
	receipt, err := workflow.commands.ApplyInvitationResend(
		ctx,
		resendCommandRequest(request, true),
		expectation.RequestHash,
	)
	if err != nil {
		return nil, err
	}
	if !validResendReceipt(receipt, request) {
		return nil, ErrInvalidWorkflow
	}
	return receipt, nil
}

func (workflow *Workflow) prepareResend(
	ctx context.Context,
	request ResendRequest,
	requireConfirmation bool,
) (ResendRequest, error) {
	if workflow == nil || workflow.commands == nil || workflow.confirmations == nil ||
		ctx == nil || request.UserID == uuid.Nil || request.ActorID == uuid.Nil ||
		request.SessionID == uuid.Nil || request.ExpectedUserVersion < 1 ||
		request.ExpectedInvitationVersion >= uint64(math.MaxInt64) ||
		(requireConfirmation && request.ExpectedInvitationVersion == 0) {
		return ResendRequest{}, ErrInvalidWorkflow
	}
	meta, err := admincommand.NormalizeCommandMeta(
		admincommand.CommandMeta{
			ActorID: request.ActorID, SessionID: request.SessionID,
			ExpectedVersion: request.ExpectedUserVersion,
			IdempotencyKey:  request.CommandID, Reason: request.Reason,
			ConfirmationToken: request.ConfirmationToken,
		},
		admincommand.CommandMetaPolicy{
			RequireIdempotency: true, RequireConfirmation: requireConfirmation,
		},
	)
	if err != nil {
		return ResendRequest{}, err
	}
	request.Reason = meta.Reason
	request.ConfirmationToken = meta.ConfirmationToken
	return request, nil
}

func resendExpectation(
	request ResendRequest,
) (admincommand.ConfirmationExpectation, error) {
	document, err := json.Marshal(struct {
		ActorID                   uuid.UUID `json:"actor_id"`
		SessionID                 uuid.UUID `json:"session_id"`
		CommandID                 uuid.UUID `json:"command_id"`
		UserID                    uuid.UUID `json:"user_id"`
		ExpectedUserVersion       int64     `json:"expected_user_version"`
		ExpectedInvitationVersion uint64    `json:"expected_invitation_version"`
		Reason                    string    `json:"reason"`
	}{
		ActorID: request.ActorID, SessionID: request.SessionID,
		CommandID: request.CommandID, UserID: request.UserID,
		ExpectedUserVersion:       request.ExpectedUserVersion,
		ExpectedInvitationVersion: request.ExpectedInvitationVersion,
		Reason:                    request.Reason,
	})
	if err != nil {
		return admincommand.ConfirmationExpectation{}, ErrInvalidWorkflow
	}
	hash, err := admincommand.ConfirmationRequestHash(
		"identity.user.invitation.resend.v1",
		document,
	)
	if err != nil || hash == ([sha256.Size]byte{}) {
		return admincommand.ConfirmationExpectation{}, ErrInvalidWorkflow
	}
	return admincommand.ConfirmationExpectation{
		ActorID: request.ActorID, SessionID: request.SessionID,
		IdempotencyKey: request.CommandID,
		Action:         resendConfirmationAction, Resource: confirmationResource,
		TargetID:        request.UserID.String(),
		ExpectedVersion: request.ExpectedUserVersion,
		RequestHash:     hash,
	}, nil
}

func resendCommandRequest(
	request ResendRequest,
	includeConfirmation bool,
) *dto.UserInvitationResendCommandRequest {
	confirmation := ""
	if includeConfirmation {
		confirmation = request.ConfirmationToken
	}
	return &dto.UserInvitationResendCommandRequest{
		ID: request.UserID, CommandID: request.CommandID,
		ExpectedUserVersion:       request.ExpectedUserVersion,
		ExpectedInvitationVersion: request.ExpectedInvitationVersion,
		Reason:                    request.Reason, ConfirmationToken: confirmation,
	}
}

func validResendPreview(
	preview *dto.UserInvitationResendPreview,
	request ResendRequest,
) bool {
	return preview != nil && preview.User.ID == request.UserID &&
		preview.User.Status == "invited" &&
		preview.User.Version == request.ExpectedUserVersion &&
		preview.User.SuspendedAt == nil && preview.MaskedDestination != "" &&
		preview.ExpectedInvitationVersion > 0 &&
		preview.ExpectedInvitationVersion < uint64(math.MaxInt64) &&
		preview.ResultingInvitationVersion == preview.ExpectedInvitationVersion+1 &&
		preview.ResultingInvitationVersion <= uint64(math.MaxInt64) &&
		!preview.CurrentExpiresAt.IsZero() &&
		(request.ExpectedInvitationVersion == 0 ||
			request.ExpectedInvitationVersion == preview.ExpectedInvitationVersion)
}

func validResendReceipt(
	receipt *dto.UserInvitationResendReceipt,
	request ResendRequest,
) bool {
	return receipt != nil && receipt.CommandID == request.CommandID &&
		receipt.EventID == request.CommandID && receipt.User.ID == request.UserID &&
		receipt.User.Status == "invited" &&
		receipt.User.Version == request.ExpectedUserVersion &&
		receipt.User.SuspendedAt == nil &&
		request.ExpectedInvitationVersion < uint64(math.MaxInt64) &&
		receipt.InvitationVersion == request.ExpectedInvitationVersion+1 &&
		receipt.InvitationExpiresAt.After(receipt.CommittedAt) &&
		receipt.DeliveryState == "queued"
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
	commands      commands
	confirmations confirmations
	config        Config
	now           func() time.Time
}

func New(
	invitationCommands commands,
	confirmationProtector confirmations,
	config Config,
) (*Workflow, error) {
	if invitationCommands == nil || confirmationProtector == nil ||
		config.RecentAuthenticationTTL < time.Minute ||
		config.RecentAuthenticationTTL > time.Hour {
		return nil, ErrInvalidWorkflow
	}
	return &Workflow{
		commands: invitationCommands, confirmations: confirmationProtector,
		config: config, now: func() time.Time { return time.Now().UTC() },
	}, nil
}

func (workflow *Workflow) Preview(
	ctx context.Context,
	request Request,
) (*dto.UserInvitationPreviewResponse, error) {
	request, expectation, err := workflow.prepare(ctx, request, false)
	if err != nil {
		return nil, err
	}
	preview, err := workflow.commands.PreviewInvitation(
		ctx,
		commandRequest(request, false),
	)
	if err != nil {
		return nil, err
	}
	if !validPreview(preview, request) {
		return nil, ErrInvalidWorkflow
	}
	token, claims, err := workflow.confirmations.IssueWithClaims(expectation)
	if err != nil {
		return nil, err
	}
	return &dto.UserInvitationPreviewResponse{
		Preview: *preview, ConfirmationToken: token,
		ExpiresAt: claims.ExpiresAt.UTC(),
	}, nil
}

func (workflow *Workflow) Apply(
	ctx context.Context,
	request Request,
) (*dto.UserInvitationReceipt, error) {
	request, expectation, err := workflow.prepare(ctx, request, true)
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
	receipt, err := workflow.commands.ApplyInvitation(
		ctx,
		commandRequest(request, true),
		expectation.RequestHash,
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
	ctx context.Context,
	request Request,
	requireConfirmation bool,
) (Request, admincommand.ConfirmationExpectation, error) {
	if workflow == nil || workflow.commands == nil || workflow.confirmations == nil ||
		ctx == nil || request.ActorID == uuid.Nil || request.SessionID == uuid.Nil {
		return Request{}, admincommand.ConfirmationExpectation{}, ErrInvalidWorkflow
	}
	meta, err := admincommand.NormalizeCommandMeta(
		admincommand.CommandMeta{
			ActorID: request.ActorID, SessionID: request.SessionID,
			ExpectedVersion: initialUserVersion,
			IdempotencyKey:  request.CommandID, Reason: request.Reason,
			ConfirmationToken: request.ConfirmationToken,
		},
		admincommand.CommandMetaPolicy{
			RequireIdempotency: true, RequireConfirmation: requireConfirmation,
		},
	)
	if err != nil {
		return Request{}, admincommand.ConfirmationExpectation{}, err
	}
	request.Username = strings.TrimSpace(request.Username)
	request.RecoveryEmail = strings.TrimSpace(request.RecoveryEmail)
	request.Roles, err = canonicalRoles(request.Roles)
	if err != nil || request.Username == "" || request.RecoveryEmail == "" {
		return Request{}, admincommand.ConfirmationExpectation{}, ErrInvalidWorkflow
	}
	request.ProfileValues, err = canonicalProfileValues(request.ProfileValues)
	if err != nil {
		return Request{}, admincommand.ConfirmationExpectation{}, err
	}
	request.Reason = meta.Reason
	request.ConfirmationToken = meta.ConfirmationToken
	document, err := json.Marshal(struct {
		ActorID               uuid.UUID               `json:"actor_id"`
		SessionID             uuid.UUID               `json:"session_id"`
		CommandID             uuid.UUID               `json:"command_id"`
		Username              string                  `json:"username"`
		RecoveryEmail         string                  `json:"recovery_email"`
		Roles                 []string                `json:"roles"`
		ProfileSchemaRevision int64                   `json:"profile_schema_revision"`
		ProfileSchemaChecksum string                  `json:"profile_schema_checksum"`
		ProfileValues         []dto.ProfileValueInput `json:"profile_values"`
		Reason                string                  `json:"reason"`
	}{
		ActorID: request.ActorID, SessionID: request.SessionID,
		CommandID: request.CommandID, Username: request.Username,
		RecoveryEmail: request.RecoveryEmail, Roles: request.Roles,
		ProfileSchemaRevision: request.ProfileSchemaRevision,
		ProfileSchemaChecksum: strings.ToLower(strings.TrimSpace(request.ProfileSchemaChecksum)),
		ProfileValues:         request.ProfileValues, Reason: request.Reason,
	})
	if err != nil {
		return Request{}, admincommand.ConfirmationExpectation{}, ErrInvalidWorkflow
	}
	hash, err := admincommand.ConfirmationRequestHash(
		"identity.user.invitation.create.v1",
		document,
	)
	if err != nil || hash == ([sha256.Size]byte{}) {
		return Request{}, admincommand.ConfirmationExpectation{}, ErrInvalidWorkflow
	}
	return request, admincommand.ConfirmationExpectation{
		ActorID: meta.ActorID, SessionID: meta.SessionID,
		IdempotencyKey: meta.IdempotencyKey,
		Action:         confirmationAction, Resource: confirmationResource,
		TargetID: request.Username, ExpectedVersion: initialUserVersion,
		RequestHash: hash,
	}, nil
}

func commandRequest(
	request Request,
	includeConfirmation bool,
) *dto.UserInvitationCommandRequest {
	confirmation := ""
	if includeConfirmation {
		confirmation = request.ConfirmationToken
	}
	return &dto.UserInvitationCommandRequest{
		ProfileSchemaEvidence: request.ProfileSchemaEvidence,
		CommandID:             request.CommandID, Username: request.Username,
		RecoveryEmail: request.RecoveryEmail,
		Roles:         append([]string(nil), request.Roles...),
		Reason:        request.Reason, ConfirmationToken: confirmation,
	}
}

func canonicalRoles(values []string) ([]string, error) {
	if len(values) == 0 || len(values) > 64 {
		return nil, ErrInvalidWorkflow
	}
	result := make([]string, len(values))
	for index, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || len(value) > 50 {
			return nil, ErrInvalidWorkflow
		}
		result[index] = value
	}
	sort.Strings(result)
	for index := 1; index < len(result); index++ {
		if result[index] == result[index-1] {
			return nil, ErrInvalidWorkflow
		}
	}
	return result, nil
}

func canonicalProfileValues(
	values []dto.ProfileValueInput,
) ([]dto.ProfileValueInput, error) {
	if len(values) == 0 || len(values) > 128 {
		return nil, ErrInvalidWorkflow
	}
	result := make([]dto.ProfileValueInput, len(values))
	for index, value := range values {
		if value.DefinitionID == uuid.Nil || value.DefinitionRevisionID == uuid.Nil ||
			len(value.Value) == 0 {
			return nil, ErrInvalidWorkflow
		}
		result[index] = value
		result[index].Value = append([]byte(nil), value.Value...)
	}
	sort.Slice(result, func(left, right int) bool {
		return bytes.Compare(result[left].DefinitionID[:], result[right].DefinitionID[:]) < 0
	})
	for index := 1; index < len(result); index++ {
		if result[index].DefinitionID == result[index-1].DefinitionID {
			return nil, ErrInvalidWorkflow
		}
	}
	return result, nil
}

func validPreview(preview *dto.UserInvitationPreview, request Request) bool {
	return preview != nil && preview.Username == request.Username &&
		preview.MaskedDestination != "" &&
		len(preview.Roles) == len(request.Roles) &&
		sameStrings(preview.Roles, request.Roles) &&
		preview.ProfileValueCount == len(request.ProfileValues)
}

func validReceipt(receipt *dto.UserInvitationReceipt, request Request) bool {
	return receipt != nil && receipt.CommandID == request.CommandID &&
		receipt.EventID == request.CommandID && receipt.User.ID != uuid.Nil &&
		receipt.User.Username == request.Username && receipt.User.Status == "invited" &&
		receipt.User.Version == initialUserVersion &&
		sameStrings(receipt.Roles, request.Roles) &&
		receipt.ProfileValueCount == len(request.ProfileValues) &&
		receipt.InvitationExpiresAt.After(receipt.CommittedAt) &&
		receipt.DeliveryState == "queued"
}

func sameStrings(left, right []string) bool {
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
