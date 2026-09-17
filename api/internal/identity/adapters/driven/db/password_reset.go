package db

import (
	"context"
	"fmt"
	"math"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	commonent "github.com/huynhanx03/go-common/pkg/database/ent"

	dbent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/ent/generate/credential"
	"github.com/huynhanx03/judgify/internal/ent/generate/passwordresettoken"
	"github.com/huynhanx03/judgify/internal/ent/generate/predicate"
	"github.com/huynhanx03/judgify/internal/ent/generate/securityemaildelivery"
	"github.com/huynhanx03/judgify/internal/ent/generate/user"
	"github.com/huynhanx03/judgify/internal/ent/generate/userrecoverycontact"
	entity "github.com/huynhanx03/judgify/internal/identity/domain"
	"github.com/huynhanx03/judgify/internal/ports"
)

const (
	passwordResetRepoName             = "password reset"
	maximumPasswordResetSiblingTokens = 256
)

type PasswordResetRepository struct {
	client *dbent.EntClient
}

func NewPasswordResetRepository(client *dbent.EntClient) ports.PasswordResetRepository {
	return &PasswordResetRepository{client: client}
}

func (repository *PasswordResetRepository) EligibleContact(
	ctx context.Context,
	userID uuid.UUID,
	securityVersion uint64,
) (*entity.RecoveryContact, error) {
	if repository == nil || repository.client == nil || ctx == nil || userID == uuid.Nil ||
		securityVersion == 0 || securityVersion > math.MaxInt64 {
		return nil, fmt.Errorf("%s: invalid eligibility lookup", passwordResetRepoName)
	}
	client := repository.client.DB(ctx)
	eligible, err := client.User.Query().Where(
		user.ID(userID),
		user.StatusEQ(user.StatusActive),
		user.SecurityVersion(int64(securityVersion)),
		user.DeletedAtIsNil(),
	).Exist(ctx)
	if err != nil {
		return nil, commonent.MapEntError(err, passwordResetRepoName)
	}
	if !eligible {
		return nil, ports.ErrPasswordResetRejected
	}
	hasPassword, err := activePasswordCredentialExists(ctx, client, userID)
	if err != nil {
		return nil, commonent.MapEntError(err, passwordResetRepoName)
	}
	if !hasPassword {
		return nil, ports.ErrPasswordResetRejected
	}
	contact, err := client.UserRecoveryContact.Query().Where(
		userrecoverycontact.UserID(userID),
		userrecoverycontact.KindEQ(userrecoverycontact.KindEmail),
		userrecoverycontact.StatusEQ(userrecoverycontact.StatusVerified),
	).Only(ctx)
	if err != nil {
		if generate.IsNotFound(err) {
			return nil, ports.ErrPasswordResetRejected
		}
		return nil, commonent.MapEntError(err, passwordResetRepoName)
	}
	return recoveryContactEntity(contact), nil
}

