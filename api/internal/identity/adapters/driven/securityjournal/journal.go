// Package securityjournal maps Identity's closed security-mutation catalog to
// the platform audit and outbox primitives. Both appends join the caller's
// physical transaction, so either both rows commit or the domain mutation is
// rolled back with them.
package securityjournal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/google/uuid"

	entity "github.com/huynhanx03/judgify/internal/identity/domain"
	"github.com/huynhanx03/judgify/internal/platform/audit"
	"github.com/huynhanx03/judgify/internal/platform/eventing"
	"github.com/huynhanx03/judgify/internal/ports"
)

var ErrInvalidSecurityMutation = errors.New("identity security journal: invalid mutation")

const (
	securityMutationAggregate = "security_mutation"
	securityMutationReason    = "security_mutation_committed"
)

type operation struct {
	eventType string
	resource  string
	status    string
}

var operations = map[entity.SecurityMutationKind]operation{
	entity.SecurityMutationUserRegistered: {
		eventType: "identity.user.registered.v1", resource: "user", status: "active",
	},
	entity.SecurityMutationOAuthUserRegistered: {
		eventType: "identity.user.registered.v1", resource: "user", status: "active",
	},
	entity.SecurityMutationOAuthIdentityLinked: {
		eventType: "identity.oauth.linked.v1", resource: "federated_identity", status: "linked",
	},
	entity.SecurityMutationOAuthIdentityUnlinked: {
		eventType: "identity.oauth.unlinked.v1", resource: "federated_identity", status: "unlinked",
	},
	entity.SecurityMutationRecoveryContactVerified: {
		eventType: "identity.recovery_contact.verified.v1", resource: "recovery_contact", status: "verified",
	},
	entity.SecurityMutationRecoveryContactReplacementStarted: {
		eventType: "identity.recovery_contact.replacement_started.v1", resource: "recovery_contact", status: "pending",
	},
	entity.SecurityMutationRecoveryContactVerificationResent: {
		eventType: "identity.recovery_contact.verification_resent.v1", resource: "recovery_contact", status: "pending",
	},
	entity.SecurityMutationRecoveryContactPendingCancelled: {
		eventType: "identity.recovery_contact.pending_cancelled.v1", resource: "recovery_contact", status: "revoked",
	},
	entity.SecurityMutationRecoveryContactRemoved: {
		eventType: "identity.recovery_contact.removed.v1", resource: "recovery_contact", status: "revoked",
	},
	entity.SecurityMutationPasswordResetConsumed: {
		eventType: "identity.password_reset.consumed.v1", resource: "user_security", status: "completed",
	},
	entity.SecurityMutationPasswordChanged: {
		eventType: "identity.password.changed.v1", resource: "user_security", status: "completed",
	},
	entity.SecurityMutationSessionRevoked: {
		eventType: "identity.session.revoked.v1", resource: "authentication_session", status: "revoked",
	},
	entity.SecurityMutationSessionsRevokedAll: {
		eventType: "identity.sessions.revoked_all.v1", resource: "user_security", status: "revoked",
	},
}

// Journal is the Identity adapter over transaction-bound platform persistence.
type Journal struct {
	audit  audit.Appender
	outbox eventing.OutboxAppender
}

func New(auditAppender audit.Appender, outbox eventing.OutboxAppender) (*Journal, error) {
	if nilDependency(auditAppender) || nilDependency(outbox) {
		return nil, ErrInvalidSecurityMutation
	}
	return &Journal{audit: auditAppender, outbox: outbox}, nil
}

var _ ports.SecurityMutationJournal = (*Journal)(nil)

type payload struct {
	MutationID          string    `json:"mutation_id"`
	UserID              string    `json:"user_id"`
	SessionID           string    `json:"session_id,omitempty"`
	TargetSessionID     string    `json:"target_session_id,omitempty"`
	ContactID           string    `json:"contact_id,omitempty"`
	IntentID            string    `json:"intent_id,omitempty"`
	FederatedIdentityID string    `json:"federated_identity_id,omitempty"`
	Provider            string    `json:"provider,omitempty"`
	Status              string    `json:"status"`
	Version             uint64    `json:"version,omitempty"`
	SecurityVersion     uint64    `json:"security_version,omitempty"`
	OccurredAt          time.Time `json:"occurred_at"`
}

