package infrastructure

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/huynhanx03/go-common/pkg/logger"
	commonoutbox "github.com/huynhanx03/go-common/pkg/mq/outbox"

	identitycontract "github.com/huynhanx03/judgify/internal/identity/contract"
	identitydto "github.com/huynhanx03/judgify/internal/identity/contract"
)

const identityAdministrationObservationHandler = "observation.identity-administration"

const identityProfilePrivacyObservationHandler = "observation.identity-profile-privacy"

type identityRoleMetadataObservation struct {
	Action      string                   `json:"action"`
	CommandID   string                   `json:"command_id"`
	RequestHash string                   `json:"request_hash"`
	AuditID     string                   `json:"audit_id"`
	Role        identitydto.RoleResponse `json:"role"`
}

// identityAttributeObservation is the closed union of the five attribute
// administration payloads. Optional fields are validated per event below;
// using the real response DTOs keeps strict JSON decoding aligned with the
// producer instead of silently accepting an arbitrary nested document.
type identityAttributeObservation struct {
	Status              string                                           `json:"status,omitempty"`
	Action              string                                           `json:"action,omitempty"`
	CommandID           string                                           `json:"command_id"`
	RequestHash         string                                           `json:"request_hash,omitempty"`
	AuditID             string                                           `json:"audit_id,omitempty"`
	DefinitionID        string                                           `json:"definition_id"`
	RevisionID          string                                           `json:"revision_id,omitempty"`
	OperationID         string                                           `json:"operation_id,omitempty"`
	ExpectedVersion     int64                                            `json:"expected_version,omitempty"`
	ResultingVersion    int64                                            `json:"resulting_version,omitempty"`
	Version             int64                                            `json:"version,omitempty"`
	SourceRevisionID    string                                           `json:"source_revision_id,omitempty"`
	CandidateRevisionID string                                           `json:"candidate_revision_id,omitempty"`
	SourceValueCount    int64                                            `json:"source_value_count,omitempty"`
	TargetValueCount    int64                                            `json:"target_value_count,omitempty"`
	Definition          *identitydto.AttributeDefinitionResponse         `json:"definition,omitempty"`
	ActiveRevision      *identitydto.AttributeDefinitionRevisionResponse `json:"active_revision,omitempty"`
	presentFields       map[string]struct{}                              `json:"-"`
}

func (event *identityAttributeObservation) UnmarshalJSON(data []byte) error {
	type wire identityAttributeObservation
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var decoded wire
	if err := decoder.Decode(&decoded); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return ErrInvalidEventDelivery
	}

	presenceDecoder := json.NewDecoder(bytes.NewReader(data))
	token, err := presenceDecoder.Token()
	if err != nil || token != json.Delim('{') {
		return ErrInvalidEventDelivery
	}
	present := make(map[string]struct{})
	for presenceDecoder.More() {
		keyToken, keyErr := presenceDecoder.Token()
		key, ok := keyToken.(string)
		if keyErr != nil || !ok {
			return ErrInvalidEventDelivery
		}
		if _, duplicate := present[key]; duplicate {
			return ErrInvalidEventDelivery
		}
		present[key] = struct{}{}
		var raw json.RawMessage
		if err := presenceDecoder.Decode(&raw); err != nil {
			return err
		}
	}
	if token, err = presenceDecoder.Token(); err != nil || token != json.Delim('}') {
		return ErrInvalidEventDelivery
	}
	if err := presenceDecoder.Decode(&struct{}{}); err != io.EOF {
		return ErrInvalidEventDelivery
	}

	*event = identityAttributeObservation(decoded)
	event.presentFields = present
	return nil
}

func (event identityAttributeObservation) hasAny(fields ...string) bool {
	for _, field := range fields {
		if _, ok := event.presentFields[field]; ok {
			return true
		}
	}
	return false
}

