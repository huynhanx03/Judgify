package db

import (
	"context"
	"database/sql/driver"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"

	dbent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	"github.com/huynhanx03/judgify/internal/ports"
)

func TestSecurityEmailEnqueueRequiresCallerTransaction(t *testing.T) {
	client, mock := newSecurityEmailMockClient(t)
	repository := NewSecurityEmailRepository(dbent.WrapClient(client, zap.NewNop()))

	err := repository.Enqueue(context.Background(), securityEmailIntentForTest())
	require.ErrorIs(t, err, platformtx.ErrTransactionRequired)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSecurityEmailEnqueuePersistsEncryptedIntentInCallerTransaction(t *testing.T) {
	client, mock := newSecurityEmailMockClient(t)
	repository := NewSecurityEmailRepository(dbent.WrapClient(client, zap.NewNop()))
	unitOfWork := platformtx.NewEntUnitOfWork(client)
	intent := securityEmailIntentForTest()

	mock.ExpectBegin()
	mock.ExpectExec(`INSERT INTO "security_email_deliveries" .*"provider_idempotency_key".*"payload_ciphertext".*"password_reset_token_id".*`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	require.NoError(t, unitOfWork.Within(context.Background(), func(txCtx context.Context) error {
		return repository.Enqueue(txCtx, intent)
	}))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSecurityEmailEnqueuePersistsVerificationIntentInCallerTransaction(t *testing.T) {
	client, mock := newSecurityEmailMockClient(t)
	repository := NewSecurityEmailRepository(dbent.WrapClient(client, zap.NewNop()))
	unitOfWork := platformtx.NewEntUnitOfWork(client)
	now := time.Date(2026, 7, 22, 9, 0, 0, 0, time.UTC)
	id := uuid.New()
	intent := ports.SecurityEmailDeliveryIntent{
		ID: id, Kind: ports.SecurityEmailKindRecoveryContactVerification,
		VerificationTokenID:    uuid.New(),
		ProviderIdempotencyKey: "judgify-security-email/" + id.String(),
		PayloadCiphertext:      []byte{0x01, 0x02}, KeyID: "security-email-v1",
		PayloadExpiresAt: now.Add(time.Hour), AvailableAt: now,
	}

	mock.ExpectBegin()
	mock.ExpectExec(`INSERT INTO "security_email_deliveries" .*"provider_idempotency_key".*"payload_ciphertext".*"verification_token_id".*`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	require.NoError(t, unitOfWork.Within(context.Background(), func(txCtx context.Context) error {
		return repository.Enqueue(txCtx, intent)
	}))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSecurityEmailIntentRejectsMismatchedCredentialLinkage(t *testing.T) {
	reset := securityEmailIntentForTest()
	reset.VerificationTokenID = uuid.New()
	require.False(t, validSecurityEmailIntent(reset))

	verification := reset
	verification.Kind = ports.SecurityEmailKindRecoveryContactVerification
	verification.PasswordResetTokenID = uuid.Nil
	require.True(t, validSecurityEmailIntent(verification))

	verification.VerificationTokenID = uuid.Nil
	require.False(t, validSecurityEmailIntent(verification))
}

func TestSecurityEmailClaimUsesSkipLockedAndReclaimsExpiredLease(t *testing.T) {
	client, mock := newSecurityEmailMockClient(t)
	repository := NewSecurityEmailRepository(dbent.WrapClient(client, zap.NewNop()))
	unitOfWork := platformtx.NewEntUnitOfWork(client)
	now := time.Date(2026, 7, 22, 9, 0, 0, 0, time.UTC)
	deliveryID := uuid.New()

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT .* FROM "security_email_deliveries" WHERE .*"status".*"next_attempt_at".*"leased_until".* ORDER BY .* FOR UPDATE SKIP LOCKED`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "kind", "provider_idempotency_key", "payload_ciphertext", "key_id",
			"payload_expires_at", "attempt_count", "verification_token_id", "password_reset_token_id",
		}).AddRow(
			deliveryID, "password_reset", "judgify-security-email/"+deliveryID.String(),
			[]byte{0x01, 0x02}, "security-email-v1", now.Add(time.Hour), 2, nil, uuid.New(),
		))
	mock.ExpectExec(`UPDATE "security_email_deliveries" SET .*"status".*"attempt_count".*"lease_owner".*"lease_token".*"leased_until".* WHERE .*"id".*"status".*`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	var leases []ports.SecurityEmailLease
	err := unitOfWork.Within(context.Background(), func(txCtx context.Context) error {
		var claimErr error
		leases, claimErr = repository.Claim(txCtx, ports.SecurityEmailClaim{
			Owner: "api-email-1", Now: now, LeaseDuration: 30 * time.Second, Limit: 10,
		})
		return claimErr
	})
	require.NoError(t, err)
	require.Len(t, leases, 1)
	require.Equal(t, deliveryID, leases[0].ID)
	require.Equal(t, uint32(3), leases[0].Attempt)
	require.NotEqual(t, uuid.Nil, leases[0].LeaseToken)
	require.Equal(t, now.Add(30*time.Second), leases[0].LeasedUntil)
	require.Equal(t, []byte{0x01, 0x02}, leases[0].PayloadCiphertext)
	require.Equal(t, uuid.Nil, leases[0].VerificationTokenID)
	require.NotEqual(t, uuid.Nil, leases[0].PasswordResetTokenID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSecurityEmailPrepareRequiresCallerTransaction(t *testing.T) {
	client, mock := newSecurityEmailMockClient(t)
	repository := NewSecurityEmailRepository(dbent.WrapClient(client, zap.NewNop()))

	_, err := repository.PrepareForSend(context.Background(), securityEmailPrepareForTest())
	require.ErrorIs(t, err, platformtx.ErrTransactionRequired)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSecurityEmailPrepareCancelsExpiredLeaseAndErasesCiphertext(t *testing.T) {
	client, mock := newSecurityEmailMockClient(t)
	repository := NewSecurityEmailRepository(dbent.WrapClient(client, zap.NewNop()))
	unitOfWork := platformtx.NewEntUnitOfWork(client)
	command := securityEmailPrepareForTest()

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT .* FROM "security_email_deliveries" WHERE .*"id".*"status".*"lease_owner".*"lease_token".*"attempt_count".* LIMIT 2`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "kind", "verification_token_id", "password_reset_token_id",
			"payload_expires_at", "leased_until",
		}).AddRow(
			command.ID, "password_reset", nil, command.CredentialID,
			command.Now.Add(-time.Second), command.Now.Add(time.Minute),
		))
	mock.ExpectExec(`UPDATE "security_email_deliveries" SET .*"payload_ciphertext".*"status".*"cancelled_at".*"ciphertext_erased_at".* WHERE .*"id".*"lease_owner".*"lease_token".*"attempt_count".*`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	var ready bool
	err := unitOfWork.Within(context.Background(), func(txCtx context.Context) error {
		var prepareErr error
		ready, prepareErr = repository.PrepareForSend(txCtx, command)
		return prepareErr
	})
	require.NoError(t, err)
	require.False(t, ready)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSecurityEmailPrepareCancelsExpiredOwnershipLeaseBeforeCredentialLookup(t *testing.T) {
	client, mock := newSecurityEmailMockClient(t)
	repository := NewSecurityEmailRepository(dbent.WrapClient(client, zap.NewNop()))
	unitOfWork := platformtx.NewEntUnitOfWork(client)
	command := securityEmailPrepareForTest()

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT .*"leased_until".* FROM "security_email_deliveries" WHERE .*"id".*"status".*"lease_owner".*"lease_token".*"attempt_count".* LIMIT 2`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "kind", "verification_token_id", "password_reset_token_id",
			"payload_expires_at", "leased_until",
		}).AddRow(
			command.ID, "password_reset", nil, command.CredentialID,
			command.Now.Add(time.Hour), command.Now.Add(-time.Nanosecond),
		))
	mock.ExpectExec(`UPDATE "security_email_deliveries" SET .*"payload_ciphertext".*"status".*"cancelled_at".*"ciphertext_erased_at".* WHERE .*"id".*"lease_owner".*"lease_token".*"attempt_count".*`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	var ready bool
	err := unitOfWork.Within(context.Background(), func(txCtx context.Context) error {
		var prepareErr error
		ready, prepareErr = repository.PrepareForSend(txCtx, command)
		return prepareErr
	})
	require.NoError(t, err)
	require.False(t, ready)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSecurityEmailPrepareCancelsVerificationWhenExactContactIsNotPending(t *testing.T) {
	client, mock := newSecurityEmailMockClient(t)
	repository := NewSecurityEmailRepository(dbent.WrapClient(client, zap.NewNop()))
	unitOfWork := platformtx.NewEntUnitOfWork(client)
	command := securityEmailPrepareForTest()
	command.Kind = ports.SecurityEmailKindRecoveryContactVerification
	userID := uuid.New()
	contactID := uuid.New()
	command.CredentialID = uuid.New()

	mock.ExpectBegin()
	expectSecurityEmailDeliverySnapshot(mock, command, command.CredentialID, uuid.Nil)
	mock.ExpectQuery(`SELECT .*"id".*"contact_id".* FROM "recovery_contact_verification_tokens" WHERE .*"id".* LIMIT 2`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "contact_id"}).
			AddRow(command.CredentialID, contactID))
	mock.ExpectQuery(`SELECT .*"id".*"user_id".* FROM "user_recovery_contacts" WHERE .*"id".* LIMIT 2`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id"}).AddRow(contactID, userID))
	mock.ExpectQuery(`SELECT .*"id".*"status".*"deleted_at".* FROM "users" WHERE .*"id".* LIMIT 2 FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "status", "deleted_at"}).
			AddRow(userID, "active", nil))
	mock.ExpectQuery(`SELECT .* FROM "user_recovery_contacts" WHERE .*"id".*"user_id".*"kind".* LIMIT 2 FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "status", "purpose", "version"}).
			AddRow(contactID, userID, "revoked", "registration", int64(7)))
	mock.ExpectQuery(`SELECT .* FROM "recovery_contact_verification_tokens" WHERE .*"id".* LIMIT 2 FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "contact_id", "contact_version", "purpose", "status", "expires_at",
		}).AddRow(
			command.CredentialID, contactID, int64(7), "registration", "issued", command.Now.Add(time.Hour),
		))
	expectSecurityEmailDeliveryLock(mock, command)
	expectSecurityEmailCancellation(mock)
	mock.ExpectCommit()

	var ready bool
	err := unitOfWork.Within(context.Background(), func(txCtx context.Context) error {
		var prepareErr error
		ready, prepareErr = repository.PrepareForSend(txCtx, command)
		return prepareErr
	})
	require.NoError(t, err)
	require.False(t, ready)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSecurityEmailPrepareCancelsVerificationWhenContactPurposeIsInvitation(t *testing.T) {
	client, mock := newSecurityEmailMockClient(t)
	repository := NewSecurityEmailRepository(dbent.WrapClient(client, zap.NewNop()))
	unitOfWork := platformtx.NewEntUnitOfWork(client)
	command := securityEmailPrepareForTest()
	command.Kind = ports.SecurityEmailKindRecoveryContactVerification
	userID := uuid.New()
	contactID := uuid.New()
	command.CredentialID = uuid.New()

	mock.ExpectBegin()
	expectSecurityEmailDeliverySnapshot(mock, command, command.CredentialID, uuid.Nil)
	mock.ExpectQuery(`SELECT .*"id".*"contact_id".* FROM "recovery_contact_verification_tokens" WHERE .*"id".* LIMIT 2`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "contact_id"}).
			AddRow(command.CredentialID, contactID))
	mock.ExpectQuery(`SELECT .*"id".*"user_id".* FROM "user_recovery_contacts" WHERE .*"id".* LIMIT 2`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id"}).AddRow(contactID, userID))
	mock.ExpectQuery(`SELECT .*"id".*"status".*"deleted_at".* FROM "users" WHERE .*"id".* LIMIT 2 FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "status", "deleted_at"}).
			AddRow(userID, "active", nil))
	mock.ExpectQuery(`SELECT .* FROM "user_recovery_contacts" WHERE .*"id".*"user_id".*"kind".* LIMIT 2 FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "status", "purpose", "version"}).
			AddRow(contactID, userID, "pending", "invitation", int64(3)))
	mock.ExpectQuery(`SELECT .* FROM "recovery_contact_verification_tokens" WHERE .*"id".* LIMIT 2 FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "contact_id", "contact_version", "purpose", "status", "expires_at",
		}).AddRow(
			command.CredentialID, contactID, int64(3), "invitation", "issued", command.Now.Add(time.Hour),
		))
	expectSecurityEmailDeliveryLock(mock, command)
	expectSecurityEmailCancellation(mock)
	mock.ExpectCommit()

	var ready bool
	err := unitOfWork.Within(context.Background(), func(txCtx context.Context) error {
		var prepareErr error
		ready, prepareErr = repository.PrepareForSend(txCtx, command)
		return prepareErr
	})
	require.NoError(t, err)
	require.False(t, ready)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSecurityEmailPrepareCancelsVerificationUnlessLinkedTokenIsLatestOverall(t *testing.T) {
	client, mock := newSecurityEmailMockClient(t)
	repository := NewSecurityEmailRepository(dbent.WrapClient(client, zap.NewNop()))
	unitOfWork := platformtx.NewEntUnitOfWork(client)
	command := securityEmailPrepareForTest()
	command.Kind = ports.SecurityEmailKindRecoveryContactVerification
	command.CredentialID = uuid.New()
	userID := uuid.New()
	contactID := uuid.New()
	version := int64(7)

	mock.ExpectBegin()
	expectSecurityEmailDeliverySnapshot(mock, command, command.CredentialID, uuid.Nil)
	mock.ExpectQuery(`SELECT .* FROM "recovery_contact_verification_tokens" WHERE .*"id".* LIMIT 2`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "contact_id"}).
			AddRow(command.CredentialID, contactID))
	mock.ExpectQuery(`SELECT .* FROM "user_recovery_contacts" WHERE .*"id".* LIMIT 2`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id"}).AddRow(contactID, userID))
	mock.ExpectQuery(`SELECT .* FROM "users" WHERE .*"id".* LIMIT 2 FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "status", "deleted_at"}).
			AddRow(userID, "active", nil))
	mock.ExpectQuery(`SELECT .* FROM "user_recovery_contacts" WHERE .*"id".*"user_id".*"kind".* LIMIT 2 FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "status", "purpose", "version"}).
			AddRow(contactID, userID, "pending", "registration", version))
	mock.ExpectQuery(`SELECT .* FROM "recovery_contact_verification_tokens" WHERE .*"id".* LIMIT 2 FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "contact_id", "contact_version", "purpose", "status", "expires_at",
		}).AddRow(
			command.CredentialID, contactID, version, "registration", "issued", command.Now.Add(time.Hour),
		))
	mock.ExpectQuery(`SELECT .* FROM "recovery_contact_verification_tokens" WHERE .*"contact_id".*"contact_version".* ORDER BY .* LIMIT 1`).
		WithArgs(contactID, version).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))
	expectSecurityEmailDeliveryLock(mock, command)
	expectSecurityEmailCancellation(mock)
	mock.ExpectCommit()

	var ready bool
	err := unitOfWork.Within(context.Background(), func(txCtx context.Context) error {
		var prepareErr error
		ready, prepareErr = repository.PrepareForSend(txCtx, command)
		return prepareErr
	})
	require.NoError(t, err)
	require.False(t, ready)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSecurityEmailPrepareCancelsPasswordResetUnlessAuthorityAndLatestTokenAreCurrent(t *testing.T) {
	client, mock := newSecurityEmailMockClient(t)
	repository := NewSecurityEmailRepository(dbent.WrapClient(client, zap.NewNop()))
	unitOfWork := platformtx.NewEntUnitOfWork(client)
	command := securityEmailPrepareForTest()
	userID := uuid.New()
	contactID := uuid.New()
	version := int64(11)
	securityVersion := int64(5)

	mock.ExpectBegin()
	expectSecurityEmailDeliverySnapshot(mock, command, uuid.Nil, command.CredentialID)
	mock.ExpectQuery(`SELECT .* FROM "password_reset_tokens" WHERE .*"id".* LIMIT 2`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "contact_id"}).
			AddRow(command.CredentialID, userID, contactID))
	mock.ExpectQuery(`SELECT .* FROM "users" WHERE .*"id".* LIMIT 2 FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "status", "deleted_at", "security_version",
		}).AddRow(userID, "active", nil, securityVersion))
	mock.ExpectQuery(`SELECT .* FROM "user_recovery_contacts" WHERE .*"id".*"user_id".*"kind".* LIMIT 2 FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "status", "version"}).
			AddRow(contactID, userID, "verified", version))
	mock.ExpectQuery(`SELECT .* FROM "password_reset_tokens" WHERE .*"id".* LIMIT 2 FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "contact_id", "contact_version", "user_security_version",
			"status", "expires_at",
		}).AddRow(
			command.CredentialID, userID, contactID, version, securityVersion,
			"issued", command.Now.Add(time.Hour),
		))
	mock.ExpectQuery(`SELECT .* FROM "credentials" WHERE .*"user_id".*"type".*"active".*"deleted_at".* LIMIT 1`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))
	mock.ExpectQuery(`SELECT .* FROM "password_reset_tokens" WHERE .*"user_id".* ORDER BY .* LIMIT 1`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))
	expectSecurityEmailDeliveryLock(mock, command)
	expectSecurityEmailCancellation(mock)
	mock.ExpectCommit()

	var ready bool
	err := unitOfWork.Within(context.Background(), func(txCtx context.Context) error {
		var prepareErr error
		ready, prepareErr = repository.PrepareForSend(txCtx, command)
		return prepareErr
	})
	require.NoError(t, err)
	require.False(t, ready)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSecurityEmailPrepareCancelsEveryStaleVerificationAuthorityState(t *testing.T) {
	now := time.Date(2026, 7, 22, 9, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name           string
		userStatus     string
		contactStatus  string
		contactVersion int64
		tokenVersion   int64
		tokenStatus    string
		tokenExpiresAt time.Time
	}{
		{
			name: "inactive user", userStatus: "suspended", contactStatus: "pending",
			contactVersion: 7, tokenVersion: 7, tokenStatus: "issued", tokenExpiresAt: now.Add(time.Hour),
		},
		{
			name: "contact no longer pending", userStatus: "active", contactStatus: "verified",
			contactVersion: 7, tokenVersion: 7, tokenStatus: "issued", tokenExpiresAt: now.Add(time.Hour),
		},
		{
			name: "contact version changed", userStatus: "active", contactStatus: "pending",
			contactVersion: 8, tokenVersion: 7, tokenStatus: "issued", tokenExpiresAt: now.Add(time.Hour),
		},
		{
			name: "token superseded", userStatus: "active", contactStatus: "pending",
			contactVersion: 7, tokenVersion: 7, tokenStatus: "superseded", tokenExpiresAt: now.Add(time.Hour),
		},
		{
			name: "token expired", userStatus: "active", contactStatus: "pending",
			contactVersion: 7, tokenVersion: 7, tokenStatus: "issued", tokenExpiresAt: now,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			client, mock := newSecurityEmailMockClient(t)
			repository := NewSecurityEmailRepository(dbent.WrapClient(client, zap.NewNop()))
			unitOfWork := platformtx.NewEntUnitOfWork(client)
			command := securityEmailPrepareForTest()
			command.Kind = ports.SecurityEmailKindRecoveryContactVerification
			command.CredentialID = uuid.New()
			command.Now = now
			userID, contactID := uuid.New(), uuid.New()

			mock.ExpectBegin()
			expectSecurityEmailDeliverySnapshot(mock, command, command.CredentialID, uuid.Nil)
			expectVerificationTokenSnapshot(mock, command.CredentialID, contactID)
			expectRecoveryContactSnapshot(mock, contactID, userID)
			expectLockedSecurityEmailUser(mock, userID, test.userStatus, 0)
			expectLockedRecoveryContact(
				mock, contactID, userID, test.contactStatus, test.contactVersion,
			)
			expectLockedVerificationToken(
				mock, command.CredentialID, contactID, test.tokenVersion,
				test.tokenStatus, test.tokenExpiresAt,
			)
			expectSecurityEmailDeliveryLock(mock, command)
			expectSecurityEmailCancellation(mock)
			mock.ExpectCommit()

			var ready bool
			err := unitOfWork.Within(context.Background(), func(txCtx context.Context) error {
				var prepareErr error
				ready, prepareErr = repository.PrepareForSend(txCtx, command)
				return prepareErr
			})
			require.NoError(t, err)
			require.False(t, ready)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestSecurityEmailPrepareCancelsEveryStalePasswordResetAuthorityState(t *testing.T) {
	now := time.Date(2026, 7, 22, 9, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name                 string
		userStatus           string
		userSecurityVersion  int64
		contactStatus        string
		contactVersion       int64
		tokenSecurityVersion int64
		tokenContactVersion  int64
		tokenStatus          string
		tokenExpiresAt       time.Time
		hasPassword          bool
	}{
		{
			name: "inactive user", userStatus: "suspended", userSecurityVersion: 5,
			contactStatus: "verified", contactVersion: 11,
			tokenSecurityVersion: 5, tokenContactVersion: 11,
			tokenStatus: "issued", tokenExpiresAt: now.Add(time.Hour), hasPassword: true,
		},
		{
			name: "security version changed", userStatus: "active", userSecurityVersion: 6,
			contactStatus: "verified", contactVersion: 11,
			tokenSecurityVersion: 5, tokenContactVersion: 11,
			tokenStatus: "issued", tokenExpiresAt: now.Add(time.Hour), hasPassword: true,
		},
		{
			name: "password credential disabled", userStatus: "active", userSecurityVersion: 5,
			contactStatus: "verified", contactVersion: 11,
			tokenSecurityVersion: 5, tokenContactVersion: 11,
			tokenStatus: "issued", tokenExpiresAt: now.Add(time.Hour), hasPassword: false,
		},
		{
			name: "recovery contact replaced", userStatus: "active", userSecurityVersion: 5,
			contactStatus: "revoked", contactVersion: 11,
			tokenSecurityVersion: 5, tokenContactVersion: 11,
			tokenStatus: "issued", tokenExpiresAt: now.Add(time.Hour), hasPassword: true,
		},
		{
			name: "recovery contact version changed", userStatus: "active", userSecurityVersion: 5,
			contactStatus: "verified", contactVersion: 12,
			tokenSecurityVersion: 5, tokenContactVersion: 11,
			tokenStatus: "issued", tokenExpiresAt: now.Add(time.Hour), hasPassword: true,
		},
		{
			name: "token superseded", userStatus: "active", userSecurityVersion: 5,
			contactStatus: "verified", contactVersion: 11,
			tokenSecurityVersion: 5, tokenContactVersion: 11,
			tokenStatus: "superseded", tokenExpiresAt: now.Add(time.Hour), hasPassword: true,
		},
		{
			name: "token expired", userStatus: "active", userSecurityVersion: 5,
			contactStatus: "verified", contactVersion: 11,
			tokenSecurityVersion: 5, tokenContactVersion: 11,
			tokenStatus: "issued", tokenExpiresAt: now, hasPassword: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			client, mock := newSecurityEmailMockClient(t)
			repository := NewSecurityEmailRepository(dbent.WrapClient(client, zap.NewNop()))
			unitOfWork := platformtx.NewEntUnitOfWork(client)
			command := securityEmailPrepareForTest()
			command.Now = now
			userID, contactID := uuid.New(), uuid.New()

			mock.ExpectBegin()
			expectSecurityEmailDeliverySnapshot(mock, command, uuid.Nil, command.CredentialID)
			expectPasswordResetTokenSnapshot(mock, command.CredentialID, userID, contactID)
			expectLockedSecurityEmailUser(
				mock, userID, test.userStatus, test.userSecurityVersion,
			)
			expectLockedRecoveryContact(
				mock, contactID, userID, test.contactStatus, test.contactVersion,
			)
			expectLockedPasswordResetToken(
				mock, command.CredentialID, userID, contactID,
				test.tokenContactVersion, test.tokenSecurityVersion,
				test.tokenStatus, test.tokenExpiresAt,
			)
			expectActivePasswordCredential(mock, test.hasPassword)
			expectSecurityEmailDeliveryLock(mock, command)
			expectSecurityEmailCancellation(mock)
			mock.ExpectCommit()

			var ready bool
			err := unitOfWork.Within(context.Background(), func(txCtx context.Context) error {
				var prepareErr error
				ready, prepareErr = repository.PrepareForSend(txCtx, command)
				return prepareErr
			})
			require.NoError(t, err)
			require.False(t, ready)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestSecurityEmailPrepareAcceptsOnlyCurrentVerificationAuthority(t *testing.T) {
	client, mock := newSecurityEmailMockClient(t)
	repository := NewSecurityEmailRepository(dbent.WrapClient(client, zap.NewNop()))
	unitOfWork := platformtx.NewEntUnitOfWork(client)
	command := securityEmailPrepareForTest()
	command.Kind = ports.SecurityEmailKindRecoveryContactVerification
	command.CredentialID = uuid.New()
	userID, contactID := uuid.New(), uuid.New()
	version := int64(7)

	mock.ExpectBegin()
	expectSecurityEmailDeliverySnapshot(mock, command, command.CredentialID, uuid.Nil)
	expectVerificationTokenSnapshot(mock, command.CredentialID, contactID)
	expectRecoveryContactSnapshot(mock, contactID, userID)
	expectLockedSecurityEmailUser(mock, userID, "active", 0)
	expectLockedRecoveryContact(mock, contactID, userID, "pending", version)
	expectLockedVerificationToken(
		mock, command.CredentialID, contactID, version, "issued", command.Now.Add(time.Hour),
	)
	mock.ExpectQuery(`SELECT .* FROM "recovery_contact_verification_tokens" WHERE .*"contact_id".*"contact_version".* ORDER BY .* LIMIT 1`).
		WithArgs(contactID, version).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(command.CredentialID))
	expectSecurityEmailDeliveryLock(mock, command)
	mock.ExpectCommit()

	var ready bool
	err := unitOfWork.Within(context.Background(), func(txCtx context.Context) error {
		var prepareErr error
		ready, prepareErr = repository.PrepareForSend(txCtx, command)
		return prepareErr
	})
	require.NoError(t, err)
	require.True(t, ready)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSecurityEmailPrepareAcceptsOnlyCurrentPasswordResetAuthority(t *testing.T) {
	client, mock := newSecurityEmailMockClient(t)
	repository := NewSecurityEmailRepository(dbent.WrapClient(client, zap.NewNop()))
	unitOfWork := platformtx.NewEntUnitOfWork(client)
	command := securityEmailPrepareForTest()
	userID, contactID := uuid.New(), uuid.New()
	contactVersion, securityVersion := int64(11), int64(5)

	mock.ExpectBegin()
	expectSecurityEmailDeliverySnapshot(mock, command, uuid.Nil, command.CredentialID)
	expectPasswordResetTokenSnapshot(mock, command.CredentialID, userID, contactID)
	expectLockedSecurityEmailUser(mock, userID, "active", securityVersion)
	expectLockedRecoveryContact(mock, contactID, userID, "verified", contactVersion)
	expectLockedPasswordResetToken(
		mock, command.CredentialID, userID, contactID, contactVersion, securityVersion,
		"issued", command.Now.Add(time.Hour),
	)
	expectActivePasswordCredential(mock, true)
	mock.ExpectQuery(`SELECT .* FROM "password_reset_tokens" WHERE .*"user_id".* ORDER BY .* LIMIT 1`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(command.CredentialID))
	expectSecurityEmailDeliveryLock(mock, command)
	mock.ExpectCommit()

	var ready bool
	err := unitOfWork.Within(context.Background(), func(txCtx context.Context) error {
		var prepareErr error
		ready, prepareErr = repository.PrepareForSend(txCtx, command)
		return prepareErr
	})
	require.NoError(t, err)
	require.True(t, ready)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSecurityEmailTerminalTransitionsAreFencedAndEraseCiphertext(t *testing.T) {
	for _, test := range []struct {
		name   string
		invoke func(*SecurityEmailRepository, context.Context, ports.SecurityEmailLeaseOutcome) error
		match  string
	}{
		{
			name: "sent",
			invoke: func(repository *SecurityEmailRepository, ctx context.Context, outcome ports.SecurityEmailLeaseOutcome) error {
				return repository.MarkSent(ctx, outcome)
			},
			match: `UPDATE "security_email_deliveries" SET .*"payload_ciphertext".*"status".*"sent_at".*"ciphertext_erased_at".* WHERE .*"id".*"lease_owner".*"lease_token".*"attempt_count".*`,
		},
		{
			name: "dead",
			invoke: func(repository *SecurityEmailRepository, ctx context.Context, outcome ports.SecurityEmailLeaseOutcome) error {
				outcome.ErrorClass = "payload_expired"
				return repository.MarkDead(ctx, outcome)
			},
			match: `UPDATE "security_email_deliveries" SET .*"payload_ciphertext".*"status".*"dead_at".*"last_error_class".*"ciphertext_erased_at".* WHERE .*"id".*"lease_owner".*"lease_token".*"attempt_count".*`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			client, mock := newSecurityEmailMockClient(t)
			repository := NewSecurityEmailRepository(dbent.WrapClient(client, zap.NewNop()))
			unitOfWork := platformtx.NewEntUnitOfWork(client)
			outcome := ports.SecurityEmailLeaseOutcome{
				ID: uuid.New(), Owner: "api-email-1", LeaseToken: uuid.New(), Attempt: 3,
				At: time.Date(2026, 7, 22, 9, 1, 0, 0, time.UTC),
			}

			mock.ExpectBegin()
			mock.ExpectExec(test.match).WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectCommit()
			require.NoError(t, unitOfWork.Within(context.Background(), func(txCtx context.Context) error {
				return test.invoke(repository, txCtx, outcome)
			}))
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestSecurityEmailRetryTransitionRetainsCiphertextAndIsFenced(t *testing.T) {
	client, mock := newSecurityEmailMockClient(t)
	repository := NewSecurityEmailRepository(dbent.WrapClient(client, zap.NewNop()))
	unitOfWork := platformtx.NewEntUnitOfWork(client)
	now := time.Date(2026, 7, 22, 9, 1, 0, 0, time.UTC)
	outcome := ports.SecurityEmailLeaseOutcome{
		ID: uuid.New(), Owner: "api-email-1", LeaseToken: uuid.New(), Attempt: 3,
		At: now, NextAttemptAt: now.Add(time.Minute), ErrorClass: "provider_unavailable",
	}

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "security_email_deliveries" SET .*"status".*"next_attempt_at".*"last_error_class".* WHERE .*"id".*"lease_owner".*"lease_token".*"attempt_count".*"payload_expires_at".*`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	require.NoError(t, unitOfWork.Within(context.Background(), func(txCtx context.Context) error {
		return repository.MarkRetry(txCtx, outcome)
	}))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSecurityEmailFinalizeRejectsStaleLeaseFence(t *testing.T) {
	client, mock := newSecurityEmailMockClient(t)
	repository := NewSecurityEmailRepository(dbent.WrapClient(client, zap.NewNop()))
	unitOfWork := platformtx.NewEntUnitOfWork(client)
	outcome := ports.SecurityEmailLeaseOutcome{
		ID: uuid.New(), Owner: "api-email-1", LeaseToken: uuid.New(), Attempt: 3,
		At: time.Now().UTC(),
	}

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "security_email_deliveries" SET .*`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()
	err := unitOfWork.Within(context.Background(), func(txCtx context.Context) error {
		return repository.MarkSent(txCtx, outcome)
	})
	require.ErrorIs(t, err, ports.ErrSecurityEmailLeaseLost)
	require.NoError(t, mock.ExpectationsWereMet())
}

func securityEmailIntentForTest() ports.SecurityEmailDeliveryIntent {
	now := time.Date(2026, 7, 22, 9, 0, 0, 0, time.UTC)
	id := uuid.New()
	return ports.SecurityEmailDeliveryIntent{
		ID: id, Kind: ports.SecurityEmailKindPasswordReset, PasswordResetTokenID: uuid.New(),
		ProviderIdempotencyKey: "judgify-security-email/" + id.String(),
		PayloadCiphertext:      []byte{0x01, 0x02}, KeyID: "security-email-v1",
		PayloadExpiresAt: now.Add(time.Hour), AvailableAt: now,
	}
}

func securityEmailPrepareForTest() ports.SecurityEmailPrepareCommand {
	return ports.SecurityEmailPrepareCommand{
		ID: uuid.New(), Kind: ports.SecurityEmailKindPasswordReset,
		CredentialID: uuid.New(), Owner: "api-email-1", LeaseToken: uuid.New(),
		Attempt: 2, Now: time.Date(2026, 7, 22, 9, 0, 0, 0, time.UTC),
	}
}

func expectSecurityEmailDeliverySnapshot(
	mock sqlmock.Sqlmock,
	command ports.SecurityEmailPrepareCommand,
	verificationTokenID uuid.UUID,
	passwordResetTokenID uuid.UUID,
) {
	var verification any
	if verificationTokenID != uuid.Nil {
		verification = verificationTokenID
	}
	var reset any
	if passwordResetTokenID != uuid.Nil {
		reset = passwordResetTokenID
	}
	mock.ExpectQuery(`SELECT .* FROM "security_email_deliveries" WHERE .*"id".*"status".*"lease_owner".*"lease_token".*"attempt_count".* LIMIT 2`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "kind", "verification_token_id", "password_reset_token_id",
			"payload_expires_at", "leased_until",
		}).AddRow(
			command.ID, string(command.Kind), verification, reset,
			command.Now.Add(time.Hour), command.Now.Add(time.Minute),
		))
}

func expectSecurityEmailDeliveryLock(mock sqlmock.Sqlmock, command ports.SecurityEmailPrepareCommand) {
	mock.ExpectQuery(`SELECT .* FROM "security_email_deliveries" WHERE .*"id".*"status".*"lease_owner".*"lease_token".*"attempt_count".* LIMIT 2 FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "payload_expires_at", "leased_until"}).
			AddRow(command.ID, command.Now.Add(time.Hour), command.Now.Add(time.Minute)))
}

