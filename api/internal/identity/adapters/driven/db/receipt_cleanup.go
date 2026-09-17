package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"entgo.io/ent/dialect/sql"

	commonent "github.com/huynhanx03/go-common/pkg/database/ent"

	dbent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/ent/generate/authrefreshreceipt"
	"github.com/huynhanx03/judgify/internal/ent/generate/authsessioncommandreceipt"
	"github.com/huynhanx03/judgify/internal/ent/generate/authsessionissuancereceipt"
	"github.com/huynhanx03/judgify/internal/ent/generate/passwordresettoken"
	"github.com/huynhanx03/judgify/internal/ent/generate/predicate"
	"github.com/huynhanx03/judgify/internal/ent/generate/recoverycontactcommandreceipt"
	"github.com/huynhanx03/judgify/internal/ent/generate/recoverycontactverificationtoken"
	"github.com/huynhanx03/judgify/internal/ent/generate/securityemaildelivery"
	"github.com/huynhanx03/judgify/internal/ent/generate/userrecoverycontact"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	"github.com/huynhanx03/judgify/internal/ports"
)

const (
	receiptCleanupRepoName = "receipt cleanup"
	cleanupWorkKinds       = ports.MinimumReceiptCleanupBatchSize
	erasedRecoveryKeyID    = "erased"
	expiredDeliveryClass   = "expired"
)

type cleanupLimits struct {
	pendingContacts        int
	revokedContactSecrets  int
	terminalContacts       int
	verificationExpiry     int
	resetExpiry            int
	terminalVerification   int
	terminalReset          int
	expiredDeliveries      int
	terminalDeliveries     int
	sessionIssuance        int
	refreshRotation        int
	commandReceipts        int
	sessionCommandReceipts int
}

// ReceiptCleanupRepository performs one short, transaction-scoped cleanup
// batch. FOR UPDATE SKIP LOCKED is the lease: locks disappear on commit or
// rollback, so there is no durable lease state to strand after a crash.
type ReceiptCleanupRepository struct {
	client *dbent.EntClient
}

func NewReceiptCleanupRepository(client *dbent.EntClient) ports.ReceiptCleanupRepository {
	return &ReceiptCleanupRepository{client: client}
}