func identityObservationRoutes() []commonoutbox.Route {
	routes := make([]commonoutbox.Route, 0, 15)
	for _, definition := range []struct {
		eventType string
		action    string
	}{
		{eventType: "identity.role.metadata.created.v1", action: "create"},
		{eventType: "identity.role.metadata.updated.v1", action: "update"},
		{eventType: "identity.role.metadata.archived.v1", action: "archive"},
	} {
		definition := definition
		routes = append(routes, strictJSONRoute(
			definition.eventType,
			identityAdministrationObservationHandler,
			8<<10,
			func(message commonoutbox.Message, event identityRoleMetadataObservation) error {
				return validateRoleMetadataObservation(
					message,
					definition.eventType,
					definition.action,
					event,
				)
			},
			func(ctx context.Context, event identityRoleMetadataObservation) error {
				logger.FromContext(ctx).Debug(
					"identity role metadata fact observed",
					zap.String("event_type", definition.eventType),
					zap.String("action", event.Action),
					zap.Int64("role_version", event.Role.Version),
				)
				return nil
			},
			nil,
		))
	}
	routes = append(routes,
		strictJSONRoute(
			identitycontract.EventUserInvitationCreatedV1,
			identityAdministrationObservationHandler,
			32<<10,
			validateUserInvitationCreatedObservation,
			func(ctx context.Context, event identitycontract.UserInvitationCreatedV1) error {
				logger.FromContext(ctx).Debug(
					"identity user invitation fact observed",
					zap.String("user_id", event.User.UserID),
					zap.String("username", event.User.Username),
					zap.Int("role_count", len(event.Roles)),
					zap.Int("profile_attribute_count", event.ProfileValueCount),
				)
				return nil
			},
			nil,
		),
		strictJSONRoute(
			identitycontract.EventUserInvitationResentV1,
			identityAdministrationObservationHandler,
			16<<10,
			validateUserInvitationResentObservation,
			func(ctx context.Context, event identitycontract.UserInvitationResentV1) error {
				logger.FromContext(ctx).Debug(
					"identity user invitation resend observed",
					zap.String("user_id", event.User.UserID),
					zap.Uint64("invitation_version", event.ResultingInvitationVersion),
				)
				return nil
			},
			nil,
		),
		strictJSONRoute(
			identitycontract.EventUserInvitationAcceptedV1,
			identityAdministrationObservationHandler,
			16<<10,
			validateUserInvitationAcceptedObservation,
			func(ctx context.Context, event identitycontract.UserInvitationAcceptedV1) error {
				logger.FromContext(ctx).Debug(
					"identity user invitation acceptance observed",
					zap.String("user_id", event.UserID),
					zap.Int64("version", event.After.Version),
				)
				return nil
			},
			nil,
		),
	)
	for _, definition := range []struct {
		eventType string
		action    string
	}{
		{eventType: identitycontract.EventUserSuspendedV1, action: "suspend"},
		{eventType: identitycontract.EventUserReactivatedV1, action: "reactivate"},
		{eventType: identitycontract.EventUserDeletedV1, action: "delete"},
	} {
		definition := definition
		routes = append(routes, strictJSONRoute(
			definition.eventType,
			identityAdministrationObservationHandler,
			16<<10,
			func(
				message commonoutbox.Message,
				event identitycontract.UserLifecycleChangedV1,
			) error {
				return validateUserLifecycleObservation(
					message,
					definition.eventType,
					definition.action,
					event,
				)
			},
			func(
				ctx context.Context,
				event identitycontract.UserLifecycleChangedV1,
			) error {
				logger.FromContext(ctx).Debug(
					"identity user lifecycle fact observed",
					zap.String("action", event.Action),
					zap.String("user_id", event.After.UserID),
					zap.Int64("version", event.ResultingVersion),
				)
				return nil
			},
			nil,
		))
	}
	for _, definition := range []struct {
		eventType string
		kind      identityAttributeObservationKind
	}{
		{eventType: "identity.attribute_definition.created.v1", kind: attributeObservationCreated},
		{eventType: "identity.attribute_definition.draft_created.v1", kind: attributeObservationDraft},
		{eventType: "identity.attribute_definition.migration_started.v1", kind: attributeObservationMigration},
		{eventType: "identity.attribute_definition.activated.v1", kind: attributeObservationActivated},
		{eventType: "identity.attribute_definition.archived.v1", kind: attributeObservationArchived},
	} {
		definition := definition
		routes = append(routes, strictJSONRoute(
			definition.eventType,
			identityAdministrationObservationHandler,
			64<<10,
			func(message commonoutbox.Message, event identityAttributeObservation) error {
				return validateAttributeObservation(
					message,
					definition.eventType,
					definition.kind,
					event,
				)
			},
			func(ctx context.Context, event identityAttributeObservation) error {
				logger.FromContext(ctx).Debug(
					"identity attribute definition fact observed",
					zap.String("event_type", definition.eventType),
					zap.String("action", event.Action),
					zap.Int64("version", observationVersion(event)),
				)
				return nil
			},
			nil,
		))
	}
	routes = append(routes, strictJSONRoute(
		identitycontract.EventProfileVisibilityChangedV1,
		identityProfilePrivacyObservationHandler,
		8<<10,
		validateProfilePrivacyObservation,
		func(ctx context.Context, event identitycontract.ProfileVisibilityChangedV1) error {
			logger.FromContext(ctx).Debug(
				"identity profile privacy fact observed",
				zap.String("user_id", event.ActorID),
				zap.String("visibility", event.State.Visibility),
				zap.Int64("profile_visibility_version", event.State.Version),
			)
			return nil
		},
		nil,
	))
	return routes
}