func expectVerificationTokenSnapshot(
	mock sqlmock.Sqlmock,
	tokenID uuid.UUID,
	contactID uuid.UUID,
) {
	mock.ExpectQuery(`SELECT .* FROM "recovery_contact_verification_tokens" WHERE .*"id".* LIMIT 2`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "contact_id"}).AddRow(tokenID, contactID))
}

func expectPasswordResetTokenSnapshot(
	mock sqlmock.Sqlmock,
	tokenID uuid.UUID,
	userID uuid.UUID,
	contactID uuid.UUID,
) {
	mock.ExpectQuery(`SELECT .* FROM "password_reset_tokens" WHERE .*"id".* LIMIT 2`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "contact_id"}).
			AddRow(tokenID, userID, contactID))
}

func expectRecoveryContactSnapshot(mock sqlmock.Sqlmock, contactID uuid.UUID, userID uuid.UUID) {
	mock.ExpectQuery(`SELECT .* FROM "user_recovery_contacts" WHERE .*"id".* LIMIT 2`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id"}).AddRow(contactID, userID))
}

func expectLockedSecurityEmailUser(
	mock sqlmock.Sqlmock,
	userID uuid.UUID,
	status string,
	securityVersion int64,
) {
	columns := []string{"id", "status", "deleted_at"}
	values := []driver.Value{userID, status, nil}
	if securityVersion > 0 {
		columns = append(columns, "security_version")
		values = append(values, securityVersion)
	}
	mock.ExpectQuery(`SELECT .* FROM "users" WHERE .*"id".* LIMIT 2 FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows(columns).AddRow(values...))
}

func expectLockedRecoveryContact(
	mock sqlmock.Sqlmock,
	contactID uuid.UUID,
	userID uuid.UUID,
	status string,
	version int64,
) {
	mock.ExpectQuery(`SELECT .* FROM "user_recovery_contacts" WHERE .*"id".*"user_id".*"kind".* LIMIT 2 FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "status", "purpose", "version"}).
			AddRow(contactID, userID, status, "registration", version))
}