func (repository *ReceiptCleanupRepository) EraseExpired(
	ctx context.Context,
	request ports.ReceiptCleanupRequest,
) (ports.ReceiptCleanupResult, error) {
	if repository == nil || repository.client == nil || ctx == nil || request.Now.IsZero() ||
		request.TerminalBefore.IsZero() || !request.TerminalBefore.Before(request.Now) ||
		request.BatchSize < ports.MinimumReceiptCleanupBatchSize ||
		request.BatchSize > ports.MaximumReceiptCleanupBatchSize {
		return ports.ReceiptCleanupResult{}, fmt.Errorf("%s: invalid cleanup request", receiptCleanupRepoName)
	}
	if err := ctx.Err(); err != nil {
		return ports.ReceiptCleanupResult{}, err
	}
	// Row locks only protect the subsequent erasure while the query and update
	// share one physical transaction. Refuse an ordinary context rather than
	// silently degrading to a racy two-statement operation.
	if _, err := platformtx.Client(ctx, nil); err != nil {
		return ports.ReceiptCleanupResult{}, errors.Join(
			platformtx.ErrTransactionRequired,
			fmt.Errorf("%s: transaction required", receiptCleanupRepoName),
		)
	}

	client := repository.client.DB(ctx)
	now := request.Now.UTC()
	terminalBefore := request.TerminalBefore.UTC()
	limits := cleanupBatchShares(request.BatchSize)

	// Acquire recovery locks in aggregate order (contacts -> tokens ->
	// deliveries). Domain commands use the same direction, so a janitor can
	// never hold a child while waiting on a parent locked by a request.
	pendingContactIDs, err := client.UserRecoveryContact.Query().
		Where(
			userrecoverycontact.StatusEQ(userrecoverycontact.StatusPending),
			// An invitation contact is account lifecycle state, not an
			// abandoned self-service contact. Keep it pending after the link
			// expires so an administrator can issue a replacement invitation.
			userrecoverycontact.PurposeNEQ(userrecoverycontact.PurposeInvitation),
			userrecoverycontact.CreatedAtLTE(now),
			userrecoverycontact.Not(userrecoverycontact.HasVerificationTokensWith(
				recoverycontactverificationtoken.StatusEQ(recoverycontactverificationtoken.StatusIssued),
				recoverycontactverificationtoken.ExpiresAtGT(now),
			)),
		).
		Order(userrecoverycontact.ByUpdatedAt(), userrecoverycontact.ByID()).
		Limit(limits.pendingContacts).
		Select(userrecoverycontact.FieldID).
		Modify(forUpdateSkipLocked).
		IDs(ctx)
	if err != nil {
		return ports.ReceiptCleanupResult{}, commonent.MapEntError(err, receiptCleanupRepoName)
	}
	revokedSecretContactIDs, err := client.UserRecoveryContact.Query().
		Where(
			userrecoverycontact.StatusEQ(userrecoverycontact.StatusRevoked),
			userrecoverycontact.Or(
				userrecoverycontact.AddressCiphertextNEQ(recoveryErasureSentinel()),
				userrecoverycontact.LookupHashNEQ(recoveryErasureSentinel()),
				userrecoverycontact.FingerprintHashNEQ(recoveryErasureSentinel()),
			),
		).
		Order(userrecoverycontact.ByUpdatedAt(), userrecoverycontact.ByID()).
		Limit(limits.revokedContactSecrets).
		Select(userrecoverycontact.FieldID).
		Modify(forUpdateSkipLocked).
		IDs(ctx)
	if err != nil {
		return ports.ReceiptCleanupResult{}, commonent.MapEntError(err, receiptCleanupRepoName)
	}
	terminalContactIDs, err := client.UserRecoveryContact.Query().
		Where(
			userrecoverycontact.StatusEQ(userrecoverycontact.StatusRevoked),
			userrecoverycontact.UpdatedAtLTE(terminalBefore),
			userrecoverycontact.AddressCiphertextEQ(recoveryErasureSentinel()),
			userrecoverycontact.Not(userrecoverycontact.HasVerificationTokens()),
			userrecoverycontact.Not(userrecoverycontact.HasPasswordResetTokens()),
		).
		Order(userrecoverycontact.ByUpdatedAt(), userrecoverycontact.ByID()).
		Limit(limits.terminalContacts).
		Select(userrecoverycontact.FieldID).
		Modify(forUpdateSkipLocked).
		IDs(ctx)
	if err != nil {
		return ports.ReceiptCleanupResult{}, commonent.MapEntError(err, receiptCleanupRepoName)
	}

	verificationExpiryIDs, err := client.RecoveryContactVerificationToken.Query().
		Where(
			recoverycontactverificationtoken.StatusEQ(recoverycontactverificationtoken.StatusIssued),
			recoverycontactverificationtoken.ExpiresAtLTE(now),
		).
		Order(recoverycontactverificationtoken.ByExpiresAt(), recoverycontactverificationtoken.ByID()).
		Limit(limits.verificationExpiry).
		Select(recoverycontactverificationtoken.FieldID).
		Modify(forUpdateSkipLocked).
		IDs(ctx)
	if err != nil {
		return ports.ReceiptCleanupResult{}, commonent.MapEntError(err, receiptCleanupRepoName)
	}
	resetExpiryIDs, err := client.PasswordResetToken.Query().
		Where(
			passwordresettoken.StatusEQ(passwordresettoken.StatusIssued),
			passwordresettoken.ExpiresAtLTE(now),
		).
		Order(passwordresettoken.ByExpiresAt(), passwordresettoken.ByID()).
		Limit(limits.resetExpiry).
		Select(passwordresettoken.FieldID).
		Modify(forUpdateSkipLocked).
		IDs(ctx)
	if err != nil {
		return ports.ReceiptCleanupResult{}, commonent.MapEntError(err, receiptCleanupRepoName)
	}
	terminalVerificationIDs, err := client.RecoveryContactVerificationToken.Query().
		Where(
			recoverycontactverificationtoken.StatusIn(
				recoverycontactverificationtoken.StatusConsumed,
				recoverycontactverificationtoken.StatusSuperseded,
				recoverycontactverificationtoken.StatusExpired,
			),
			recoverycontactverificationtoken.UpdatedAtLTE(terminalBefore),
			recoverycontactverificationtoken.Not(recoverycontactverificationtoken.HasEmailDeliveries()),
			notPendingInvitationVerificationToken(),
		).
		Order(recoverycontactverificationtoken.ByUpdatedAt(), recoverycontactverificationtoken.ByID()).
		Limit(limits.terminalVerification).
		Select(recoverycontactverificationtoken.FieldID).
		Modify(forUpdateSkipLocked).
		IDs(ctx)
	if err != nil {
		return ports.ReceiptCleanupResult{}, commonent.MapEntError(err, receiptCleanupRepoName)
	}
	terminalResetIDs, err := client.PasswordResetToken.Query().
		Where(
			passwordresettoken.StatusIn(
				passwordresettoken.StatusConsumed,
				passwordresettoken.StatusSuperseded,
				passwordresettoken.StatusExpired,
			),
			passwordresettoken.UpdatedAtLTE(terminalBefore),
			passwordresettoken.Not(passwordresettoken.HasEmailDeliveries()),
		).
		Order(passwordresettoken.ByUpdatedAt(), passwordresettoken.ByID()).
		Limit(limits.terminalReset).
		Select(passwordresettoken.FieldID).
		Modify(forUpdateSkipLocked).
		IDs(ctx)
	if err != nil {
		return ports.ReceiptCleanupResult{}, commonent.MapEntError(err, receiptCleanupRepoName)
	}

	expiredDeliveryIDs, err := client.SecurityEmailDelivery.Query().
		Where(
			securityemaildelivery.StatusIn(
				securityemaildelivery.StatusReady,
				securityemaildelivery.StatusLeased,
			),
			securityemaildelivery.PayloadExpiresAtLTE(now),
		).
		Order(securityemaildelivery.ByPayloadExpiresAt(), securityemaildelivery.ByID()).
		Limit(limits.expiredDeliveries).
		Select(securityemaildelivery.FieldID).
		Modify(forUpdateSkipLocked).
		IDs(ctx)
	if err != nil {
		return ports.ReceiptCleanupResult{}, commonent.MapEntError(err, receiptCleanupRepoName)
	}
	terminalDeliveryIDs, err := client.SecurityEmailDelivery.Query().
		Where(
			securityemaildelivery.StatusIn(
				securityemaildelivery.StatusSent,
				securityemaildelivery.StatusDead,
				securityemaildelivery.StatusCancelled,
			),
			securityemaildelivery.UpdatedAtLTE(terminalBefore),
			securityemaildelivery.CiphertextErasedAtNotNil(),
		).
		Order(securityemaildelivery.ByUpdatedAt(), securityemaildelivery.ByID()).
		Limit(limits.terminalDeliveries).
		Select(securityemaildelivery.FieldID).
		Modify(forUpdateSkipLocked).
		IDs(ctx)
	if err != nil {
		return ports.ReceiptCleanupResult{}, commonent.MapEntError(err, receiptCleanupRepoName)
	}

	issuanceIDs, err := client.AuthSessionIssuanceReceipt.Query().
		Where(
			authsessionissuancereceipt.ExpiresAtLTE(now),
			authsessionissuancereceipt.ErasedAtIsNil(),
		).
		Order(
			authsessionissuancereceipt.ByExpiresAt(),
			authsessionissuancereceipt.ByID(),
		).
		Limit(limits.sessionIssuance).
		Select(authsessionissuancereceipt.FieldID).
		Modify(forUpdateSkipLocked).
		IDs(ctx)
	if err != nil {
		return ports.ReceiptCleanupResult{}, commonent.MapEntError(err, receiptCleanupRepoName)
	}

	refreshIDs, err := client.AuthRefreshReceipt.Query().
		Where(
			authrefreshreceipt.ExpiresAtLTE(now),
			authrefreshreceipt.ErasedAtIsNil(),
		).
		Order(
			authrefreshreceipt.ByExpiresAt(),
			authrefreshreceipt.ByID(),
		).
		Limit(limits.refreshRotation).
		Select(authrefreshreceipt.FieldID).
		Modify(forUpdateSkipLocked).
		IDs(ctx)
	if err != nil {
		return ports.ReceiptCleanupResult{}, commonent.MapEntError(err, receiptCleanupRepoName)
	}

	commandReceiptIDs, err := client.RecoveryContactCommandReceipt.Query().
		Where(recoverycontactcommandreceipt.ExpiresAtLTE(now)).
		Order(recoverycontactcommandreceipt.ByExpiresAt(), recoverycontactcommandreceipt.ByID()).
		Limit(limits.commandReceipts).
		Select(recoverycontactcommandreceipt.FieldID).
		Modify(forUpdateSkipLocked).
		IDs(ctx)
	if err != nil {
		return ports.ReceiptCleanupResult{}, commonent.MapEntError(err, receiptCleanupRepoName)
	}
	sessionCommandReceiptIDs, err := client.AuthSessionCommandReceipt.Query().
		Where(authsessioncommandreceipt.ExpiresAtLTE(now)).
		Order(authsessioncommandreceipt.ByExpiresAt(), authsessioncommandreceipt.ByID()).
		Limit(limits.sessionCommandReceipts).
		Select(authsessioncommandreceipt.FieldID).
		Modify(forUpdateSkipLocked).
		IDs(ctx)
	if err != nil {
		return ports.ReceiptCleanupResult{}, commonent.MapEntError(err, receiptCleanupRepoName)
	}

	pendingRevoked, err := revokeStaleRecoveryContacts(ctx, client, pendingContactIDs, now)
	if err != nil {
		return ports.ReceiptCleanupResult{}, err
	}
	revokedErased, err := eraseRevokedRecoveryContacts(ctx, client, revokedSecretContactIDs)
	if err != nil {
		return ports.ReceiptCleanupResult{}, err
	}
	verificationExpired, err := expireVerificationTokens(ctx, client, verificationExpiryIDs, now)
	if err != nil {
		return ports.ReceiptCleanupResult{}, err
	}
	resetExpired, err := expirePasswordResetTokens(ctx, client, resetExpiryIDs, now)
	if err != nil {
		return ports.ReceiptCleanupResult{}, err
	}
	deliveriesCancelled, err := cancelExpiredSecurityEmailDeliveries(ctx, client, expiredDeliveryIDs, now)
	if err != nil {
		return ports.ReceiptCleanupResult{}, err
	}
	terminalDeliveriesDeleted, err := deleteTerminalSecurityEmailDeliveries(
		ctx, client, terminalDeliveryIDs, terminalBefore,
	)
	if err != nil {
		return ports.ReceiptCleanupResult{}, err
	}
	terminalVerificationDeleted, err := deleteTerminalVerificationTokens(
		ctx, client, terminalVerificationIDs, terminalBefore,
	)
	if err != nil {
		return ports.ReceiptCleanupResult{}, err
	}
	terminalResetDeleted, err := deleteTerminalPasswordResetTokens(
		ctx, client, terminalResetIDs, terminalBefore,
	)
	if err != nil {
		return ports.ReceiptCleanupResult{}, err
	}
	terminalContactsDeleted, err := deleteTerminalRecoveryContacts(
		ctx, client, terminalContactIDs, terminalBefore,
	)
	if err != nil {
		return ports.ReceiptCleanupResult{}, err
	}
	commandReceiptsDeleted, err := deleteExpiredRecoveryCommandReceipts(
		ctx, client, commandReceiptIDs, now,
	)
	if err != nil {
		return ports.ReceiptCleanupResult{}, err
	}
	sessionCommandReceiptsDeleted, err := deleteExpiredSessionCommandReceipts(
		ctx, client, sessionCommandReceiptIDs, now,
	)
	if err != nil {
		return ports.ReceiptCleanupResult{}, err
	}

	erasedAt := now
	issuanceErased, err := eraseSessionIssuanceReceipts(ctx, client, issuanceIDs, erasedAt)
	if err != nil {
		return ports.ReceiptCleanupResult{}, err
	}
	refreshErased, err := eraseRefreshRotationReceipts(ctx, client, refreshIDs, erasedAt)
	if err != nil {
		return ports.ReceiptCleanupResult{}, err
	}
	return ports.ReceiptCleanupResult{
		SessionIssuanceErased:                  issuanceErased,
		RefreshRotationErased:                  refreshErased,
		RecoveryCommandReceiptsDeleted:         commandReceiptsDeleted,
		SessionCommandReceiptsDeleted:          sessionCommandReceiptsDeleted,
		VerificationTokensExpired:              verificationExpired,
		PasswordResetTokensExpired:             resetExpired,
		SecurityEmailDeliveriesCancelled:       deliveriesCancelled,
		PendingRecoveryContactsRevoked:         pendingRevoked,
		RevokedRecoveryContactsErased:          revokedErased,
		TerminalSecurityEmailDeliveriesDeleted: terminalDeliveriesDeleted,
		TerminalVerificationTokensDeleted:      terminalVerificationDeleted,
		TerminalPasswordResetTokensDeleted:     terminalResetDeleted,
		TerminalRecoveryContactsDeleted:        terminalContactsDeleted,
		MayHaveMore: len(pendingContactIDs) == limits.pendingContacts ||
			len(revokedSecretContactIDs) == limits.revokedContactSecrets ||
			len(terminalContactIDs) == limits.terminalContacts ||
			len(verificationExpiryIDs) == limits.verificationExpiry ||
			len(resetExpiryIDs) == limits.resetExpiry ||
			len(terminalVerificationIDs) == limits.terminalVerification ||
			len(terminalResetIDs) == limits.terminalReset ||
			len(expiredDeliveryIDs) == limits.expiredDeliveries ||
			len(terminalDeliveryIDs) == limits.terminalDeliveries ||
			len(issuanceIDs) == limits.sessionIssuance ||
			len(refreshIDs) == limits.refreshRotation ||
			len(commandReceiptIDs) == limits.commandReceipts ||
			len(sessionCommandReceiptIDs) == limits.sessionCommandReceipts,
	}, nil
}