func validateUserInvitationResentObservation(
	message commonoutbox.Message,
	event identitycontract.UserInvitationResentV1,
) error {
	commandID, commandErr := canonicalMessageUUID(event.CommandID)
	auditID, auditErr := canonicalMessageUUID(event.AuditID)
	actorID, actorErr := canonicalMessageUUID(event.ActorID)
	sessionID, sessionErr := canonicalMessageUUID(event.ActorSessionID)
	userID, userErr := canonicalMessageUUID(event.User.UserID)
	if commandErr != nil || auditErr != nil || actorErr != nil || sessionErr != nil ||
		userErr != nil || commandID == uuid.Nil || auditID == uuid.Nil ||
		actorID == uuid.Nil || sessionID == uuid.Nil || userID == uuid.Nil ||
		actorID == userID || commandID != canonicalUUID(message.ID) ||
		event.Version != identitycontract.UserInvitationPayloadVersionV1 ||
		!validAttributeObservationHash(event.RequestHash) ||
		event.User.Username == "" ||
		strings.TrimSpace(event.User.Username) != event.User.Username ||
		event.User.Status != "invited" || event.User.Version < 1 ||
		event.User.SuspendedAt != nil || event.ExpectedInvitationVersion < 1 ||
		event.ExpectedInvitationVersion >= 1<<63-1 ||
		event.ResultingInvitationVersion != event.ExpectedInvitationVersion+1 ||
		event.ResultingInvitationVersion > 1<<63-1 ||
		event.InvitationExpiresAt.IsZero() {
		return ErrInvalidEventDelivery
	}
	if err := validateObservedEnvelope(
		message,
		identitycontract.EventUserInvitationResentV1,
		identitycontract.AggregateUserInvitation,
		userID,
		int64(event.ResultingInvitationVersion),
		"",
		time.Time{},
	); err != nil {
		return err
	}
	occurredText, err := uniqueMessageAttribute(message, "occurred_at")
	if err != nil {
		return ErrInvalidEventDelivery
	}
	occurredAt, err := time.Parse(time.RFC3339Nano, occurredText)
	if err != nil || !event.InvitationExpiresAt.After(occurredAt) {
		return ErrInvalidEventDelivery
	}
	return nil
}

func validateUserInvitationCreatedObservation(
	message commonoutbox.Message,
	event identitycontract.UserInvitationCreatedV1,
) error {
	commandID, commandErr := canonicalMessageUUID(event.CommandID)
	auditID, auditErr := canonicalMessageUUID(event.AuditID)
	actorID, actorErr := canonicalMessageUUID(event.ActorID)
	sessionID, sessionErr := canonicalMessageUUID(event.ActorSessionID)
	userID, userErr := canonicalMessageUUID(event.User.UserID)
	if commandErr != nil || auditErr != nil || actorErr != nil || sessionErr != nil ||
		userErr != nil || commandID == uuid.Nil || auditID == uuid.Nil ||
		actorID == uuid.Nil || sessionID == uuid.Nil || userID == uuid.Nil ||
		actorID == userID || commandID != canonicalUUID(message.ID) ||
		event.Version != identitycontract.UserInvitationPayloadVersionV1 ||
		!validAttributeObservationHash(event.RequestHash) ||
		event.User.Username == "" ||
		strings.TrimSpace(event.User.Username) != event.User.Username ||
		event.User.Status != "invited" || event.User.Version != 1 ||
		event.User.SuspendedAt != nil || !validObservedInvitationRoles(event.Roles) ||
		event.ProfileValueCount < 1 || event.ProfileValueCount > 128 ||
		event.InvitationExpiresAt.IsZero() {
		return ErrInvalidEventDelivery
	}
	if err := validateObservedEnvelope(
		message,
		identitycontract.EventUserInvitationCreatedV1,
		identitycontract.AggregateUser,
		userID,
		1,
		"",
		time.Time{},
	); err != nil {
		return err
	}
	occurredText, err := uniqueMessageAttribute(message, "occurred_at")
	if err != nil {
		return ErrInvalidEventDelivery
	}
	occurredAt, err := time.Parse(time.RFC3339Nano, occurredText)
	if err != nil || !event.InvitationExpiresAt.After(occurredAt) {
		return ErrInvalidEventDelivery
	}
	return nil
}