func expectLockedVerificationToken(
	mock sqlmock.Sqlmock,
	tokenID uuid.UUID,
	contactID uuid.UUID,
	contactVersion int64,
	status string,
	expiresAt time.Time,
) {
	mock.ExpectQuery(`SELECT .* FROM "recovery_contact_verification_tokens" WHERE .*"id".* LIMIT 2 FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "contact_id", "contact_version", "purpose", "status", "expires_at",
		}).AddRow(tokenID, contactID, contactVersion, "registration", status, expiresAt))
}

func expectLockedPasswordResetToken(
	mock sqlmock.Sqlmock,
	tokenID uuid.UUID,
	userID uuid.UUID,
	contactID uuid.UUID,
	contactVersion int64,
	securityVersion int64,
	status string,
	expiresAt time.Time,
) {
	mock.ExpectQuery(`SELECT .* FROM "password_reset_tokens" WHERE .*"id".* LIMIT 2 FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "contact_id", "contact_version", "user_security_version",
			"status", "expires_at",
		}).AddRow(
			tokenID, userID, contactID, contactVersion, securityVersion, status, expiresAt,
		))
}

func expectActivePasswordCredential(mock sqlmock.Sqlmock, exists bool) {
	rows := sqlmock.NewRows([]string{"id"})
	if exists {
		rows.AddRow(uuid.New())
	}
	mock.ExpectQuery(`SELECT .* FROM "credentials" WHERE .*"user_id".*"type".*"active".*"deleted_at".* LIMIT 1`).
		WillReturnRows(rows)
}

func expectSecurityEmailCancellation(mock sqlmock.Sqlmock) {
	mock.ExpectExec(`UPDATE "security_email_deliveries" SET .*"payload_ciphertext".*"status".*"cancelled_at".*"ciphertext_erased_at".* WHERE .*"id".*"lease_owner".*"lease_token".*"attempt_count".*`).
		WillReturnResult(sqlmock.NewResult(0, 1))
}

func newSecurityEmailMockClient(t *testing.T) (*generate.Client, sqlmock.Sqlmock) {
	t.Helper()
	database, mock, err := sqlmock.New()
	require.NoError(t, err)
	driver := entsql.OpenDB(dialect.Postgres, database)
	client := generate.NewClient(generate.Driver(driver))
	t.Cleanup(func() { _ = client.Close() })
	return client, mock
}