func (repository *PasswordResetRepository) Issue(
	ctx context.Context,
	command ports.PasswordResetIssue,
) error {
	if repository == nil || repository.client == nil || ctx == nil ||
		command.ID == uuid.Nil || command.UserID == uuid.Nil || command.ContactID == uuid.Nil ||
		command.ContactVersion == 0 || command.ContactVersion > math.MaxInt64 ||
		command.UserSecurityVersion == 0 || command.UserSecurityVersion > math.MaxInt64 ||
		!validKeyedTokenHash(command.Token) || command.IssuedAt.IsZero() ||
		!command.ExpiresAt.After(command.IssuedAt) || command.ShortWindowStart.IsZero() ||
		command.DailyWindowStart.IsZero() || !command.DailyWindowStart.Before(command.ShortWindowStart) ||
		!command.ShortWindowStart.Before(command.IssuedAt) || command.MaximumPerShortWindow < 1 ||
		command.MaximumPerShortWindow > 100 || command.MaximumPerDay < command.MaximumPerShortWindow ||
		command.MaximumPerDay > 1000 {
		return fmt.Errorf("%s: invalid issue command", passwordResetRepoName)
	}
	if err := requireSecurityEmailTransaction(ctx); err != nil {
		return err
	}
	client := repository.client.DB(ctx)
	owner, err := client.User.Query().Where(user.ID(command.UserID)).ForUpdate().Only(ctx)
	if err != nil {
		return commonent.MapEntError(err, passwordResetRepoName)
	}
	if owner.Status != user.StatusActive || owner.DeletedAt != nil ||
		owner.SecurityVersion != int64(command.UserSecurityVersion) {
		return ports.ErrPasswordResetRejected
	}
	contact, err := client.UserRecoveryContact.Query().Where(
		userrecoverycontact.ID(command.ContactID),
		userrecoverycontact.UserID(command.UserID),
		userrecoverycontact.KindEQ(userrecoverycontact.KindEmail),
		userrecoverycontact.StatusEQ(userrecoverycontact.StatusVerified),
		userrecoverycontact.Version(int64(command.ContactVersion)),
	).ForUpdate().Only(ctx)
	if err != nil {
		if generate.IsNotFound(err) {
			return ports.ErrPasswordResetRejected
		}
		return commonent.MapEntError(err, passwordResetRepoName)
	}
	if contact.ID != command.ContactID {
		return ports.ErrPasswordResetRejected
	}
	hasPassword, err := activePasswordCredentialExists(ctx, client, command.UserID)
	if err != nil {
		return commonent.MapEntError(err, passwordResetRepoName)
	}
	if !hasPassword {
		return ports.ErrPasswordResetRejected
	}

	issuedAt := command.IssuedAt.UTC()
	tokens, err := client.PasswordResetToken.Query().Where(
		passwordresettoken.UserID(command.UserID),
		passwordresettoken.Or(
			passwordresettoken.CreatedAtGTE(command.DailyWindowStart.UTC()),
			passwordresettoken.StatusEQ(passwordresettoken.StatusIssued),
		),
	).Select(
		passwordresettoken.FieldID,
		passwordresettoken.FieldStatus,
		passwordresettoken.FieldCreatedAt,
	).Order(
		passwordresettoken.ByCreatedAt(sql.OrderDesc()),
		passwordresettoken.ByID(sql.OrderDesc()),
	).Limit(command.MaximumPerDay + 9).Modify(func(selector *sql.Selector) {
		selector.ForUpdate()
	}).All(ctx)
	if err != nil {
		return commonent.MapEntError(err, passwordResetRepoName)
	}
	shortCount, dailyCount := 0, 0
	issuedIDs := make([]uuid.UUID, 0, len(tokens))
	for _, token := range tokens {
		createdAt := token.CreatedAt.UTC()
		if !createdAt.Before(command.DailyWindowStart.UTC()) {
			dailyCount++
		}
		if !createdAt.Before(command.ShortWindowStart.UTC()) {
			shortCount++
		}
		if token.Status == passwordresettoken.StatusIssued {
			issuedIDs = append(issuedIDs, token.ID)
		}
	}
	if shortCount >= command.MaximumPerShortWindow || dailyCount >= command.MaximumPerDay {
		return ports.ErrPasswordResetRateLimited
	}
	if err := supersedePasswordResetTokens(ctx, client, issuedIDs, issuedAt); err != nil {
		return err
	}
	if _, err := client.PasswordResetToken.Create().
		SetID(command.ID).
		SetUserID(command.UserID).
		SetContactID(command.ContactID).
		SetContactVersion(int64(command.ContactVersion)).
		SetUserSecurityVersion(int64(command.UserSecurityVersion)).
		SetHashKeyID(command.Token.KeyID).
		SetTokenHash(append([]byte(nil), command.Token.Hash...)).
		SetStatus(passwordresettoken.StatusIssued).
		SetExpiresAt(command.ExpiresAt.UTC()).
		SetCreatedAt(issuedAt).
		SetUpdatedAt(issuedAt).
		Save(ctx); err != nil {
		return commonent.MapEntError(err, passwordResetRepoName)
	}
	return nil
}

func supersedePasswordResetTokens(
	ctx context.Context,
	client *generate.Client,
	issuedIDs []uuid.UUID,
	now time.Time,
) error {
	if len(issuedIDs) == 0 {
		return nil
	}
	deliveryIDs, err := lockPasswordResetDeliveries(ctx, client, issuedIDs)
	if err != nil {
		return err
	}
	if _, err := client.PasswordResetToken.Update().Where(
		passwordresettoken.IDIn(issuedIDs...),
		passwordresettoken.StatusEQ(passwordresettoken.StatusIssued),
	).SetStatus(passwordresettoken.StatusSuperseded).
		SetSupersededAt(now.UTC()).
		Save(ctx); err != nil {
		return commonent.MapEntError(err, passwordResetRepoName)
	}
	return cancelPasswordResetDeliveries(ctx, client, deliveryIDs, now)
}

