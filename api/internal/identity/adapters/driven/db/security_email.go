package db

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"

	"entgo.io/ent/dialect/sql"

	commonent "github.com/huynhanx03/go-common/pkg/database/ent"

	dbent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/ent/generate/passwordresettoken"
	"github.com/huynhanx03/judgify/internal/ent/generate/predicate"
	"github.com/huynhanx03/judgify/internal/ent/generate/recoverycontactverificationtoken"
	"github.com/huynhanx03/judgify/internal/ent/generate/securityemaildelivery"
	"github.com/huynhanx03/judgify/internal/ent/generate/user"
	"github.com/huynhanx03/judgify/internal/ent/generate/userrecoverycontact"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	"github.com/huynhanx03/judgify/internal/ports"
)

const (
	securityEmailRepositoryName = "security email delivery"
	maximumSecurityEmailBatch   = 100
	maximumSecurityEmailLease   = 10 * time.Minute
	maximumSecurityEmailPayload = 64 << 10
)

type SecurityEmailRepository struct {
	client *dbent.EntClient
}

func NewSecurityEmailRepository(client *dbent.EntClient) *SecurityEmailRepository {
	return &SecurityEmailRepository{client: client}
}

func (repository *SecurityEmailRepository) Enqueue(
	ctx context.Context,
	intent ports.SecurityEmailDeliveryIntent,
) error {
	if repository == nil || repository.client == nil || ctx == nil ||
		!validSecurityEmailIntent(intent) {
		return fmt.Errorf("%s: invalid enqueue request", securityEmailRepositoryName)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := requireSecurityEmailTransaction(ctx); err != nil {
		return err
	}
	create := repository.client.DB(ctx).SecurityEmailDelivery.Create().
		SetID(intent.ID).
		SetKind(securityemaildelivery.Kind(intent.Kind)).
		SetProviderIdempotencyKey(intent.ProviderIdempotencyKey).
		SetPayloadCiphertext(append([]byte(nil), intent.PayloadCiphertext...)).
		SetKeyID(intent.KeyID).
		SetPayloadExpiresAt(intent.PayloadExpiresAt.UTC()).
		SetStatus(securityemaildelivery.StatusReady).
		SetAttemptCount(0).
		SetNextAttemptAt(intent.AvailableAt.UTC())
	if intent.Kind == ports.SecurityEmailKindPasswordReset {
		create.SetPasswordResetTokenID(intent.PasswordResetTokenID)
	} else {
		create.SetVerificationTokenID(intent.VerificationTokenID)
	}
	_, err := create.Save(ctx)
	if err != nil {
		return commonent.MapEntError(err, securityEmailRepositoryName)
	}
	return nil
}

func (repository *SecurityEmailRepository) Claim(
	ctx context.Context,
	claim ports.SecurityEmailClaim,
) ([]ports.SecurityEmailLease, error) {
	if repository == nil || repository.client == nil || ctx == nil ||
		!validSecurityEmailClaim(claim) {
		return nil, fmt.Errorf("%s: invalid claim request", securityEmailRepositoryName)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := requireSecurityEmailTransaction(ctx); err != nil {
		return nil, err
	}
	client := repository.client.DB(ctx)
	now := claim.Now.UTC()
	rows, err := client.SecurityEmailDelivery.Query().
		Where(
			securityemaildelivery.Or(
				securityemaildelivery.And(
					securityemaildelivery.StatusEQ(securityemaildelivery.StatusReady),
					securityemaildelivery.NextAttemptAtLTE(now),
				),
				securityemaildelivery.And(
					securityemaildelivery.StatusEQ(securityemaildelivery.StatusLeased),
					securityemaildelivery.LeasedUntilLTE(now),
				),
			),
		).
		Order(
			securityemaildelivery.ByNextAttemptAt(),
			securityemaildelivery.ByCreatedAt(),
			securityemaildelivery.ByID(),
		).
		Limit(claim.Limit).
		Select(
			securityemaildelivery.FieldID,
			securityemaildelivery.FieldKind,
			securityemaildelivery.FieldProviderIdempotencyKey,
			securityemaildelivery.FieldPayloadCiphertext,
			securityemaildelivery.FieldKeyID,
			securityemaildelivery.FieldPayloadExpiresAt,
			securityemaildelivery.FieldAttemptCount,
			securityemaildelivery.FieldVerificationTokenID,
			securityemaildelivery.FieldPasswordResetTokenID,
		).
		Modify(func(selector *sql.Selector) {
			selector.ForUpdate(sql.WithLockAction(sql.SkipLocked))
		}).
		All(ctx)
	if err != nil {
		return nil, commonent.MapEntError(err, securityEmailRepositoryName)
	}
	leases := make([]ports.SecurityEmailLease, 0, len(rows))
	leasedUntil := now.Add(claim.LeaseDuration)
	for _, row := range rows {
		if row == nil || row.ID == uuid.Nil || row.AttemptCount == math.MaxUint32 {
			return nil, fmt.Errorf("%s: invalid claim state", securityEmailRepositoryName)
		}
		leaseToken, err := uuid.NewV7()
		if err != nil {
			return nil, fmt.Errorf("%s: generate lease token: %w", securityEmailRepositoryName, err)
		}
		attempt := row.AttemptCount + 1
		updated, err := client.SecurityEmailDelivery.Update().
			Where(
				securityemaildelivery.ID(row.ID),
				securityemaildelivery.Or(
					securityemaildelivery.And(
						securityemaildelivery.StatusEQ(securityemaildelivery.StatusReady),
						securityemaildelivery.NextAttemptAtLTE(now),
					),
					securityemaildelivery.And(
						securityemaildelivery.StatusEQ(securityemaildelivery.StatusLeased),
						securityemaildelivery.LeasedUntilLTE(now),
					),
				),
			).
			SetStatus(securityemaildelivery.StatusLeased).
			SetAttemptCount(attempt).
			SetLeaseOwner(claim.Owner).
			SetLeaseToken(leaseToken).
			SetLeasedUntil(leasedUntil).
			Save(ctx)
		if err != nil {
			return nil, commonent.MapEntError(err, securityEmailRepositoryName)
		}
		if updated != 1 {
			return nil, ports.ErrSecurityEmailLeaseLost
		}
		lease := ports.SecurityEmailLease{
			ID: row.ID, Kind: ports.SecurityEmailKind(row.Kind),
			ProviderIdempotencyKey: row.ProviderIdempotencyKey,
			PayloadCiphertext:      append([]byte(nil), row.PayloadCiphertext...),
			KeyID:                  row.KeyID,
			PayloadExpiresAt:       row.PayloadExpiresAt.UTC(),
			Attempt:                attempt,
			Owner:                  claim.Owner,
			LeaseToken:             leaseToken,
			LeasedUntil:            leasedUntil,
		}
		if row.VerificationTokenID != nil {
			lease.VerificationTokenID = *row.VerificationTokenID
		}
		if row.PasswordResetTokenID != nil {
			lease.PasswordResetTokenID = *row.PasswordResetTokenID
		}
		leases = append(leases, lease)
	}
	return leases, nil
}

func (repository *SecurityEmailRepository) PrepareForSend(
	ctx context.Context,
	command ports.SecurityEmailPrepareCommand,
) (bool, error) {
	if repository == nil || repository.client == nil || ctx == nil ||
		!validSecurityEmailPrepareCommand(command) {
		return false, fmt.Errorf("%s: invalid prepare command", securityEmailRepositoryName)
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if err := requireSecurityEmailTransaction(ctx); err != nil {
		return false, err
	}
	client := repository.client.DB(ctx)
	snapshot, err := client.SecurityEmailDelivery.Query().Where(
		securityEmailPrepareFence(command),
	).Select(
		securityemaildelivery.FieldID,
		securityemaildelivery.FieldKind,
		securityemaildelivery.FieldVerificationTokenID,
		securityemaildelivery.FieldPasswordResetTokenID,
		securityemaildelivery.FieldPayloadExpiresAt,
		securityemaildelivery.FieldLeasedUntil,
	).Only(ctx)
	if err != nil {
		if generate.IsNotFound(err) {
			return false, ports.ErrSecurityEmailLeaseLost
		}
		return false, commonent.MapEntError(err, securityEmailRepositoryName)
	}
	if snapshot.Kind != securityemaildelivery.Kind(command.Kind) || snapshot.LeasedUntil == nil ||
		!snapshot.LeasedUntil.After(command.Now.UTC()) ||
		!snapshot.PayloadExpiresAt.After(command.Now.UTC()) {
		return repository.cancelPreparedLease(ctx, client, command)
	}
	switch command.Kind {
	case ports.SecurityEmailKindRecoveryContactVerification,
		ports.SecurityEmailKindAccountInvitation:
		if snapshot.VerificationTokenID == nil || *snapshot.VerificationTokenID != command.CredentialID ||
			snapshot.PasswordResetTokenID != nil {
			return repository.cancelPreparedLease(ctx, client, command)
		}
		return repository.prepareVerificationDelivery(ctx, client, command)
	case ports.SecurityEmailKindPasswordReset:
		if snapshot.PasswordResetTokenID == nil || *snapshot.PasswordResetTokenID != command.CredentialID ||
			snapshot.VerificationTokenID != nil {
			return repository.cancelPreparedLease(ctx, client, command)
		}
		return repository.preparePasswordResetDelivery(ctx, client, command)
	default:
		return repository.cancelPreparedLease(ctx, client, command)
	}
}

func (repository *SecurityEmailRepository) prepareVerificationDelivery(
	ctx context.Context,
	client *generate.Client,
	command ports.SecurityEmailPrepareCommand,
) (bool, error) {
	snapshot, err := client.RecoveryContactVerificationToken.Query().Where(
		recoverycontactverificationtoken.ID(command.CredentialID),
	).Select(
		recoverycontactverificationtoken.FieldID,
		recoverycontactverificationtoken.FieldContactID,
	).Only(ctx)
	if err != nil {
		if generate.IsNotFound(err) {
			return repository.cancelPreparedLease(ctx, client, command)
		}
		return false, commonent.MapEntError(err, securityEmailRepositoryName)
	}
	contactSnapshot, err := client.UserRecoveryContact.Query().Where(
		userrecoverycontact.ID(snapshot.ContactID),
	).Select(
		userrecoverycontact.FieldID,
		userrecoverycontact.FieldUserID,
	).Only(ctx)
	if err != nil {
		if generate.IsNotFound(err) {
			return repository.cancelPreparedLease(ctx, client, command)
		}
		return false, commonent.MapEntError(err, securityEmailRepositoryName)
	}
	owner, err := client.User.Query().Where(user.ID(contactSnapshot.UserID)).Select(
		user.FieldID,
		user.FieldStatus,
		user.FieldDeletedAt,
	).ForUpdate().Only(ctx)
	if err != nil {
		if generate.IsNotFound(err) {
			return repository.cancelPreparedLease(ctx, client, command)
		}
		return false, commonent.MapEntError(err, securityEmailRepositoryName)
	}
	candidate, err := client.UserRecoveryContact.Query().Where(
		userrecoverycontact.ID(snapshot.ContactID),
		userrecoverycontact.UserID(owner.ID),
		userrecoverycontact.KindEQ(userrecoverycontact.KindEmail),
	).Select(
		userrecoverycontact.FieldID,
		userrecoverycontact.FieldUserID,
		userrecoverycontact.FieldStatus,
		userrecoverycontact.FieldPurpose,
		userrecoverycontact.FieldVersion,
	).ForUpdate().Only(ctx)
	if err != nil {
		if generate.IsNotFound(err) {
			return repository.cancelPreparedLease(ctx, client, command)
		}
		return false, commonent.MapEntError(err, securityEmailRepositoryName)
	}
	token, err := client.RecoveryContactVerificationToken.Query().Where(
		recoverycontactverificationtoken.ID(command.CredentialID),
	).Select(
		recoverycontactverificationtoken.FieldID,
		recoverycontactverificationtoken.FieldContactID,
		recoverycontactverificationtoken.FieldContactVersion,
		recoverycontactverificationtoken.FieldPurpose,
		recoverycontactverificationtoken.FieldStatus,
		recoverycontactverificationtoken.FieldExpiresAt,
	).ForUpdate().Only(ctx)
	if err != nil {
		if generate.IsNotFound(err) {
			return repository.cancelPreparedLease(ctx, client, command)
		}
		return false, commonent.MapEntError(err, securityEmailRepositoryName)
	}
	expectedOwnerStatus := user.StatusActive
	validPurpose := string(candidate.Purpose) == string(token.Purpose) &&
		(token.Purpose == recoverycontactverificationtoken.PurposeRegistration ||
			token.Purpose == recoverycontactverificationtoken.PurposeReplacement)
	if command.Kind == ports.SecurityEmailKindAccountInvitation {
		expectedOwnerStatus = user.StatusInvited
		validPurpose = token.Purpose == recoverycontactverificationtoken.PurposeInvitation &&
			string(candidate.Purpose) == string(token.Purpose)
	}
	valid := owner.Status == expectedOwnerStatus && owner.DeletedAt == nil &&
		token.Status == recoverycontactverificationtoken.StatusIssued &&
		token.ExpiresAt.After(command.Now.UTC()) && validPurpose
	valid = valid && token.ContactID == candidate.ID &&
		candidate.Status == userrecoverycontact.StatusPending &&
		candidate.Version == token.ContactVersion
	if valid {
		latest, latestErr := client.RecoveryContactVerificationToken.Query().Where(
			recoverycontactverificationtoken.ContactID(token.ContactID),
			recoverycontactverificationtoken.ContactVersion(token.ContactVersion),
		).Order(
			recoverycontactverificationtoken.ByCreatedAt(sql.OrderDesc()),
			recoverycontactverificationtoken.ByID(sql.OrderDesc()),
		).Select(recoverycontactverificationtoken.FieldID).First(ctx)
		if latestErr != nil && !generate.IsNotFound(latestErr) {
			return false, commonent.MapEntError(latestErr, securityEmailRepositoryName)
		}
		valid = latestErr == nil && latest.ID == token.ID
	}
	delivery, err := repository.lockPreparedDelivery(ctx, client, command)
	if err != nil {
		return false, err
	}
	valid = valid && delivery.LeasedUntil != nil && delivery.LeasedUntil.After(command.Now.UTC()) &&
		delivery.PayloadExpiresAt.After(command.Now.UTC())
	if !valid {
		return repository.cancelPreparedLease(ctx, client, command)
	}
	return true, nil
}

func (repository *SecurityEmailRepository) preparePasswordResetDelivery(
	ctx context.Context,
	client *generate.Client,
	command ports.SecurityEmailPrepareCommand,
) (bool, error) {
	snapshot, err := client.PasswordResetToken.Query().Where(
		passwordresettoken.ID(command.CredentialID),
	).Select(
		passwordresettoken.FieldID,
		passwordresettoken.FieldUserID,
		passwordresettoken.FieldContactID,
	).Only(ctx)
	if err != nil {
		if generate.IsNotFound(err) {
			return repository.cancelPreparedLease(ctx, client, command)
		}
		return false, commonent.MapEntError(err, securityEmailRepositoryName)
	}
	owner, err := client.User.Query().Where(user.ID(snapshot.UserID)).Select(
		user.FieldID,
		user.FieldStatus,
		user.FieldDeletedAt,
		user.FieldSecurityVersion,
	).ForUpdate().Only(ctx)
	if err != nil {
		if generate.IsNotFound(err) {
			return repository.cancelPreparedLease(ctx, client, command)
		}
		return false, commonent.MapEntError(err, securityEmailRepositoryName)
	}
	verified, err := client.UserRecoveryContact.Query().Where(
		userrecoverycontact.ID(snapshot.ContactID),
		userrecoverycontact.UserID(owner.ID),
		userrecoverycontact.KindEQ(userrecoverycontact.KindEmail),
	).Select(
		userrecoverycontact.FieldID,
		userrecoverycontact.FieldUserID,
		userrecoverycontact.FieldStatus,
		userrecoverycontact.FieldVersion,
	).ForUpdate().Only(ctx)
	if err != nil {
		if generate.IsNotFound(err) {
			return repository.cancelPreparedLease(ctx, client, command)
		}
		return false, commonent.MapEntError(err, securityEmailRepositoryName)
	}
	token, err := client.PasswordResetToken.Query().Where(
		passwordresettoken.ID(command.CredentialID),
	).Select(
		passwordresettoken.FieldID,
		passwordresettoken.FieldUserID,
		passwordresettoken.FieldContactID,
		passwordresettoken.FieldContactVersion,
		passwordresettoken.FieldUserSecurityVersion,
		passwordresettoken.FieldStatus,
		passwordresettoken.FieldExpiresAt,
	).ForUpdate().Only(ctx)
	if err != nil {
		if generate.IsNotFound(err) {
			return repository.cancelPreparedLease(ctx, client, command)
		}
		return false, commonent.MapEntError(err, securityEmailRepositoryName)
	}
	hasPassword, err := activePasswordCredentialExists(ctx, client, owner.ID)
	if err != nil {
		return false, commonent.MapEntError(err, securityEmailRepositoryName)
	}
	valid := owner.Status == user.StatusActive && owner.DeletedAt == nil && hasPassword &&
		owner.SecurityVersion == token.UserSecurityVersion &&
		token.Status == passwordresettoken.StatusIssued && token.ExpiresAt.After(command.Now.UTC())
	valid = valid && token.UserID == owner.ID && token.ContactID == verified.ID &&
		verified.Status == userrecoverycontact.StatusVerified &&
		verified.Version == token.ContactVersion
	if valid {
		latest, latestErr := client.PasswordResetToken.Query().Where(
			passwordresettoken.UserID(token.UserID),
		).Order(
			passwordresettoken.ByCreatedAt(sql.OrderDesc()),
			passwordresettoken.ByID(sql.OrderDesc()),
		).Select(passwordresettoken.FieldID).First(ctx)
		if latestErr != nil && !generate.IsNotFound(latestErr) {
			return false, commonent.MapEntError(latestErr, securityEmailRepositoryName)
		}
		valid = latestErr == nil && latest.ID == token.ID
	}
	delivery, err := repository.lockPreparedDelivery(ctx, client, command)
	if err != nil {
		return false, err
	}
	valid = valid && delivery.LeasedUntil != nil && delivery.LeasedUntil.After(command.Now.UTC()) &&
		delivery.PayloadExpiresAt.After(command.Now.UTC())
	if !valid {
		return repository.cancelPreparedLease(ctx, client, command)
	}
	return true, nil
}

func (repository *SecurityEmailRepository) lockPreparedDelivery(
	ctx context.Context,
	client *generate.Client,
	command ports.SecurityEmailPrepareCommand,
) (*generate.SecurityEmailDelivery, error) {
	delivery, err := client.SecurityEmailDelivery.Query().Where(
		securityEmailPrepareFence(command),
	).Select(
		securityemaildelivery.FieldID,
		securityemaildelivery.FieldPayloadExpiresAt,
		securityemaildelivery.FieldLeasedUntil,
	).ForUpdate().Only(ctx)
	if err != nil {
		if generate.IsNotFound(err) {
			return nil, ports.ErrSecurityEmailLeaseLost
		}
		return nil, commonent.MapEntError(err, securityEmailRepositoryName)
	}
	return delivery, nil
}

func (repository *SecurityEmailRepository) cancelPreparedLease(
	ctx context.Context,
	client *generate.Client,
	command ports.SecurityEmailPrepareCommand,
) (bool, error) {
	updated, err := client.SecurityEmailDelivery.Update().Where(
		securityEmailPrepareFence(command),
	).SetPayloadCiphertext([]byte{0}).
		SetStatus(securityemaildelivery.StatusCancelled).
		SetCancelledAt(command.Now.UTC()).
		SetCiphertextErasedAt(command.Now.UTC()).
		ClearLeaseOwner().
		ClearLeaseToken().
		ClearLeasedUntil().
		Save(ctx)
	if err != nil {
		return false, commonent.MapEntError(err, securityEmailRepositoryName)
	}
	if updated != 1 {
		return false, ports.ErrSecurityEmailLeaseLost
	}
	return false, nil
}

func securityEmailPrepareFence(command ports.SecurityEmailPrepareCommand) predicate.SecurityEmailDelivery {
	return securityemaildelivery.And(
		securityemaildelivery.ID(command.ID),
		securityemaildelivery.StatusEQ(securityemaildelivery.StatusLeased),
		securityemaildelivery.LeaseOwnerEQ(command.Owner),
		securityemaildelivery.LeaseTokenEQ(command.LeaseToken),
		securityemaildelivery.AttemptCountEQ(command.Attempt),
	)
}

func (repository *SecurityEmailRepository) MarkSent(
	ctx context.Context,
	outcome ports.SecurityEmailLeaseOutcome,
) error {
	if !validSecurityEmailOutcome(outcome, false) {
		return fmt.Errorf("%s: invalid sent outcome", securityEmailRepositoryName)
	}
	return repository.finalize(ctx, outcome, securityemaildelivery.StatusSent)
}

func (repository *SecurityEmailRepository) MarkDead(
	ctx context.Context,
	outcome ports.SecurityEmailLeaseOutcome,
) error {
	if !validSecurityEmailOutcome(outcome, true) {
		return fmt.Errorf("%s: invalid dead outcome", securityEmailRepositoryName)
	}
	return repository.finalize(ctx, outcome, securityemaildelivery.StatusDead)
}

func (repository *SecurityEmailRepository) MarkRetry(
	ctx context.Context,
	outcome ports.SecurityEmailLeaseOutcome,
) error {
	if repository == nil || repository.client == nil || ctx == nil ||
		!validSecurityEmailOutcome(outcome, true) || !outcome.NextAttemptAt.After(outcome.At) {
		return fmt.Errorf("%s: invalid retry outcome", securityEmailRepositoryName)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := requireSecurityEmailTransaction(ctx); err != nil {
		return err
	}
	updated, err := repository.client.DB(ctx).SecurityEmailDelivery.Update().
		Where(
			securityEmailLeaseFence(outcome),
			securityemaildelivery.PayloadExpiresAtGT(outcome.NextAttemptAt.UTC()),
		).
		SetStatus(securityemaildelivery.StatusReady).
		SetNextAttemptAt(outcome.NextAttemptAt.UTC()).
		SetLastErrorClass(outcome.ErrorClass).
		ClearLeaseOwner().
		ClearLeaseToken().
		ClearLeasedUntil().
		Save(ctx)
	if err != nil {
		return commonent.MapEntError(err, securityEmailRepositoryName)
	}
	if updated != 1 {
		return ports.ErrSecurityEmailLeaseLost
	}
	return nil
}

func (repository *SecurityEmailRepository) finalize(
	ctx context.Context,
	outcome ports.SecurityEmailLeaseOutcome,
	status securityemaildelivery.Status,
) error {
	if repository == nil || repository.client == nil || ctx == nil {
		return fmt.Errorf("%s: invalid finalization", securityEmailRepositoryName)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := requireSecurityEmailTransaction(ctx); err != nil {
		return err
	}
	update := repository.client.DB(ctx).SecurityEmailDelivery.Update().
		Where(securityEmailLeaseFence(outcome)).
		SetPayloadCiphertext([]byte{0}).
		SetStatus(status).
		SetCiphertextErasedAt(outcome.At.UTC()).
		ClearLeaseOwner().
		ClearLeaseToken().
		ClearLeasedUntil()
	if status == securityemaildelivery.StatusSent {
		update.SetSentAt(outcome.At.UTC()).ClearLastErrorClass()
	} else {
		update.SetDeadAt(outcome.At.UTC()).SetLastErrorClass(outcome.ErrorClass)
	}
	updated, err := update.Save(ctx)
	if err != nil {
		return commonent.MapEntError(err, securityEmailRepositoryName)
	}
	if updated != 1 {
		return ports.ErrSecurityEmailLeaseLost
	}
	return nil
}

func securityEmailLeaseFence(
	outcome ports.SecurityEmailLeaseOutcome,
) func(*sql.Selector) {
	return securityemaildelivery.And(
		securityemaildelivery.ID(outcome.ID),
		securityemaildelivery.StatusEQ(securityemaildelivery.StatusLeased),
		securityemaildelivery.LeaseOwnerEQ(outcome.Owner),
		securityemaildelivery.LeaseTokenEQ(outcome.LeaseToken),
		securityemaildelivery.AttemptCountEQ(outcome.Attempt),
	)
}

func requireSecurityEmailTransaction(ctx context.Context) error {
	if _, err := platformtx.Client(ctx, nil); err != nil {
		return errors.Join(
			platformtx.ErrTransactionRequired,
			fmt.Errorf("%s: transaction required", securityEmailRepositoryName),
		)
	}
	return nil
}

func validSecurityEmailIntent(intent ports.SecurityEmailDeliveryIntent) bool {
	validLinkage := intent.Kind == ports.SecurityEmailKindPasswordReset &&
		intent.PasswordResetTokenID != uuid.Nil && intent.VerificationTokenID == uuid.Nil ||
		(intent.Kind == ports.SecurityEmailKindRecoveryContactVerification ||
			intent.Kind == ports.SecurityEmailKindAccountInvitation) &&
			intent.VerificationTokenID != uuid.Nil && intent.PasswordResetTokenID == uuid.Nil
	return intent.ID != uuid.Nil && validLinkage &&
		validSecurityEmailText(intent.ProviderIdempotencyKey, 128) &&
		len(intent.PayloadCiphertext) > 1 && len(intent.PayloadCiphertext) <= maximumSecurityEmailPayload &&
		validSecurityEmailText(intent.KeyID, 128) && !intent.PayloadExpiresAt.IsZero() &&
		!intent.AvailableAt.IsZero() && intent.PayloadExpiresAt.After(intent.AvailableAt)
}

func validSecurityEmailClaim(claim ports.SecurityEmailClaim) bool {
	return validSecurityEmailText(claim.Owner, 128) && !claim.Now.IsZero() &&
		claim.LeaseDuration > 0 && claim.LeaseDuration <= maximumSecurityEmailLease &&
		claim.Limit > 0 && claim.Limit <= maximumSecurityEmailBatch
}

func validSecurityEmailPrepareCommand(command ports.SecurityEmailPrepareCommand) bool {
	return command.ID != uuid.Nil && command.CredentialID != uuid.Nil &&
		(command.Kind == ports.SecurityEmailKindPasswordReset ||
			command.Kind == ports.SecurityEmailKindRecoveryContactVerification ||
			command.Kind == ports.SecurityEmailKindAccountInvitation) &&
		validSecurityEmailText(command.Owner, 128) && command.LeaseToken != uuid.Nil &&
		command.Attempt > 0 && !command.Now.IsZero()
}

func validSecurityEmailOutcome(outcome ports.SecurityEmailLeaseOutcome, requireClass bool) bool {
	if outcome.ID == uuid.Nil || outcome.LeaseToken == uuid.Nil || outcome.Attempt == 0 ||
		!validSecurityEmailText(outcome.Owner, 128) || outcome.At.IsZero() {
		return false
	}
	if requireClass {
		return validSecurityEmailText(outcome.ErrorClass, 64)
	}
	return outcome.ErrorClass == ""
}

func validSecurityEmailText(value string, maximum int) bool {
	if value == "" || len(value) > maximum || strings.TrimSpace(value) != value {
		return false
	}
	for index := 0; index < len(value); index++ {
		if value[index] <= 0x20 || value[index] >= 0x7f {
			return false
		}
	}
	return true
}

var _ ports.SecurityEmailDeliveryRepository = (*SecurityEmailRepository)(nil)