func validateUserInvitationAcceptedObservation(
	message commonoutbox.Message,
	event identitycontract.UserInvitationAcceptedV1,
) error {
	auditID, auditErr := canonicalMessageUUID(event.AuditID)
	userID, userErr := canonicalMessageUUID(event.UserID)
	beforeID, beforeErr := canonicalMessageUUID(event.Before.UserID)
	afterID, afterErr := canonicalMessageUUID(event.After.UserID)
	if auditErr != nil || userErr != nil || beforeErr != nil || afterErr != nil ||
		auditID == uuid.Nil || userID == uuid.Nil || beforeID != userID || afterID != userID ||
		event.Version != identitycontract.UserInvitationPayloadVersionV1 ||
		event.Before.Username == "" || event.Before.Username != event.After.Username ||
		strings.TrimSpace(event.Before.Username) != event.Before.Username ||
		event.Before.Status != "invited" || event.After.Status != "active" ||
		event.Before.Version < 1 || event.Before.Version == 1<<63-1 ||
		event.After.Version != event.Before.Version+1 ||
		event.Before.SuspendedAt != nil || event.After.SuspendedAt != nil ||
		event.AcceptedAt.IsZero() {
		return ErrInvalidEventDelivery
	}
	return validateObservedEnvelope(
		message,
		identitycontract.EventUserInvitationAcceptedV1,
		identitycontract.AggregateUser,
		userID,
		event.After.Version,
		"",
		event.AcceptedAt.UTC(),
	)
}

func validObservedInvitationRoles(values []string) bool {
	if len(values) == 0 || len(values) > 64 {
		return false
	}
	for index, value := range values {
		if len(value) < 2 || len(value) > 50 || strings.TrimSpace(value) != value ||
			(index > 0 && value <= values[index-1]) {
			return false
		}
	}
	return true
}

func validateUserLifecycleObservation(
	message commonoutbox.Message,
	eventType string,
	action string,
	event identitycontract.UserLifecycleChangedV1,
) error {
	commandID, commandErr := canonicalMessageUUID(event.CommandID)
	auditID, auditErr := canonicalMessageUUID(event.AuditID)
	actorID, actorErr := canonicalMessageUUID(event.ActorID)
	sessionID, sessionErr := canonicalMessageUUID(event.ActorSessionID)
	beforeID, beforeErr := canonicalMessageUUID(event.Before.UserID)
	afterID, afterErr := canonicalMessageUUID(event.After.UserID)
	if commandErr != nil || auditErr != nil || actorErr != nil ||
		sessionErr != nil || beforeErr != nil || afterErr != nil ||
		commandID == uuid.Nil || auditID == uuid.Nil || actorID == uuid.Nil ||
		sessionID == uuid.Nil || beforeID == uuid.Nil || afterID != beforeID ||
		actorID == beforeID || commandID != canonicalUUID(message.ID) ||
		event.Version != identitycontract.UserLifecyclePayloadVersionV1 ||
		event.Action != action || !validAttributeObservationHash(event.RequestHash) ||
		event.ExpectedVersion < 1 ||
		event.ResultingVersion != event.ExpectedVersion+1 ||
		event.Before.Version != event.ExpectedVersion ||
		event.After.Version != event.ResultingVersion ||
		strings.TrimSpace(event.Before.Username) != event.Before.Username ||
		event.Before.Username == "" ||
		event.After.Username != event.Before.Username ||
		!validObservedUserLifecycleTransition(action, event.Before, event.After) {
		return ErrInvalidEventDelivery
	}
	return validateObservedEnvelope(
		message,
		eventType,
		identitycontract.AggregateUser,
		beforeID,
		event.ResultingVersion,
		"",
		time.Time{},
	)
}