func lockPasswordResetDeliveries(
	ctx context.Context,
	client *generate.Client,
	tokenIDs []uuid.UUID,
) ([]uuid.UUID, error) {
	if len(tokenIDs) == 0 || len(tokenIDs) > maximumPasswordResetSiblingTokens+1 {
		return nil, fmt.Errorf("%s: invalid delivery token batch", passwordResetRepoName)
	}
	deliveries, err := client.SecurityEmailDelivery.Query().Where(
		securityemaildelivery.PasswordResetTokenIDIn(tokenIDs...),
		securityemaildelivery.StatusIn(securityemaildelivery.StatusReady, securityemaildelivery.StatusLeased),
	).Select(securityemaildelivery.FieldID).
		Order(securityemaildelivery.ByID()).Limit(len(tokenIDs) + 1).Modify(func(selector *sql.Selector) {
		selector.ForUpdate()
	}).All(ctx)
	if err != nil {
		return nil, commonent.MapEntError(err, passwordResetRepoName)
	}
	if len(deliveries) > len(tokenIDs) {
		return nil, fmt.Errorf("%s: invalid delivery cardinality", passwordResetRepoName)
	}
	deliveryIDs := make([]uuid.UUID, 0, len(deliveries))
	for _, delivery := range deliveries {
		deliveryIDs = append(deliveryIDs, delivery.ID)
	}
	return deliveryIDs, nil
}

func cancelPasswordResetDeliveries(
	ctx context.Context,
	client *generate.Client,
	deliveryIDs []uuid.UUID,
	now time.Time,
) error {
	if len(deliveryIDs) == 0 {
		return nil
	}
	updated, err := client.SecurityEmailDelivery.Update().Where(
		securityemaildelivery.IDIn(deliveryIDs...),
		securityemaildelivery.StatusIn(securityemaildelivery.StatusReady, securityemaildelivery.StatusLeased),
	).SetPayloadCiphertext([]byte{0}).
		SetStatus(securityemaildelivery.StatusCancelled).
		SetCancelledAt(now.UTC()).
		SetCiphertextErasedAt(now.UTC()).
		ClearLeaseOwner().ClearLeaseToken().ClearLeasedUntil().
		Save(ctx)
	if err != nil {
		return commonent.MapEntError(err, passwordResetRepoName)
	}
	if updated != len(deliveryIDs) {
		return fmt.Errorf("%s: delivery cancellation race", passwordResetRepoName)
	}
	return nil
}

