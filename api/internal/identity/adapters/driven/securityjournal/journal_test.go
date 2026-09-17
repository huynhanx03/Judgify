package securityjournal

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"

	"github.com/huynhanx03/go-common/pkg/correlation"

	"github.com/huynhanx03/judgify/internal/ent/generate"
	entity "github.com/huynhanx03/judgify/internal/identity/domain"
	"github.com/huynhanx03/judgify/internal/platform/audit"
	"github.com/huynhanx03/judgify/internal/platform/eventing"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
)

type auditCapture struct {
	entries []audit.Entry
	err     error
}

func (capture *auditCapture) Append(_ context.Context, entry audit.Entry) (uuid.UUID, error) {
	if capture.err != nil {
		return uuid.Nil, capture.err
	}
	capture.entries = append(capture.entries, entry)
	return entry.ID, nil
}

type outboxCapture struct {
	events []eventing.Event
	err    error
}

func (capture *outboxCapture) Append(_ context.Context, events ...eventing.Event) error {
	if capture.err != nil {
		return capture.err
	}
	capture.events = append(capture.events, events...)
	return nil
}

func TestJournalMapsClosedSecurityMutationsToRedactedAuditAndOutbox(t *testing.T) {
	tests := []struct {
		kind      entity.SecurityMutationKind
		eventType string
		status    string
		resource  string
		withActor bool
	}{
		{entity.SecurityMutationUserRegistered, "identity.user.registered.v1", "active", "user", false},
		{entity.SecurityMutationOAuthUserRegistered, "identity.user.registered.v1", "active", "user", false},
		{entity.SecurityMutationRecoveryContactVerified, "identity.recovery_contact.verified.v1", "verified", "recovery_contact", false},
		{entity.SecurityMutationRecoveryContactReplacementStarted, "identity.recovery_contact.replacement_started.v1", "pending", "recovery_contact", true},
		{entity.SecurityMutationRecoveryContactVerificationResent, "identity.recovery_contact.verification_resent.v1", "pending", "recovery_contact", true},
		{entity.SecurityMutationRecoveryContactPendingCancelled, "identity.recovery_contact.pending_cancelled.v1", "revoked", "recovery_contact", true},
		{entity.SecurityMutationRecoveryContactRemoved, "identity.recovery_contact.removed.v1", "revoked", "recovery_contact", true},
		{entity.SecurityMutationPasswordResetConsumed, "identity.password_reset.consumed.v1", "completed", "user_security", false},
		{entity.SecurityMutationPasswordChanged, "identity.password.changed.v1", "completed", "user_security", true},
		{entity.SecurityMutationSessionRevoked, "identity.session.revoked.v1", "revoked", "authentication_session", true},
		{entity.SecurityMutationSessionsRevokedAll, "identity.sessions.revoked_all.v1", "revoked", "user_security", true},
	}

	for _, test := range tests {
		t.Run(string(test.kind), func(t *testing.T) {
			audits := &auditCapture{}
			outbox := &outboxCapture{}
			journal, err := New(audits, outbox)
			require.NoError(t, err)
			mutation := validSecurityMutation(test.kind)

			require.NoError(t, journal.Append(context.Background(), mutation))
			require.Len(t, audits.entries, 1)
			require.Len(t, outbox.events, 1)

			entry := audits.entries[0]
			event := outbox.events[0]
			require.Equal(t, mutation.ID, entry.ID)
			require.Equal(t, test.eventType, event.Type)
			require.Equal(t, mutation.ID, event.ID)
			require.Equal(t, mutation.ID, event.AggregateID)
			require.Equal(t, int64(1), event.AggregateVersion)
			require.Equal(t, "security_mutation", event.AggregateType)
			require.Equal(t, mutation.OccurredAt, entry.OccurredAt)
			require.Equal(t, mutation.OccurredAt, event.OccurredAt)
			require.Equal(t, test.resource, entry.Resource)
			require.Equal(t, "security_mutation_committed", entry.Reason)
			require.Empty(t, entry.Before)
			if test.withActor {
				require.NotNil(t, entry.ActorUserID)
				require.Equal(t, mutation.UserID, *entry.ActorUserID)
				require.NotNil(t, entry.ActorSessionID)
				require.Equal(t, *mutation.SessionID, *entry.ActorSessionID)
			} else {
				require.Nil(t, entry.ActorUserID)
				require.Nil(t, entry.ActorSessionID)
			}

			var payload map[string]any
			require.NoError(t, json.Unmarshal(event.Payload, &payload))
			require.Equal(t, mutation.ID.String(), payload["mutation_id"])
			require.Equal(t, mutation.UserID.String(), payload["user_id"])
			require.Equal(t, test.status, payload["status"])
			for key := range payload {
				require.Contains(t, []string{
					"mutation_id", "user_id", "session_id", "target_session_id", "contact_id", "intent_id",
					"status", "version", "security_version", "occurred_at",
				}, key)
			}
			encoded, err := json.Marshal(entry.After)
			require.NoError(t, err)
			require.JSONEq(t, string(event.Payload), string(encoded))
		})
	}
}