// Append validates and prepares both records before touching persistence.
// Platform appenders independently require the same live transaction scope.
func (journal *Journal) Append(ctx context.Context, mutation entity.SecurityMutation) error {
	if journal == nil || nilDependency(journal.audit) || nilDependency(journal.outbox) ||
		ctx == nil || ctx.Err() != nil {
		return ErrInvalidSecurityMutation
	}
	entry, event, err := prepare(mutation)
	if err != nil {
		return err
	}
	if _, err := journal.audit.Append(ctx, entry); err != nil {
		return fmt.Errorf("append identity security audit: %w", err)
	}
	if err := journal.outbox.Append(ctx, event); err != nil {
		return fmt.Errorf("append identity security event: %w", err)
	}
	return nil
}

func prepare(mutation entity.SecurityMutation) (audit.Entry, eventing.Event, error) {
	operation, ok := operations[mutation.Kind]
	if !ok || !validMutation(mutation) {
		return audit.Entry{}, eventing.Event{}, ErrInvalidSecurityMutation
	}
	document := payload{
		MutationID:      mutation.ID.String(),
		UserID:          mutation.UserID.String(),
		Status:          operation.status,
		Version:         mutation.Version,
		SecurityVersion: mutation.SecurityVersion,
		Provider:        mutation.Provider,
		OccurredAt:      mutation.OccurredAt.UTC(),
	}
	if mutation.SessionID != nil {
		document.SessionID = mutation.SessionID.String()
	}
	if mutation.TargetSessionID != nil {
		document.TargetSessionID = mutation.TargetSessionID.String()
	}
	if mutation.ContactID != nil {
		document.ContactID = mutation.ContactID.String()
	}
	if mutation.IntentID != nil {
		document.IntentID = mutation.IntentID.String()
	}
	if mutation.FederatedIdentityID != nil {
		document.FederatedIdentityID = mutation.FederatedIdentityID.String()
	}
	payloadJSON, err := json.Marshal(document)
	if err != nil {
		return audit.Entry{}, eventing.Event{}, fmt.Errorf("marshal identity security event: %w", err)
	}
	after := map[string]any{
		"mutation_id": document.MutationID,
		"user_id":     document.UserID,
		"status":      document.Status,
		"occurred_at": document.OccurredAt,
	}
	if document.SessionID != "" {
		after["session_id"] = document.SessionID
	}
	if document.TargetSessionID != "" {
		after["target_session_id"] = document.TargetSessionID
	}
	if document.ContactID != "" {
		after["contact_id"] = document.ContactID
	}
	if document.IntentID != "" {
		after["intent_id"] = document.IntentID
	}
	if document.FederatedIdentityID != "" {
		after["federated_identity_id"] = document.FederatedIdentityID
	}
	if document.Provider != "" {
		after["provider"] = document.Provider
	}
	if document.Version != 0 {
		after["version"] = document.Version
	}
	if document.SecurityVersion != 0 {
		after["security_version"] = document.SecurityVersion
	}

	resourceID := mutation.UserID.String()
	if mutation.ContactID != nil {
		resourceID = mutation.ContactID.String()
	}
	if mutation.TargetSessionID != nil {
		resourceID = mutation.TargetSessionID.String()
	}
	if mutation.FederatedIdentityID != nil {
		resourceID = mutation.FederatedIdentityID.String()
	}
	var actorUserID, actorSessionID *uuid.UUID
	if mutation.SessionID != nil {
		actor := mutation.UserID
		session := *mutation.SessionID
		actorUserID = &actor
		actorSessionID = &session
	}
	return audit.Entry{
			ID: mutation.ID, ActorUserID: actorUserID, ActorSessionID: actorSessionID,
			Action: operation.eventType, Resource: operation.resource,
			ResourceID: resourceID, Reason: securityMutationReason,
			Before: map[string]any{}, After: after, OccurredAt: mutation.OccurredAt.UTC(),
		}, eventing.Event{
			ID: mutation.ID, Type: operation.eventType,
			AggregateType: securityMutationAggregate, AggregateID: mutation.ID,
			AggregateVersion: 1, Payload: payloadJSON, OccurredAt: mutation.OccurredAt.UTC(),
		}, nil
}