func forUpdateSkipLocked(selector *sql.Selector) {
	selector.ForUpdate(sql.WithLockAction(sql.SkipLocked))
}

func cleanupBatchShares(batchSize int) cleanupLimits {
	base := batchSize / cleanupWorkKinds
	limits := cleanupLimits{
		pendingContacts: base, revokedContactSecrets: base, terminalContacts: base,
		verificationExpiry: base, resetExpiry: base, terminalVerification: base,
		terminalReset: base, expiredDeliveries: base, terminalDeliveries: base,
		sessionIssuance: base, refreshRotation: base,
		commandReceipts: base, sessionCommandReceipts: base,
	}
	shares := []*int{
		&limits.pendingContacts, &limits.revokedContactSecrets, &limits.terminalContacts,
		&limits.verificationExpiry, &limits.resetExpiry, &limits.terminalVerification,
		&limits.terminalReset, &limits.expiredDeliveries, &limits.terminalDeliveries,
		&limits.sessionIssuance, &limits.refreshRotation,
		&limits.commandReceipts, &limits.sessionCommandReceipts,
	}
	for index := 0; index < batchSize%cleanupWorkKinds; index++ {
		(*shares[index])++
	}
	return limits
}

func recoveryErasureSentinel() []byte { return []byte{0} }