func validObservedUserLifecycleTransition(
	action string,
	before identitycontract.UserLifecycleStateV1,
	after identitycontract.UserLifecycleStateV1,
) bool {
	switch action {
	case "suspend":
		return before.Status == "active" && before.SuspendedAt == nil &&
			after.Status == "suspended" && after.SuspendedAt != nil &&
			!after.SuspendedAt.IsZero()
	case "reactivate":
		return before.Status == "suspended" && before.SuspendedAt != nil &&
			!before.SuspendedAt.IsZero() && after.Status == "active" &&
			after.SuspendedAt == nil
	case "delete":
		return (before.Status == "invited" || before.Status == "active" ||
			before.Status == "suspended") &&
			after.Status == "deleted" && after.SuspendedAt == nil &&
			(before.Status != "suspended" ||
				(before.SuspendedAt != nil && !before.SuspendedAt.IsZero()))
	default:
		return false
	}
}

func validateProfilePrivacyObservation(
	message commonoutbox.Message,
	event identitycontract.ProfileVisibilityChangedV1,
) error {
	actorID, actorErr := canonicalMessageUUID(event.ActorID)
	actorSessionID, sessionErr := canonicalMessageUUID(event.ActorSessionID)
	commandID, commandErr := canonicalMessageUUID(event.CommandID)
	auditID, auditErr := canonicalMessageUUID(event.AuditID)
	updatedAt, updatedErr := time.Parse(time.RFC3339Nano, event.State.UpdatedAt)
	if actorErr != nil || sessionErr != nil || commandErr != nil ||
		auditErr != nil || updatedErr != nil || actorID == uuid.Nil ||
		actorSessionID == uuid.Nil || commandID == uuid.Nil || auditID == uuid.Nil ||
		commandID != canonicalUUID(message.ID) ||
		event.Version != identitycontract.ProfileVisibilityPayloadVersionV1 ||
		!validAttributeObservationHash(event.RequestHash) ||
		event.ExpectedVersion < 1 ||
		event.ResultingVersion != event.ExpectedVersion+1 ||
		event.State.Version != event.ResultingVersion ||
		(event.State.Visibility != "private" && event.State.Visibility != "public") ||
		updatedAt.IsZero() ||
		updatedAt.UTC().Format(time.RFC3339Nano) != event.State.UpdatedAt {
		return ErrInvalidEventDelivery
	}
	return validateObservedEnvelope(
		message,
		identitycontract.EventProfileVisibilityChangedV1,
		identitycontract.AggregateUserProfilePrivacy,
		actorID,
		event.ResultingVersion,
		"",
		time.Time{},
	)
}

func validateRoleMetadataObservation(
	message commonoutbox.Message,
	eventType string,
	action string,
	event identityRoleMetadataObservation,
) error {
	commandID, err := canonicalMessageUUID(event.CommandID)
	if err != nil || commandID != canonicalUUID(message.ID) ||
		event.Action != action || !validObservationHash(event.RequestHash) ||
		event.Role.ID == uuid.Nil || event.Role.Version < 1 {
		return ErrInvalidEventDelivery
	}
	if _, err := canonicalMessageUUID(event.AuditID); err != nil {
		return ErrInvalidEventDelivery
	}
	return validateObservedEnvelope(
		message,
		eventType,
		"role",
		event.Role.ID,
		event.Role.Version,
		"",
		time.Time{},
	)
}

type identityAttributeObservationKind uint8

const (
	attributeObservationCreated identityAttributeObservationKind = iota + 1
	attributeObservationDraft
	attributeObservationMigration
	attributeObservationActivated
	attributeObservationArchived
)