func (repository *PasswordResetRepository) LockIssued(
	ctx context.Context,
	candidates []entity.KeyedTokenHash,
	now time.Time,
) (*entity.PasswordResetIntent, error) {
	if repository == nil || repository.client == nil || ctx == nil ||
		!validKeyedTokenHashes(candidates) || now.IsZero() {
		return nil, fmt.Errorf("%s: invalid consume lookup", passwordResetRepoName)
	}
	if err := requireSecurityEmailTransaction(ctx); err != nil {
		return nil, err
	}
	client := repository.client.DB(ctx)
	tokenPredicate := passwordResetCandidatesPredicate(candidates)
	snapshot, err := client.PasswordResetToken.Query().Where(tokenPredicate).
		Select(
			passwordresettoken.FieldID,
			passwordresettoken.FieldUserID,
			passwordresettoken.FieldContactID,
		).
		Only(ctx)
	if err != nil {
		if generate.IsNotFound(err) {
			return nil, ports.ErrPasswordResetRejected
		}
		return nil, commonent.MapEntError(err, passwordResetRepoName)
	}
	owner, err := client.User.Query().Where(user.ID(snapshot.UserID)).ForUpdate().Only(ctx)
	if err != nil {
		if generate.IsNotFound(err) {
			return nil, ports.ErrPasswordResetRejected
		}
		return nil, commonent.MapEntError(err, passwordResetRepoName)
	}
	contact, err := client.UserRecoveryContact.Query().Where(
		userrecoverycontact.ID(snapshot.ContactID),
	).ForUpdate().Only(ctx)
	if err != nil {
		if generate.IsNotFound(err) {
			return nil, ports.ErrPasswordResetRejected
		}
		return nil, commonent.MapEntError(err, passwordResetRepoName)
	}
	record, err := client.PasswordResetToken.Query().Where(
		passwordresettoken.ID(snapshot.ID),
		tokenPredicate,
	).ForUpdate().Only(ctx)
	if err != nil {
		if generate.IsNotFound(err) {
			return nil, ports.ErrPasswordResetRejected
		}
		return nil, commonent.MapEntError(err, passwordResetRepoName)
	}
	if record.Status != passwordresettoken.StatusIssued || !record.ExpiresAt.After(now.UTC()) ||
		record.UserID != owner.ID || record.UserSecurityVersion < 1 || record.ContactVersion < 1 ||
		owner.Status != user.StatusActive || owner.DeletedAt != nil ||
		owner.SecurityVersion != record.UserSecurityVersion || contact.UserID != record.UserID ||
		contact.Status != userrecoverycontact.StatusVerified || contact.Version != record.ContactVersion {
		return nil, ports.ErrPasswordResetRejected
	}
	hasPassword, err := activePasswordCredentialExists(ctx, client, record.UserID)
	if err != nil {
		return nil, commonent.MapEntError(err, passwordResetRepoName)
	}
	if !hasPassword {
		return nil, ports.ErrPasswordResetRejected
	}
	return &entity.PasswordResetIntent{
		ID: record.ID, UserID: record.UserID, ContactID: record.ContactID,
		ContactVersion:      uint64(record.ContactVersion),
		UserSecurityVersion: uint64(record.UserSecurityVersion),
		ExpiresAt:           record.ExpiresAt.UTC(),
	}, nil
}

func (repository *PasswordResetRepository) ValidateIssued(
	ctx context.Context,
	candidates []entity.KeyedTokenHash,
	now time.Time,
) error {
	if repository == nil || repository.client == nil || ctx == nil ||
		!validKeyedTokenHashes(candidates) || now.IsZero() {
		return fmt.Errorf("%s: invalid validation lookup", passwordResetRepoName)
	}
	exists, err := repository.client.DB(ctx).PasswordResetToken.Query().Where(
		passwordResetCandidatesPredicate(candidates),
		passwordresettoken.StatusEQ(passwordresettoken.StatusIssued),
		passwordresettoken.ExpiresAtGT(now.UTC()),
	).Exist(ctx)
	if err != nil {
		return commonent.MapEntError(err, passwordResetRepoName)
	}
	if !exists {
		return ports.ErrPasswordResetRejected
	}
	return nil
}