func notPendingInvitationVerificationToken() predicate.RecoveryContactVerificationToken {
	return recoverycontactverificationtoken.Not(
		recoverycontactverificationtoken.And(
			recoverycontactverificationtoken.PurposeEQ(
				recoverycontactverificationtoken.PurposeInvitation,
			),
			recoverycontactverificationtoken.StatusEQ(
				recoverycontactverificationtoken.StatusExpired,
			),
			recoverycontactverificationtoken.HasContactWith(
				userrecoverycontact.PurposeEQ(userrecoverycontact.PurposeInvitation),
				userrecoverycontact.StatusEQ(userrecoverycontact.StatusPending),
			),
		),
	)
}

func revokeStaleRecoveryContacts(
	ctx context.Context,
	client *generate.Client,
	ids []uuid.UUID,
	now time.Time,
) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	updated, err := client.UserRecoveryContact.Update().
		Where(
			userrecoverycontact.IDIn(ids...),
			userrecoverycontact.StatusEQ(userrecoverycontact.StatusPending),
			userrecoverycontact.PurposeNEQ(userrecoverycontact.PurposeInvitation),
			userrecoverycontact.Not(userrecoverycontact.HasVerificationTokensWith(
				recoverycontactverificationtoken.StatusEQ(recoverycontactverificationtoken.StatusIssued),
				recoverycontactverificationtoken.ExpiresAtGT(now),
			)),
		).
		SetUpdatedAt(now).
		SetStatus(userrecoverycontact.StatusRevoked).
		SetRevokedAt(now).
		SetAddressCiphertext(recoveryErasureSentinel()).
		SetKeyID(erasedRecoveryKeyID).
		Modify(eraseRecoveryContactBlindIndexes).
		Save(ctx)
	if err != nil {
		return 0, commonent.MapEntError(err, receiptCleanupRepoName)
	}
	return requireCleanupFence("pending recovery contact revocation", updated, len(ids))
}