func validMutation(mutation entity.SecurityMutation) bool {
	if mutation.ID == uuid.Nil || mutation.UserID == uuid.Nil || mutation.OccurredAt.IsZero() ||
		invalidUUIDPointer(mutation.SessionID) || invalidUUIDPointer(mutation.TargetSessionID) ||
		invalidUUIDPointer(mutation.ContactID) ||
		invalidUUIDPointer(mutation.IntentID) ||
		invalidUUIDPointer(mutation.FederatedIdentityID) {
		return false
	}
	if mutation.Kind != entity.SecurityMutationOAuthIdentityLinked &&
		mutation.Kind != entity.SecurityMutationOAuthIdentityUnlinked &&
		(mutation.FederatedIdentityID != nil || mutation.Provider != "") {
		return false
	}
	switch mutation.Kind {
	case entity.SecurityMutationUserRegistered:
		return mutation.SessionID == nil && mutation.TargetSessionID == nil && mutation.ContactID != nil && mutation.IntentID == nil &&
			mutation.Version == 1 && mutation.SecurityVersion > 0
	case entity.SecurityMutationOAuthUserRegistered:
		return mutation.SessionID == nil && mutation.TargetSessionID == nil && mutation.ContactID == nil && mutation.IntentID == nil &&
			mutation.FederatedIdentityID == nil && mutation.Provider == "" && mutation.Version == 0 && mutation.SecurityVersion > 0
	case entity.SecurityMutationOAuthIdentityLinked,
		entity.SecurityMutationOAuthIdentityUnlinked:
		return mutation.SessionID != nil && mutation.TargetSessionID == nil && mutation.ContactID == nil && mutation.IntentID == nil &&
			mutation.FederatedIdentityID != nil && validOAuthProvider(mutation.Provider) && mutation.Version == 0 && mutation.SecurityVersion == 0
	case entity.SecurityMutationRecoveryContactVerified:
		return mutation.SessionID == nil && mutation.TargetSessionID == nil && mutation.ContactID != nil && mutation.IntentID == nil &&
			mutation.Version > 0 && mutation.SecurityVersion == 0
	case entity.SecurityMutationRecoveryContactReplacementStarted,
		entity.SecurityMutationRecoveryContactVerificationResent:
		return mutation.SessionID != nil && mutation.TargetSessionID == nil && mutation.ContactID != nil && mutation.IntentID == nil &&
			mutation.Version > 0 && mutation.SecurityVersion == 0
	case entity.SecurityMutationRecoveryContactPendingCancelled,
		entity.SecurityMutationRecoveryContactRemoved:
		return mutation.SessionID != nil && mutation.TargetSessionID == nil && mutation.ContactID == nil && mutation.IntentID == nil &&
			mutation.Version > 0 && mutation.SecurityVersion == 0
	case entity.SecurityMutationPasswordResetConsumed:
		return mutation.SessionID == nil && mutation.ContactID != nil && mutation.IntentID != nil &&
			mutation.TargetSessionID == nil && mutation.Version > 0 && mutation.SecurityVersion > 0
	case entity.SecurityMutationPasswordChanged:
		return mutation.SessionID != nil && mutation.TargetSessionID == nil && mutation.ContactID == nil &&
			mutation.IntentID == nil && mutation.Version == 0 && mutation.SecurityVersion > 0
	case entity.SecurityMutationSessionRevoked:
		return mutation.SessionID != nil && mutation.TargetSessionID != nil && mutation.ContactID == nil &&
			mutation.IntentID == nil && mutation.Version == 0 && mutation.SecurityVersion == 0
	case entity.SecurityMutationSessionsRevokedAll:
		return mutation.SessionID != nil && mutation.TargetSessionID == nil && mutation.ContactID == nil &&
			mutation.IntentID == nil && mutation.Version == 0 && mutation.SecurityVersion > 0
	default:
		return false
	}
}

func validOAuthProvider(value string) bool {
	return value == "google" || value == "github"
}

func invalidUUIDPointer(value *uuid.UUID) bool {
	return value != nil && *value == uuid.Nil
}

func nilDependency(dependency any) bool {
	if dependency == nil {
		return true
	}
	value := reflect.ValueOf(dependency)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}