func (repository *PasswordResetRepository) Consume(
	ctx context.Context,
	intent *entity.PasswordResetIntent,
	now time.Time,
) error {
	if repository == nil || repository.client == nil || ctx == nil || intent == nil ||
		intent.ID == uuid.Nil || intent.UserID == uuid.Nil || intent.ContactID == uuid.Nil ||
		intent.ContactVersion == 0 || intent.UserSecurityVersion == 0 ||
		intent.UserSecurityVersion >= math.MaxInt64 || now.IsZero() || !intent.ExpiresAt.After(now.UTC()) {
		return ports.ErrPasswordResetRejected
	}
	if err := requireSecurityEmailTransaction(ctx); err != nil {
		return err
	}
	client := repository.client.DB(ctx)
	siblings, err := client.PasswordResetToken.Query().Where(
		passwordresettoken.UserID(intent.UserID),
		passwordresettoken.IDNEQ(intent.ID),
		passwordresettoken.StatusEQ(passwordresettoken.StatusIssued),
	).Select(passwordresettoken.FieldID).
		Order(passwordresettoken.ByID()).Limit(maximumPasswordResetSiblingTokens + 1).
		Modify(func(selector *sql.Selector) { selector.ForUpdate() }).All(ctx)
	if err != nil {
		return commonent.MapEntError(err, passwordResetRepoName)
	}
	if len(siblings) > maximumPasswordResetSiblingTokens {
		return fmt.Errorf("%s: invalid issued sibling cardinality", passwordResetRepoName)
	}
	siblingIDs := make([]uuid.UUID, 0, len(siblings))
	for _, sibling := range siblings {
		siblingIDs = append(siblingIDs, sibling.ID)
	}
	if err := supersedePasswordResetTokens(ctx, client, siblingIDs, now); err != nil {
		return err
	}
	currentDeliveries, err := lockPasswordResetDeliveries(ctx, client, []uuid.UUID{intent.ID})
	if err != nil {
		return err
	}
	if err := cancelPasswordResetDeliveries(ctx, client, currentDeliveries, now); err != nil {
		return err
	}
	updated, err := client.PasswordResetToken.Update().Where(
		passwordresettoken.ID(intent.ID),
		passwordresettoken.UserID(intent.UserID),
		passwordresettoken.ContactID(intent.ContactID),
		passwordresettoken.ContactVersion(int64(intent.ContactVersion)),
		passwordresettoken.UserSecurityVersion(int64(intent.UserSecurityVersion)),
		passwordresettoken.StatusEQ(passwordresettoken.StatusIssued),
		passwordresettoken.ExpiresAtGT(now.UTC()),
	).SetStatus(passwordresettoken.StatusConsumed).
		SetConsumedAt(now.UTC()).
		AddAttemptCount(1).
		SetLastAttemptAt(now.UTC()).
		Save(ctx)
	if err != nil {
		return commonent.MapEntError(err, passwordResetRepoName)
	}
	if updated != 1 {
		return ports.ErrPasswordResetRejected
	}
	updated, err = client.User.Update().Where(
		user.ID(intent.UserID),
		user.StatusEQ(user.StatusActive),
		user.SecurityVersion(int64(intent.UserSecurityVersion)),
	).AddSecurityVersion(1).Save(ctx)
	if err != nil {
		return commonent.MapEntError(err, passwordResetRepoName)
	}
	if updated != 1 {
		return ports.ErrPasswordResetRejected
	}
	return nil
}

func activePasswordCredentialExists(
	ctx context.Context,
	client *generate.Client,
	userID uuid.UUID,
) (bool, error) {
	return client.Credential.Query().Where(
		credential.UserID(userID),
		credential.Type("password"),
		credential.ActiveEQ(true),
		credential.DeletedAtIsNil(),
	).Exist(ctx)
}

func recoveryContactEntity(record *generate.UserRecoveryContact) *entity.RecoveryContact {
	if record == nil || record.Version < 1 {
		return nil
	}
	result := &entity.RecoveryContact{
		ID: record.ID, UserID: record.UserID, Kind: string(record.Kind),
		Purpose: entity.RecoveryContactPurpose(record.Purpose),
		Status:  entity.RecoveryContactStatus(record.Status), Version: uint64(record.Version),
		AddressCiphertext: append([]byte(nil), record.AddressCiphertext...), KeyID: record.KeyID,
		VerifiedAt: record.VerifiedAt, RevokedAt: record.RevokedAt,
		ReplacedContactID: record.ReplacedContactID,
		CreatedAt:         record.CreatedAt.UTC(), UpdatedAt: record.UpdatedAt.UTC(),
	}
	return result
}

func validKeyedTokenHash(candidate entity.KeyedTokenHash) bool {
	return candidate.KeyID != "" && len(candidate.KeyID) <= 128 && len(candidate.Hash) == 32
}

func validKeyedTokenHashes(candidates []entity.KeyedTokenHash) bool {
	if len(candidates) == 0 || len(candidates) > 8 {
		return false
	}
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		if !validKeyedTokenHash(candidate) {
			return false
		}
		if _, exists := seen[candidate.KeyID]; exists {
			return false
		}
		seen[candidate.KeyID] = struct{}{}
	}
	return true
}

func passwordResetCandidatesPredicate(
	candidates []entity.KeyedTokenHash,
) predicate.PasswordResetToken {
	predicates := make([]predicate.PasswordResetToken, 0, len(candidates))
	for _, candidate := range candidates {
		predicates = append(predicates, passwordresettoken.And(
			passwordresettoken.HashKeyID(candidate.KeyID),
			passwordresettoken.TokenHashEQ(append([]byte(nil), candidate.Hash...)),
		))
	}
	return passwordresettoken.Or(predicates...)
}

var _ ports.PasswordResetRepository = (*PasswordResetRepository)(nil)