func eraseRevokedRecoveryContacts(
	ctx context.Context,
	client *generate.Client,
	ids []uuid.UUID,
) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	updated, err := client.UserRecoveryContact.Update().
		Where(
			userrecoverycontact.IDIn(ids...),
			userrecoverycontact.StatusEQ(userrecoverycontact.StatusRevoked),
			userrecoverycontact.Or(
				userrecoverycontact.AddressCiphertextNEQ(recoveryErasureSentinel()),
				userrecoverycontact.LookupHashNEQ(recoveryErasureSentinel()),
				userrecoverycontact.FingerprintHashNEQ(recoveryErasureSentinel()),
			),
		).
		SetAddressCiphertext(recoveryErasureSentinel()).
		SetKeyID(erasedRecoveryKeyID).
		Modify(eraseRecoveryContactBlindIndexes).
		Save(ctx)
	if err != nil {
		return 0, commonent.MapEntError(err, receiptCleanupRepoName)
	}
	return requireCleanupFence("revoked recovery contact erasure", updated, len(ids))
}

func eraseRecoveryContactBlindIndexes(update *sql.UpdateBuilder) {
	update.Set(userrecoverycontact.FieldLookupHash, recoveryErasureSentinel())
	update.Set(userrecoverycontact.FieldFingerprintHash, recoveryErasureSentinel())
}