func TestJournalRejectsInvalidMutationBeforeCallingDependencies(t *testing.T) {
	audits := &auditCapture{}
	outbox := &outboxCapture{}
	journal, err := New(audits, outbox)
	require.NoError(t, err)
	mutation := validSecurityMutation(entity.SecurityMutationRecoveryContactReplacementStarted)
	mutation.SessionID = nil

	err = journal.Append(context.Background(), mutation)
	require.ErrorIs(t, err, ErrInvalidSecurityMutation)
	require.Empty(t, audits.entries)
	require.Empty(t, outbox.events)
}

func TestJournalStopsBeforeOutboxWhenAuditFails(t *testing.T) {
	auditErr := errors.New("audit unavailable")
	audits := &auditCapture{err: auditErr}
	outbox := &outboxCapture{}
	journal, err := New(audits, outbox)
	require.NoError(t, err)

	err = journal.Append(
		context.Background(),
		validSecurityMutation(entity.SecurityMutationRecoveryContactVerified),
	)
	require.ErrorIs(t, err, auditErr)
	require.Empty(t, outbox.events)
}

func TestJournalPropagatesOutboxFailureToEnclosingTransaction(t *testing.T) {
	database, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = database.Close() })
	client := generate.NewClient(generate.Driver(entsql.OpenDB(dialect.Postgres, database)))
	journal, err := New(audit.NewRepository(), eventing.NewOutbox())
	require.NoError(t, err)
	mutation := validSecurityMutation(entity.SecurityMutationRecoveryContactVerified)
	ctx := correlation.WithContext(
		context.Background(),
		"018f5fbe-3d77-7e10-8fd1-8b6f1ca32e17",
	)
	outboxErr := errors.New("outbox unavailable")

	mock.ExpectBegin()
	mock.ExpectExec(`INSERT INTO "audit_logs"`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`INSERT INTO "outbox_events"`).WillReturnError(outboxErr)
	mock.ExpectRollback()

	err = platformtx.NewEntUnitOfWork(client).Within(ctx, func(txCtx context.Context) error {
		return journal.Append(txCtx, mutation)
	})
	require.ErrorIs(t, err, outboxErr)
	require.NoError(t, mock.ExpectationsWereMet())
}

func validSecurityMutation(kind entity.SecurityMutationKind) entity.SecurityMutation {
	sessionID := uuid.MustParse("00000000-0000-7000-8000-000000000002")
	contactID := uuid.MustParse("00000000-0000-7000-8000-000000000003")
	intentID := uuid.MustParse("00000000-0000-7000-8000-000000000004")
	targetSessionID := uuid.MustParse("00000000-0000-7000-8000-000000000006")
	mutation := entity.SecurityMutation{
		ID:        uuid.MustParse("00000000-0000-7000-8000-000000000001"),
		Kind:      kind,
		UserID:    uuid.MustParse("00000000-0000-7000-8000-000000000005"),
		ContactID: &contactID,
		Version:   2,
		OccurredAt: time.Date(
			2026, time.July, 22, 12, 0, 0, 0, time.UTC,
		),
	}
	switch kind {
	case entity.SecurityMutationUserRegistered:
		mutation.Version = 1
		mutation.SecurityVersion = 1
	case entity.SecurityMutationOAuthUserRegistered:
		mutation.ContactID = nil
		mutation.Version = 0
		mutation.SecurityVersion = 1
	case entity.SecurityMutationRecoveryContactReplacementStarted,
		entity.SecurityMutationRecoveryContactVerificationResent:
		mutation.SessionID = &sessionID
	case entity.SecurityMutationRecoveryContactPendingCancelled,
		entity.SecurityMutationRecoveryContactRemoved:
		mutation.SessionID = &sessionID
		mutation.ContactID = nil
	case entity.SecurityMutationPasswordResetConsumed:
		mutation.IntentID = &intentID
		mutation.SecurityVersion = 5
	case entity.SecurityMutationPasswordChanged:
		mutation.SessionID = &sessionID
		mutation.ContactID = nil
		mutation.Version = 0
		mutation.SecurityVersion = 5
	case entity.SecurityMutationSessionRevoked:
		mutation.SessionID = &sessionID
		mutation.TargetSessionID = &targetSessionID
		mutation.ContactID = nil
		mutation.Version = 0
	case entity.SecurityMutationSessionsRevokedAll:
		mutation.SessionID = &sessionID
		mutation.ContactID = nil
		mutation.Version = 0
		mutation.SecurityVersion = 5
	}
	return mutation
}