func validateAttributeObservation(
	message commonoutbox.Message,
	eventType string,
	kind identityAttributeObservationKind,
	event identityAttributeObservation,
) error {
	commandID, err := canonicalMessageUUID(event.CommandID)
	if err != nil || commandID == uuid.Nil ||
		event.SourceValueCount < 0 || event.TargetValueCount < 0 {
		return ErrInvalidEventDelivery
	}
	definitionID, err := canonicalMessageUUID(event.DefinitionID)
	if err != nil || definitionID == uuid.Nil {
		return ErrInvalidEventDelivery
	}
	switch kind {
	case attributeObservationCreated:
		return validateCreatedAttributeObservation(message, eventType, commandID, definitionID, event)
	case attributeObservationDraft:
		return validateDraftAttributeObservation(message, eventType, commandID, definitionID, event)
	case attributeObservationMigration:
		return validateMigrationAttributeObservation(message, eventType, commandID, definitionID, event)
	case attributeObservationActivated, attributeObservationArchived:
		return validateCommittedAttributeObservation(
			message,
			eventType,
			kind,
			commandID,
			definitionID,
			event,
		)
	default:
		return ErrInvalidEventDelivery
	}
}

func validateCreatedAttributeObservation(
	message commonoutbox.Message,
	eventType string,
	commandID uuid.UUID,
	definitionID uuid.UUID,
	event identityAttributeObservation,
) error {
	revisionID, revisionErr := canonicalMessageUUID(event.RevisionID)
	if commandID != canonicalUUID(message.ID) || event.Action != "create" ||
		!validAttributeObservationHash(event.RequestHash) || event.Version != 1 ||
		event.ExpectedVersion != 0 || event.ResultingVersion != 0 ||
		event.Status != "" || event.OperationID != "" ||
		event.SourceRevisionID != "" || event.CandidateRevisionID != "" ||
		event.SourceValueCount != 0 || event.TargetValueCount != 0 ||
		event.hasAny(
			"status", "operation_id", "expected_version", "resulting_version",
			"source_revision_id", "candidate_revision_id", "source_value_count",
			"target_value_count", "active_revision",
		) ||
		event.Definition == nil || event.Definition.ID != definitionID ||
		event.Definition.Version != 1 || event.ActiveRevision != nil ||
		!validAttributeObservationUUID(event.AuditID) || revisionErr != nil ||
		revisionID == uuid.Nil ||
		event.Definition.DraftRevisionID == nil ||
		*event.Definition.DraftRevisionID != revisionID {
		return ErrInvalidEventDelivery
	}
	return validateObservedEnvelope(
		message,
		eventType,
		"attribute_definition",
		definitionID,
		1,
		"",
		time.Time{},
	)
}

func validateDraftAttributeObservation(
	message commonoutbox.Message,
	eventType string,
	commandID uuid.UUID,
	definitionID uuid.UUID,
	event identityAttributeObservation,
) error {
	revisionID, revisionErr := canonicalMessageUUID(event.RevisionID)
	if commandID != canonicalUUID(message.ID) || event.Action != "draft" ||
		!validAttributeObservationHash(event.RequestHash) || event.ExpectedVersion < 1 ||
		event.ResultingVersion != event.ExpectedVersion+1 || event.Version != 0 ||
		event.Status != "" || event.OperationID != "" ||
		event.SourceRevisionID != "" || event.CandidateRevisionID != "" ||
		event.SourceValueCount != 0 || event.TargetValueCount != 0 ||
		event.hasAny(
			"status", "version", "operation_id", "source_revision_id",
			"candidate_revision_id", "source_value_count", "target_value_count",
			"active_revision",
		) ||
		event.Definition == nil || event.Definition.ID != definitionID ||
		event.Definition.Version != event.ResultingVersion || event.ActiveRevision != nil ||
		!validAttributeObservationUUID(event.AuditID) || revisionErr != nil ||
		revisionID == uuid.Nil ||
		event.Definition.DraftRevisionID == nil ||
		*event.Definition.DraftRevisionID != revisionID {
		return ErrInvalidEventDelivery
	}
	return validateObservedEnvelope(
		message,
		eventType,
		"attribute_definition",
		definitionID,
		event.ResultingVersion,
		"",
		time.Time{},
	)
}