func expireVerificationTokens(
	ctx context.Context,
	client *generate.Client,
	ids []uuid.UUID,
	now time.Time,
) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	updated, err := client.RecoveryContactVerificationToken.Update().
		Where(
			recoverycontactverificationtoken.IDIn(ids...),
			recoverycontactverificationtoken.StatusEQ(recoverycontactverificationtoken.StatusIssued),
			recoverycontactverificationtoken.ExpiresAtLTE(now),
		).
		SetUpdatedAt(now).
		SetStatus(recoverycontactverificationtoken.StatusExpired).
		Save(ctx)
	if err != nil {
		return 0, commonent.MapEntError(err, receiptCleanupRepoName)
	}
	return requireCleanupFence("verification token expiry", updated, len(ids))
}

func expirePasswordResetTokens(
	ctx context.Context,
	client *generate.Client,
	ids []uuid.UUID,
	now time.Time,
) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	updated, err := client.PasswordResetToken.Update().
		Where(
			passwordresettoken.IDIn(ids...),
			passwordresettoken.StatusEQ(passwordresettoken.StatusIssued),
			passwordresettoken.ExpiresAtLTE(now),
		).
		SetUpdatedAt(now).
		SetStatus(passwordresettoken.StatusExpired).
		Save(ctx)
	if err != nil {
		return 0, commonent.MapEntError(err, receiptCleanupRepoName)
	}
	return requireCleanupFence("password reset token expiry", updated, len(ids))
}

func cancelExpiredSecurityEmailDeliveries(
	ctx context.Context,
	client *generate.Client,
	ids []uuid.UUID,
	now time.Time,
) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	updated, err := client.SecurityEmailDelivery.Update().
		Where(
			securityemaildelivery.IDIn(ids...),
			securityemaildelivery.StatusIn(
				securityemaildelivery.StatusReady,
				securityemaildelivery.StatusLeased,
			),
			securityemaildelivery.PayloadExpiresAtLTE(now),
		).
		SetUpdatedAt(now).
		SetStatus(securityemaildelivery.StatusCancelled).
		SetPayloadCiphertext(recoveryErasureSentinel()).
		SetCancelledAt(now).
		SetCiphertextErasedAt(now).
		SetLastErrorClass(expiredDeliveryClass).
		ClearLeaseOwner().
		ClearLeaseToken().
		ClearLeasedUntil().
		Save(ctx)
	if err != nil {
		return 0, commonent.MapEntError(err, receiptCleanupRepoName)
	}
	return requireCleanupFence("security email delivery expiry", updated, len(ids))
}

func deleteTerminalSecurityEmailDeliveries(
	ctx context.Context,
	client *generate.Client,
	ids []uuid.UUID,
	terminalBefore time.Time,
) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	deleted, err := client.SecurityEmailDelivery.Delete().
		Where(
			securityemaildelivery.IDIn(ids...),
			securityemaildelivery.StatusIn(
				securityemaildelivery.StatusSent,
				securityemaildelivery.StatusDead,
				securityemaildelivery.StatusCancelled,
			),
			securityemaildelivery.UpdatedAtLTE(terminalBefore),
			securityemaildelivery.CiphertextErasedAtNotNil(),
		).
		Exec(ctx)
	if err != nil {
		return 0, commonent.MapEntError(err, receiptCleanupRepoName)
	}
	return requireCleanupFence("terminal security email deletion", deleted, len(ids))
}

func deleteTerminalVerificationTokens(
	ctx context.Context,
	client *generate.Client,
	ids []uuid.UUID,
	terminalBefore time.Time,
) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	deleted, err := client.RecoveryContactVerificationToken.Delete().
		Where(
			recoverycontactverificationtoken.IDIn(ids...),
			recoverycontactverificationtoken.StatusIn(
				recoverycontactverificationtoken.StatusConsumed,
				recoverycontactverificationtoken.StatusSuperseded,
				recoverycontactverificationtoken.StatusExpired,
			),
			recoverycontactverificationtoken.UpdatedAtLTE(terminalBefore),
			recoverycontactverificationtoken.Not(recoverycontactverificationtoken.HasEmailDeliveries()),
			notPendingInvitationVerificationToken(),
		).
		Exec(ctx)
	if err != nil {
		return 0, commonent.MapEntError(err, receiptCleanupRepoName)
	}
	return requireCleanupFence("terminal verification token deletion", deleted, len(ids))
}

func deleteTerminalPasswordResetTokens(
	ctx context.Context,
	client *generate.Client,
	ids []uuid.UUID,
	terminalBefore time.Time,
) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	deleted, err := client.PasswordResetToken.Delete().
		Where(
			passwordresettoken.IDIn(ids...),
			passwordresettoken.StatusIn(
				passwordresettoken.StatusConsumed,
				passwordresettoken.StatusSuperseded,
				passwordresettoken.StatusExpired,
			),
			passwordresettoken.UpdatedAtLTE(terminalBefore),
			passwordresettoken.Not(passwordresettoken.HasEmailDeliveries()),
		).
		Exec(ctx)
	if err != nil {
		return 0, commonent.MapEntError(err, receiptCleanupRepoName)
	}
	return requireCleanupFence("terminal password reset token deletion", deleted, len(ids))
}