func validateMigrationAttributeObservation(
	message commonoutbox.Message,
	eventType string,
	commandID uuid.UUID,
	definitionID uuid.UUID,
	event identityAttributeObservation,
) error {
	operationID, operationErr := canonicalMessageUUID(event.OperationID)
	sourceID, sourceErr := canonicalMessageUUID(event.SourceRevisionID)
	candidateID, candidateErr := canonicalMessageUUID(event.CandidateRevisionID)
	if operationErr != nil || sourceErr != nil || candidateErr != nil ||
		operationID == uuid.Nil || sourceID == uuid.Nil || candidateID == uuid.Nil ||
		operationID != commandID || sourceID == candidateID ||
		event.ExpectedVersion < 1 ||
		(event.SourceValueCount == 0 && event.TargetValueCount == 0) ||
		event.TargetValueCount > (1<<63-1)-event.SourceValueCount ||
		event.Status != "" || event.Action != "" || event.RequestHash != "" ||
		event.AuditID != "" || event.RevisionID != "" ||
		event.ResultingVersion != 0 || event.Version != 0 ||
		event.hasAny(
			"status", "action", "request_hash", "audit_id", "revision_id",
			"resulting_version", "version", "definition", "active_revision",
		) ||
		event.Definition != nil || event.ActiveRevision != nil ||
		canonicalUUID(message.ID) != attributeMigrationStartedObservationID(commandID) {
		return ErrInvalidEventDelivery
	}
	_ = definitionID
	return validateObservedEnvelope(
		message,
		eventType,
		"attribute_migration",
		operationID,
		1,
		"",
		time.Time{},
	)
}

func validateCommittedAttributeObservation(
	message commonoutbox.Message,
	eventType string,
	kind identityAttributeObservationKind,
	commandID uuid.UUID,
	definitionID uuid.UUID,
	event identityAttributeObservation,
) error {
	action := "activate"
	if kind == attributeObservationArchived {
		action = "archive"
	}
	if commandID != canonicalUUID(message.ID) || event.Status != "committed" ||
		event.Action != action || !validAttributeObservationHash(event.RequestHash) ||
		event.ExpectedVersion < 1 || event.ResultingVersion != event.ExpectedVersion+1 ||
		event.RevisionID != "" || event.OperationID != "" || event.Version != 0 ||
		event.SourceRevisionID != "" || event.SourceValueCount != 0 ||
		event.TargetValueCount != 0 ||
		event.hasAny(
			"revision_id", "operation_id", "version", "source_revision_id",
			"source_value_count", "target_value_count",
		) ||
		event.Definition == nil || event.Definition.ID != definitionID ||
		event.Definition.Version != event.ResultingVersion ||
		!validAttributeObservationUUID(event.AuditID) {
		return ErrInvalidEventDelivery
	}
	if kind == attributeObservationActivated {
		candidateID, err := canonicalMessageUUID(event.CandidateRevisionID)
		if err != nil || candidateID == uuid.Nil || event.ActiveRevision == nil ||
			event.ActiveRevision.ID != candidateID ||
			event.ActiveRevision.DefinitionID != definitionID ||
			event.Definition.Status != "active" ||
			event.Definition.ActiveRevisionID == nil ||
			*event.Definition.ActiveRevisionID != candidateID {
			return ErrInvalidEventDelivery
		}
	} else if event.CandidateRevisionID != "" || event.ActiveRevision != nil ||
		event.hasAny("candidate_revision_id", "active_revision") ||
		event.Definition.Status != "archived" {
		return ErrInvalidEventDelivery
	}
	return validateObservedEnvelope(
		message,
		eventType,
		"attribute_definition",
		definitionID,
		event.ResultingVersion,
		"",
		time.Time{},
	)
}

func observationVersion(event identityAttributeObservation) int64 {
	if event.ResultingVersion > 0 {
		return event.ResultingVersion
	}
	return event.Version
}

func validObservationHash(value string) bool {
	decoded, err := hex.DecodeString(value)
	return len(value) == 64 && err == nil && len(decoded) == 32
}

func validAttributeObservationHash(value string) bool {
	decoded, err := hex.DecodeString(value)
	return len(value) == 64 && err == nil && len(decoded) == 32 &&
		hex.EncodeToString(decoded) == value
}

func validAttributeObservationUUID(value string) bool {
	id, err := canonicalMessageUUID(value)
	return err == nil && id != uuid.Nil
}

func canonicalUUID(value string) uuid.UUID {
	id, _ := uuid.Parse(value)
	return id
}

func attributeMigrationStartedObservationID(commandID uuid.UUID) uuid.UUID {
	return uuid.NewSHA1(
		uuid.NameSpaceURL,
		[]byte("judgify/identity/attribute-definition/migration-started/"+commandID.String()),
	)
}