func deleteTerminalRecoveryContacts(
	ctx context.Context,
	client *generate.Client,
	ids []uuid.UUID,
	terminalBefore time.Time,
) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	deleted, err := client.UserRecoveryContact.Delete().
		Where(
			userrecoverycontact.IDIn(ids...),
			userrecoverycontact.StatusEQ(userrecoverycontact.StatusRevoked),
			userrecoverycontact.UpdatedAtLTE(terminalBefore),
			userrecoverycontact.AddressCiphertextEQ(recoveryErasureSentinel()),
			userrecoverycontact.Not(userrecoverycontact.HasVerificationTokens()),
			userrecoverycontact.Not(userrecoverycontact.HasPasswordResetTokens()),
		).
		Exec(ctx)
	if err != nil {
		return 0, commonent.MapEntError(err, receiptCleanupRepoName)
	}
	return requireCleanupFence("terminal recovery contact deletion", deleted, len(ids))
}

func deleteExpiredRecoveryCommandReceipts(
	ctx context.Context,
	client *generate.Client,
	ids []uuid.UUID,
	now time.Time,
) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	deleted, err := client.RecoveryContactCommandReceipt.Delete().
		Where(
			recoverycontactcommandreceipt.IDIn(ids...),
			recoverycontactcommandreceipt.ExpiresAtLTE(now),
		).
		Exec(ctx)
	if err != nil {
		return 0, commonent.MapEntError(err, receiptCleanupRepoName)
	}
	return requireCleanupFence("recovery command receipt deletion", deleted, len(ids))
}

func deleteExpiredSessionCommandReceipts(
	ctx context.Context,
	client *generate.Client,
	ids []uuid.UUID,
	now time.Time,
) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	deleted, err := client.AuthSessionCommandReceipt.Delete().
		Where(
			authsessioncommandreceipt.IDIn(ids...),
			authsessioncommandreceipt.ExpiresAtLTE(now),
		).
		Exec(ctx)
	if err != nil {
		return 0, commonent.MapEntError(err, receiptCleanupRepoName)
	}
	return requireCleanupFence("session command receipt deletion", deleted, len(ids))
}

func requireCleanupFence(operation string, affected, expected int) (int, error) {
	if affected != expected {
		return 0, fmt.Errorf("%s: %s fence changed", receiptCleanupRepoName, operation)
	}
	return affected, nil
}

func eraseSessionIssuanceReceipts(
	ctx context.Context,
	client *generate.Client,
	ids []uuid.UUID,
	erasedAt time.Time,
) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	updated, err := client.AuthSessionIssuanceReceipt.Update().
		Where(
			authsessionissuancereceipt.IDIn(ids...),
			authsessionissuancereceipt.ExpiresAtLTE(erasedAt),
			authsessionissuancereceipt.ErasedAtIsNil(),
		).
		SetErasedAt(erasedAt).
		Modify(func(update *sql.UpdateBuilder) {
			// The schema keeps recovery_ciphertext immutable to ordinary domain
			// mutations. This privacy-specific adapter is the sole deliberate
			// exception and overwrites it with a non-secret NotEmpty sentinel.
			update.Set(authsessionissuancereceipt.FieldRecoveryCiphertext, []byte{0})
		}).
		Save(ctx)
	if err != nil {
		return 0, commonent.MapEntError(err, receiptCleanupRepoName)
	}
	if updated != len(ids) {
		return 0, fmt.Errorf("%s: session issuance cleanup fence changed", receiptCleanupRepoName)
	}
	return updated, nil
}

func eraseRefreshRotationReceipts(
	ctx context.Context,
	client *generate.Client,
	ids []uuid.UUID,
	erasedAt time.Time,
) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	updated, err := client.AuthRefreshReceipt.Update().
		Where(
			authrefreshreceipt.IDIn(ids...),
			authrefreshreceipt.ExpiresAtLTE(erasedAt),
			authrefreshreceipt.ErasedAtIsNil(),
		).
		SetErasedAt(erasedAt).
		Modify(func(update *sql.UpdateBuilder) {
			update.Set(authrefreshreceipt.FieldRecoveryCiphertext, []byte{0})
		}).
		Save(ctx)
	if err != nil {
		return 0, commonent.MapEntError(err, receiptCleanupRepoName)
	}
	if updated != len(ids) {
		return 0, fmt.Errorf("%s: refresh rotation cleanup fence changed", receiptCleanupRepoName)
	}
	return updated, nil
}

var _ ports.ReceiptCleanupRepository = (*ReceiptCleanupRepository)(nil)
