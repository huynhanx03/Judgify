package db

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"

	"entgo.io/ent/dialect/sql"

	commonent "github.com/huynhanx03/go-common/pkg/database/ent"

	dbent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/ent/generate/authsession"
	"github.com/huynhanx03/judgify/internal/ent/generate/predicate"
	"github.com/huynhanx03/judgify/internal/ent/generate/recoverycontactcommandreceipt"
	"github.com/huynhanx03/judgify/internal/ent/generate/recoverycontactverificationtoken"
	"github.com/huynhanx03/judgify/internal/ent/generate/securityemaildelivery"
	"github.com/huynhanx03/judgify/internal/ent/generate/user"
	"github.com/huynhanx03/judgify/internal/ent/generate/userrecoverycontact"
	entity "github.com/huynhanx03/judgify/internal/identity/domain"
	"github.com/huynhanx03/judgify/internal/ports"
)

const (
	recoveryContactRepositoryName       = "recovery contact"
	recoveryContactEmailKind            = "email"
	maximumStoredContactCiphertextBytes = 1024
)

type RecoveryContactRepository struct {
	client *dbent.EntClient
}

func NewRecoveryContactRepository(client *dbent.EntClient) *RecoveryContactRepository {
	return &RecoveryContactRepository{client: client}
}

func (repository *RecoveryContactRepository) GetInvitationResendState(
	ctx context.Context,
	userID uuid.UUID,
) (*ports.AccountInvitationResendState, error) {
	if repository == nil || repository.client == nil || ctx == nil || userID == uuid.Nil {
		return nil, ports.ErrAccountInvitationConflict
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	client := repository.client.DB(ctx)
	owner, err := client.User.Query().Where(
		user.ID(userID),
		user.StatusEQ(user.StatusInvited),
		user.DeletedAtIsNil(),
	).Select(
		user.FieldID,
		user.FieldUsername,
		user.FieldStatus,
		user.FieldSecurityVersion,
	).Only(ctx)
	if err != nil {
		if generate.IsNotFound(err) {
			return nil, ports.ErrAccountInvitationConflict
		}
		return nil, commonent.MapEntError(err, recoveryContactRepositoryName)
	}
	contact, err := client.UserRecoveryContact.Query().Where(
		userrecoverycontact.UserID(userID),
		userrecoverycontact.KindEQ(userrecoverycontact.KindEmail),
		userrecoverycontact.PurposeEQ(userrecoverycontact.PurposeInvitation),
		userrecoverycontact.StatusEQ(userrecoverycontact.StatusPending),
	).Only(ctx)
	if err != nil {
		if generate.IsNotFound(err) {
			return nil, ports.ErrAccountInvitationConflict
		}
		return nil, commonent.MapEntError(err, recoveryContactRepositoryName)
	}
	token, err := client.RecoveryContactVerificationToken.Query().Where(
		recoverycontactverificationtoken.ContactID(contact.ID),
		recoverycontactverificationtoken.ContactVersion(contact.Version),
		recoverycontactverificationtoken.PurposeEQ(
			recoverycontactverificationtoken.PurposeInvitation,
		),
		recoverycontactverificationtoken.StatusIn(
			recoverycontactverificationtoken.StatusIssued,
			recoverycontactverificationtoken.StatusExpired,
		),
	).Order(
		recoverycontactverificationtoken.ByCreatedAt(sql.OrderDesc()),
		recoverycontactverificationtoken.ByID(sql.OrderDesc()),
	).First(ctx)
	if err != nil {
		if generate.IsNotFound(err) {
			return nil, ports.ErrAccountInvitationConflict
		}
		return nil, commonent.MapEntError(err, recoveryContactRepositoryName)
	}
	delivery, err := client.SecurityEmailDelivery.Query().Where(
		securityemaildelivery.VerificationTokenID(token.ID),
		securityemaildelivery.KindEQ(securityemaildelivery.KindAccountInvitation),
	).Only(ctx)
	if err != nil {
		// Cleanup may erase a terminal delivery before an administrator
		// resends an expired invitation. The retained token is the durable
		// resend cursor; a missing delivery is valid only after token expiry.
		if !generate.IsNotFound(err) ||
			token.Status != recoverycontactverificationtoken.StatusExpired {
			return nil, commonent.MapEntError(err, recoveryContactRepositoryName)
		}
	}
	deliveryState := entity.RecoveryContactDeliveryFailed
	if delivery != nil {
		switch delivery.Status {
		case securityemaildelivery.StatusReady, securityemaildelivery.StatusLeased:
			deliveryState = entity.RecoveryContactDeliveryQueued
		case securityemaildelivery.StatusSent:
			deliveryState = entity.RecoveryContactDeliverySent
		case securityemaildelivery.StatusDead, securityemaildelivery.StatusCancelled:
			deliveryState = entity.RecoveryContactDeliveryFailed
		default:
			return nil, ports.ErrAccountInvitationConflict
		}
	}
	contactEntity := recoveryContactEntity(contact)
	if contactEntity == nil || owner.SecurityVersion < 1 {
		return nil, ports.ErrAccountInvitationConflict
	}
	return &ports.AccountInvitationResendState{
		UserID: owner.ID, Username: owner.Username,
		UserSecurityVersion: uint64(owner.SecurityVersion),
		Contact:             *contactEntity, LastTokenID: token.ID,
		LastIssuedAt: token.CreatedAt.UTC(), ExpiresAt: token.ExpiresAt.UTC(),
		DeliveryState: deliveryState,
	}, nil
}

func (repository *RecoveryContactRepository) RotateInvitation(
	ctx context.Context,
	command ports.AccountInvitationResendCommand,
) error {
	prepared := command.Prepared
	contact := prepared.Contact
	if repository == nil || repository.client == nil || ctx == nil ||
		command.UserID == uuid.Nil || command.ExpectedTokenID == uuid.Nil ||
		command.ExpectedUserSecurityVersion == 0 ||
		command.ExpectedUserSecurityVersion > math.MaxInt64 ||
		command.ExpectedContactVersion == 0 || command.ExpectedContactVersion >= math.MaxInt64 ||
		contact.ID == uuid.Nil || contact.UserID != command.UserID ||
		contact.Kind != recoveryContactEmailKind ||
		contact.Purpose != entity.RecoveryContactPurposeInvitation ||
		contact.Status != entity.RecoveryContactStatusPending ||
		contact.Version != command.ExpectedContactVersion+1 ||
		len(contact.AddressCiphertext) <= 1 ||
		len(contact.AddressCiphertext) > maximumStoredContactCiphertextBytes ||
		contact.KeyID == "" || prepared.TokenID == uuid.Nil ||
		!validKeyedTokenHash(prepared.Token) || prepared.IssuedAt.IsZero() ||
		!prepared.ExpiresAt.After(prepared.IssuedAt) ||
		prepared.HourlyWindowStart.IsZero() || prepared.DailyWindowStart.IsZero() ||
		!prepared.DailyWindowStart.Before(prepared.HourlyWindowStart) ||
		!prepared.HourlyWindowStart.Before(prepared.IssuedAt) ||
		prepared.MaximumPerHour < 1 || prepared.MaximumPerHour > 100 ||
		prepared.MaximumPerDay < prepared.MaximumPerHour || prepared.MaximumPerDay > 1000 ||
		prepared.Cooldown <= 0 || prepared.Cooldown > time.Hour ||
		prepared.Delivery.ID == uuid.Nil ||
		prepared.Delivery.Kind != ports.SecurityEmailKindAccountInvitation ||
		prepared.Delivery.VerificationTokenID != prepared.TokenID ||
		!prepared.Delivery.PayloadExpiresAt.Equal(prepared.ExpiresAt) {
		return ports.ErrAccountInvitationConflict
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := requireSecurityEmailTransaction(ctx); err != nil {
		return err
	}
	client := repository.client.DB(ctx)
	owner, err := client.User.Query().Where(
		user.ID(command.UserID),
		user.StatusEQ(user.StatusInvited),
		user.SecurityVersionEQ(int64(command.ExpectedUserSecurityVersion)),
		user.DeletedAtIsNil(),
	).Select(user.FieldID).ForUpdate().Only(ctx)
	if err != nil || owner == nil {
		if generate.IsNotFound(err) {
			return ports.ErrAccountInvitationConflict
		}
		return commonent.MapEntError(err, recoveryContactRepositoryName)
	}
	current, err := client.UserRecoveryContact.Query().Where(
		userrecoverycontact.ID(contact.ID),
		userrecoverycontact.UserID(command.UserID),
		userrecoverycontact.KindEQ(userrecoverycontact.KindEmail),
		userrecoverycontact.PurposeEQ(userrecoverycontact.PurposeInvitation),
		userrecoverycontact.StatusEQ(userrecoverycontact.StatusPending),
		userrecoverycontact.Version(int64(command.ExpectedContactVersion)),
	).ForUpdate().Only(ctx)
	if err != nil || current == nil {
		if generate.IsNotFound(err) {
			return ports.ErrAccountInvitationConflict
		}
		return commonent.MapEntError(err, recoveryContactRepositoryName)
	}
	tokens, err := client.RecoveryContactVerificationToken.Query().Where(
		recoverycontactverificationtoken.ContactID(current.ID),
		recoverycontactverificationtoken.PurposeEQ(
			recoverycontactverificationtoken.PurposeInvitation,
		),
		recoverycontactverificationtoken.Or(
			recoverycontactverificationtoken.CreatedAtGTE(prepared.DailyWindowStart.UTC()),
			recoverycontactverificationtoken.StatusEQ(
				recoverycontactverificationtoken.StatusIssued,
			),
			recoverycontactverificationtoken.ID(command.ExpectedTokenID),
		),
	).Order(
		recoverycontactverificationtoken.ByCreatedAt(sql.OrderDesc()),
		recoverycontactverificationtoken.ByID(sql.OrderDesc()),
	).Limit(prepared.MaximumPerDay + 9).Modify(func(selector *sql.Selector) {
		selector.ForUpdate()
	}).All(ctx)
	if err != nil {
		return commonent.MapEntError(err, recoveryContactRepositoryName)
	}
	if len(tokens) == 0 || tokens[0].ID != command.ExpectedTokenID ||
		tokens[0].Status != recoverycontactverificationtoken.StatusIssued &&
			tokens[0].Status != recoverycontactverificationtoken.StatusExpired {
		return ports.ErrAccountInvitationConflict
	}
	if prepared.IssuedAt.Before(tokens[0].CreatedAt.UTC().Add(prepared.Cooldown)) {
		return ports.ErrRecoveryContactRateLimited
	}
	hourly, daily := 0, 0
	supersedableIDs := make([]uuid.UUID, 0, len(tokens))
	for _, token := range tokens {
		createdAt := token.CreatedAt.UTC()
		if !createdAt.Before(prepared.DailyWindowStart.UTC()) {
			daily++
		}
		if !createdAt.Before(prepared.HourlyWindowStart.UTC()) {
			hourly++
		}
		if token.Status == recoverycontactverificationtoken.StatusIssued ||
			token.ID == command.ExpectedTokenID &&
				token.Status == recoverycontactverificationtoken.StatusExpired {
			supersedableIDs = append(supersedableIDs, token.ID)
		}
	}
	if hourly >= prepared.MaximumPerHour || daily >= prepared.MaximumPerDay {
		return ports.ErrRecoveryContactRateLimited
	}
	if err := supersedeRecoveryVerificationTokens(
		ctx, client, supersedableIDs, prepared.IssuedAt,
	); err != nil {
		return err
	}
	updated, err := client.UserRecoveryContact.Update().Where(
		userrecoverycontact.ID(current.ID),
		userrecoverycontact.StatusEQ(userrecoverycontact.StatusPending),
		userrecoverycontact.Version(int64(command.ExpectedContactVersion)),
	).SetVersion(int64(contact.Version)).
		SetAddressCiphertext(append([]byte(nil), contact.AddressCiphertext...)).
		SetKeyID(contact.KeyID).
		SetUpdatedAt(prepared.IssuedAt.UTC()).
		Save(ctx)
	if err != nil {
		return commonent.MapEntError(err, recoveryContactRepositoryName)
	}
	if updated != 1 {
		return ports.ErrAccountInvitationConflict
	}
	if _, err := client.RecoveryContactVerificationToken.Create().
		SetID(prepared.TokenID).
		SetContactID(current.ID).
		SetContactVersion(int64(contact.Version)).
		SetHashKeyID(prepared.Token.KeyID).
		SetTokenHash(append([]byte(nil), prepared.Token.Hash...)).
		SetPurpose(recoverycontactverificationtoken.PurposeInvitation).
		SetStatus(recoverycontactverificationtoken.StatusIssued).
		SetExpiresAt(prepared.ExpiresAt.UTC()).
		SetCreatedAt(prepared.IssuedAt.UTC()).
		SetUpdatedAt(prepared.IssuedAt.UTC()).
		Save(ctx); err != nil {
		return commonent.MapEntError(err, recoveryContactRepositoryName)
	}
	return nil
}

func (repository *RecoveryContactRepository) CreatePending(
	ctx context.Context,
	issue ports.RecoveryContactIssue,
) error {
	if repository == nil || repository.client == nil || ctx == nil ||
		!validRecoveryContactIssue(issue) {
		return fmt.Errorf("%s: invalid registration issue", recoveryContactRepositoryName)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := requireSecurityEmailTransaction(ctx); err != nil {
		return err
	}
	client := repository.client.DB(ctx)
	ownerStatus := user.StatusActive
	if issue.Contact.Purpose == entity.RecoveryContactPurposeInvitation {
		ownerStatus = user.StatusInvited
	}
	owner, err := client.User.Query().Where(
		user.ID(issue.Contact.UserID),
		user.StatusEQ(ownerStatus),
		user.DeletedAtIsNil(),
	).ForUpdate().Only(ctx)
	if err != nil {
		return commonent.MapEntError(err, recoveryContactRepositoryName)
	}
	if owner.ID != issue.Contact.UserID {
		return ports.ErrRecoveryContactRejected
	}
	existing, err := client.UserRecoveryContact.Query().Where(
		userrecoverycontact.UserID(issue.Contact.UserID),
		userrecoverycontact.KindEQ(userrecoverycontact.KindEmail),
		userrecoverycontact.StatusIn(
			userrecoverycontact.StatusPending,
			userrecoverycontact.StatusVerified,
		),
	).Order(userrecoverycontact.ByID()).Modify(func(selector *sql.Selector) {
		selector.ForUpdate()
	}).All(ctx)
	if err != nil {
		return commonent.MapEntError(err, recoveryContactRepositoryName)
	}
	if len(existing) != 0 {
		return ports.ErrRecoveryContactRejected
	}
	createdAt := issue.IssuedAt.UTC()
	if _, err := client.UserRecoveryContact.Create().
		SetID(issue.Contact.ID).
		SetUserID(issue.Contact.UserID).
		SetKind(userrecoverycontact.KindEmail).
		SetPurpose(userrecoverycontact.Purpose(issue.Contact.Purpose)).
		SetLookupHash(append([]byte(nil), issue.LookupHash...)).
		SetFingerprintHash(append([]byte(nil), issue.FingerprintHash...)).
		SetAddressCiphertext(append([]byte(nil), issue.Contact.AddressCiphertext...)).
		SetKeyID(issue.Contact.KeyID).
		SetStatus(userrecoverycontact.StatusPending).
		SetVersion(int64(issue.Contact.Version)).
		SetCreatedAt(createdAt).
		SetUpdatedAt(createdAt).
		Save(ctx); err != nil {
		return commonent.MapEntError(err, recoveryContactRepositoryName)
	}
	if _, err := client.RecoveryContactVerificationToken.Create().
		SetID(issue.TokenID).
		SetContactID(issue.Contact.ID).
		SetContactVersion(int64(issue.Contact.Version)).
		SetHashKeyID(issue.Token.KeyID).
		SetTokenHash(append([]byte(nil), issue.Token.Hash...)).
		SetPurpose(recoverycontactverificationtoken.Purpose(issue.Contact.Purpose)).
		SetStatus(recoverycontactverificationtoken.StatusIssued).
		SetExpiresAt(issue.ExpiresAt.UTC()).
		SetCreatedAt(createdAt).
		SetUpdatedAt(createdAt).
		Save(ctx); err != nil {
		return commonent.MapEntError(err, recoveryContactRepositoryName)
	}
	return nil
}

func (repository *RecoveryContactRepository) GetState(
	ctx context.Context,
	userID, sessionID uuid.UUID,
	now time.Time,
) (*ports.RecoveryContactState, error) {
	if repository == nil || repository.client == nil || ctx == nil ||
		userID == uuid.Nil || sessionID == uuid.Nil || now.IsZero() {
		return nil, fmt.Errorf("%s: invalid management lookup", recoveryContactRepositoryName)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	now = now.UTC()
	client := repository.client.DB(ctx)
	session, err := client.AuthSession.Query().Where(
		authsession.ID(sessionID),
		authsession.UserID(userID),
		authsession.StatusEQ(authsession.StatusActive),
		authsession.IdleExpiresAtGT(now),
		authsession.AbsoluteExpiresAtGT(now),
	).WithUser(func(query *generate.UserQuery) {
		query.Select(
			user.FieldID,
			user.FieldStatus,
			user.FieldSecurityVersion,
			user.FieldDeletedAt,
		)
	}).Only(ctx)
	if err != nil {
		if generate.IsNotFound(err) {
			return nil, ports.ErrRecoveryContactRejected
		}
		return nil, commonent.MapEntError(err, recoveryContactRepositoryName)
	}
	owner, err := session.Edges.UserOrErr()
	if err != nil {
		return nil, commonent.MapEntError(err, recoveryContactRepositoryName)
	}
	if owner.ID != userID || owner.Status != user.StatusActive || owner.DeletedAt != nil ||
		owner.SecurityVersion < 1 || session.UserSecurityVersion != owner.SecurityVersion {
		return nil, ports.ErrRecoveryContactRejected
	}

	contacts, err := client.UserRecoveryContact.Query().Where(
		userrecoverycontact.UserID(userID),
		userrecoverycontact.KindEQ(userrecoverycontact.KindEmail),
		userrecoverycontact.StatusIn(
			userrecoverycontact.StatusPending,
			userrecoverycontact.StatusVerified,
		),
	).Order(userrecoverycontact.ByID()).All(ctx)
	if err != nil {
		return nil, commonent.MapEntError(err, recoveryContactRepositoryName)
	}
	if len(contacts) > 2 {
		return nil, fmt.Errorf("%s: invalid active contact cardinality", recoveryContactRepositoryName)
	}
	state := &ports.RecoveryContactState{
		UserID: userID, SessionID: sessionID,
		LastReauthenticatedAt: session.LastReauthenticatedAt.UTC(),
	}
	for _, record := range contacts {
		contact := recoveryContactEntity(record)
		if contact == nil {
			return nil, fmt.Errorf("%s: invalid contact projection", recoveryContactRepositoryName)
		}
		switch contact.Status {
		case entity.RecoveryContactStatusVerified:
			if state.Current != nil {
				return nil, fmt.Errorf("%s: duplicate verified contact", recoveryContactRepositoryName)
			}
			state.Current = contact
		case entity.RecoveryContactStatusPending:
			if state.Pending != nil {
				return nil, fmt.Errorf("%s: duplicate pending contact", recoveryContactRepositoryName)
			}
			state.Pending = contact
		default:
			return nil, fmt.Errorf("%s: invalid active contact status", recoveryContactRepositoryName)
		}
	}
	hasPassword, err := activePasswordCredentialExists(ctx, client, userID)
	if err != nil {
		return nil, commonent.MapEntError(err, recoveryContactRepositoryName)
	}
	state.CanRemove = state.Current != nil && state.Pending == nil && !hasPassword
	if state.Pending == nil {
		return state, nil
	}
	token, err := client.RecoveryContactVerificationToken.Query().Where(
		recoverycontactverificationtoken.ContactID(state.Pending.ID),
		recoverycontactverificationtoken.ContactVersion(int64(state.Pending.Version)),
	).Order(
		recoverycontactverificationtoken.ByCreatedAt(sql.OrderDesc()),
		recoverycontactverificationtoken.ByID(sql.OrderDesc()),
	).First(ctx)
	if err != nil {
		return nil, commonent.MapEntError(err, recoveryContactRepositoryName)
	}
	state.PendingVerificationExpiresAt = token.ExpiresAt.UTC()
	state.PendingLastIssuedAt = token.CreatedAt.UTC()
	delivery, err := client.SecurityEmailDelivery.Query().Where(
		securityemaildelivery.VerificationTokenID(token.ID),
	).Order(
		securityemaildelivery.ByCreatedAt(sql.OrderDesc()),
		securityemaildelivery.ByID(sql.OrderDesc()),
	).First(ctx)
	if err != nil {
		return nil, commonent.MapEntError(err, recoveryContactRepositoryName)
	}
	switch delivery.Status {
	case securityemaildelivery.StatusReady, securityemaildelivery.StatusLeased:
		state.PendingDeliveryState = entity.RecoveryContactDeliveryQueued
	case securityemaildelivery.StatusSent:
		state.PendingDeliveryState = entity.RecoveryContactDeliverySent
	case securityemaildelivery.StatusDead, securityemaildelivery.StatusCancelled:
		state.PendingDeliveryState = entity.RecoveryContactDeliveryFailed
	default:
		return nil, fmt.Errorf("%s: invalid delivery state", recoveryContactRepositoryName)
	}
	return state, nil
}

func (repository *RecoveryContactRepository) VerifyIssued(
	ctx context.Context,
	candidates []entity.KeyedTokenHash,
	now time.Time,
) (*entity.RecoveryContactVerificationResult, error) {
	if repository == nil || repository.client == nil || ctx == nil ||
		!validKeyedTokenHashes(candidates) || now.IsZero() {
		return nil, fmt.Errorf("%s: invalid verification lookup", recoveryContactRepositoryName)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := requireSecurityEmailTransaction(ctx); err != nil {
		return nil, err
	}
	client := repository.client.DB(ctx)
	tokenPredicate := recoveryVerificationCandidatesPredicate(candidates)
	snapshot, err := client.RecoveryContactVerificationToken.Query().Where(tokenPredicate).
		Select(
			recoverycontactverificationtoken.FieldID,
			recoverycontactverificationtoken.FieldContactID,
		).
		Only(ctx)
	if err != nil {
		if generate.IsNotFound(err) {
			return nil, ports.ErrRecoveryContactRejected
		}
		return nil, commonent.MapEntError(err, recoveryContactRepositoryName)
	}
	contactSnapshot, err := client.UserRecoveryContact.Query().Where(
		userrecoverycontact.ID(snapshot.ContactID),
	).Select(
		userrecoverycontact.FieldID,
		userrecoverycontact.FieldUserID,
	).Only(ctx)
	if err != nil {
		if generate.IsNotFound(err) {
			return nil, ports.ErrRecoveryContactRejected
		}
		return nil, commonent.MapEntError(err, recoveryContactRepositoryName)
	}
	owner, err := client.User.Query().Where(user.ID(contactSnapshot.UserID)).ForUpdate().Only(ctx)
	if err != nil {
		if generate.IsNotFound(err) {
			return nil, ports.ErrRecoveryContactRejected
		}
		return nil, commonent.MapEntError(err, recoveryContactRepositoryName)
	}
	contacts, err := client.UserRecoveryContact.Query().Where(
		userrecoverycontact.UserID(owner.ID),
		userrecoverycontact.KindEQ(userrecoverycontact.KindEmail),
	).Order(userrecoverycontact.ByID()).Modify(func(selector *sql.Selector) {
		selector.ForUpdate()
	}).All(ctx)
	if err != nil {
		return nil, commonent.MapEntError(err, recoveryContactRepositoryName)
	}
	record, err := client.RecoveryContactVerificationToken.Query().Where(
		recoverycontactverificationtoken.ID(snapshot.ID),
		tokenPredicate,
	).ForUpdate().Only(ctx)
	if err != nil {
		if generate.IsNotFound(err) {
			return nil, ports.ErrRecoveryContactRejected
		}
		return nil, commonent.MapEntError(err, recoveryContactRepositoryName)
	}
	now = now.UTC()
	if owner.Status != user.StatusActive || owner.DeletedAt != nil ||
		record.Status != recoverycontactverificationtoken.StatusIssued ||
		!record.ExpiresAt.After(now) || record.ContactVersion < 1 {
		return nil, ports.ErrRecoveryContactRejected
	}

	var candidate *generate.UserRecoveryContact
	var current *generate.UserRecoveryContact
	for _, contact := range contacts {
		if contact.ID == record.ContactID {
			candidate = contact
		}
		if contact.Status == userrecoverycontact.StatusVerified {
			if current != nil {
				return nil, ports.ErrRecoveryContactRejected
			}
			current = contact
		}
	}
	if candidate == nil || candidate.UserID != owner.ID ||
		candidate.Status != userrecoverycontact.StatusPending ||
		candidate.Version != record.ContactVersion ||
		string(candidate.Purpose) != string(record.Purpose) {
		return nil, ports.ErrRecoveryContactRejected
	}
	latest, err := client.RecoveryContactVerificationToken.Query().Where(
		recoverycontactverificationtoken.ContactID(record.ContactID),
		recoverycontactverificationtoken.ContactVersion(record.ContactVersion),
		recoverycontactverificationtoken.StatusEQ(recoverycontactverificationtoken.StatusIssued),
	).Order(
		recoverycontactverificationtoken.ByCreatedAt(sql.OrderDesc()),
		recoverycontactverificationtoken.ByID(sql.OrderDesc()),
	).First(ctx)
	if err != nil || latest.ID != record.ID {
		return nil, ports.ErrRecoveryContactRejected
	}
	purpose := entity.RecoveryContactPurpose(candidate.Purpose)
	switch purpose {
	case entity.RecoveryContactPurposeRegistration:
		if current != nil {
			return nil, ports.ErrRecoveryContactRejected
		}
	case entity.RecoveryContactPurposeReplacement:
		if current == nil || current.ID == candidate.ID {
			return nil, ports.ErrRecoveryContactRejected
		}
	default:
		return nil, ports.ErrRecoveryContactRejected
	}

	if current != nil {
		updated, updateErr := client.UserRecoveryContact.Update().Where(
			userrecoverycontact.ID(current.ID),
			userrecoverycontact.StatusEQ(userrecoverycontact.StatusVerified),
		).SetStatus(userrecoverycontact.StatusRevoked).
			SetRevokedAt(now).
			SetReplacedContactID(candidate.ID).
			Save(ctx)
		if updateErr != nil {
			return nil, commonent.MapEntError(updateErr, recoveryContactRepositoryName)
		}
		if updated != 1 {
			return nil, ports.ErrRecoveryContactRejected
		}
	}
	updated, err := client.UserRecoveryContact.Update().Where(
		userrecoverycontact.ID(candidate.ID),
		userrecoverycontact.StatusEQ(userrecoverycontact.StatusPending),
		userrecoverycontact.Version(record.ContactVersion),
	).SetStatus(userrecoverycontact.StatusVerified).
		SetVerifiedAt(now).
		Save(ctx)
	if err != nil {
		return nil, commonent.MapEntError(err, recoveryContactRepositoryName)
	}
	if updated != 1 {
		return nil, ports.ErrRecoveryContactRejected
	}
	updated, err = client.RecoveryContactVerificationToken.Update().Where(
		recoverycontactverificationtoken.ID(record.ID),
		recoverycontactverificationtoken.StatusEQ(recoverycontactverificationtoken.StatusIssued),
		recoverycontactverificationtoken.ExpiresAtGT(now),
	).SetStatus(recoverycontactverificationtoken.StatusConsumed).
		SetConsumedAt(now).
		AddAttemptCount(1).
		SetLastAttemptAt(now).
		Save(ctx)
	if err != nil {
		return nil, commonent.MapEntError(err, recoveryContactRepositoryName)
	}
	if updated != 1 {
		return nil, ports.ErrRecoveryContactRejected
	}
	if _, err := client.RecoveryContactVerificationToken.Update().Where(
		recoverycontactverificationtoken.ContactID(record.ContactID),
		recoverycontactverificationtoken.IDNEQ(record.ID),
		recoverycontactverificationtoken.StatusEQ(recoverycontactverificationtoken.StatusIssued),
	).SetStatus(recoverycontactverificationtoken.StatusSuperseded).
		SetSupersededAt(now).
		Save(ctx); err != nil {
		return nil, commonent.MapEntError(err, recoveryContactRepositoryName)
	}
	verificationIDs, err := client.RecoveryContactVerificationToken.Query().Where(
		recoverycontactverificationtoken.ContactID(record.ContactID),
	).IDs(ctx)
	if err != nil {
		return nil, commonent.MapEntError(err, recoveryContactRepositoryName)
	}
	if len(verificationIDs) > 0 {
		if _, err := client.SecurityEmailDelivery.Update().Where(
			securityemaildelivery.VerificationTokenIDIn(verificationIDs...),
			securityemaildelivery.StatusIn(
				securityemaildelivery.StatusReady,
				securityemaildelivery.StatusLeased,
			),
		).SetPayloadCiphertext([]byte{0}).
			SetStatus(securityemaildelivery.StatusCancelled).
			SetCancelledAt(now).
			SetCiphertextErasedAt(now).
			ClearLeaseOwner().
			ClearLeaseToken().
			ClearLeasedUntil().
			Save(ctx); err != nil {
			return nil, commonent.MapEntError(err, recoveryContactRepositoryName)
		}
	}
	return &entity.RecoveryContactVerificationResult{
		UserID: owner.ID, ContactID: candidate.ID, Purpose: purpose,
		Version: uint64(candidate.Version),
	}, nil
}

func (repository *RecoveryContactRepository) LockInvitation(
	ctx context.Context,
	candidates []entity.KeyedTokenHash,
	now time.Time,
) (*ports.AccountInvitationAuthority, error) {
	if repository == nil || repository.client == nil || ctx == nil ||
		!validKeyedTokenHashes(candidates) || now.IsZero() {
		return nil, fmt.Errorf("%s: invalid invitation lookup", recoveryContactRepositoryName)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := requireSecurityEmailTransaction(ctx); err != nil {
		return nil, err
	}
	client := repository.client.DB(ctx)
	predicate := recoveryVerificationCandidatesPredicate(candidates)
	snapshot, err := client.RecoveryContactVerificationToken.Query().Where(
		predicate,
		recoverycontactverificationtoken.PurposeEQ(
			recoverycontactverificationtoken.PurposeInvitation,
		),
	).Select(
		recoverycontactverificationtoken.FieldID,
		recoverycontactverificationtoken.FieldContactID,
	).Only(ctx)
	if err != nil {
		if generate.IsNotFound(err) {
			return nil, ports.ErrAccountInvitationRejected
		}
		return nil, commonent.MapEntError(err, recoveryContactRepositoryName)
	}
	contactSnapshot, err := client.UserRecoveryContact.Query().Where(
		userrecoverycontact.ID(snapshot.ContactID),
		userrecoverycontact.PurposeEQ(userrecoverycontact.PurposeInvitation),
	).Select(
		userrecoverycontact.FieldID,
		userrecoverycontact.FieldUserID,
	).Only(ctx)
	if err != nil {
		if generate.IsNotFound(err) {
			return nil, ports.ErrAccountInvitationRejected
		}
		return nil, commonent.MapEntError(err, recoveryContactRepositoryName)
	}
	owner, err := client.User.Query().Where(
		user.ID(contactSnapshot.UserID),
		user.DeletedAtIsNil(),
	).Select(
		user.FieldID,
		user.FieldStatus,
		user.FieldSecurityVersion,
		user.FieldDeletedAt,
	).ForUpdate().Only(ctx)
	if err != nil {
		if generate.IsNotFound(err) {
			return nil, ports.ErrAccountInvitationRejected
		}
		return nil, commonent.MapEntError(err, recoveryContactRepositoryName)
	}
	contact, err := client.UserRecoveryContact.Query().Where(
		userrecoverycontact.ID(contactSnapshot.ID),
		userrecoverycontact.UserID(owner.ID),
		userrecoverycontact.PurposeEQ(userrecoverycontact.PurposeInvitation),
	).Select(
		userrecoverycontact.FieldID,
		userrecoverycontact.FieldUserID,
		userrecoverycontact.FieldPurpose,
		userrecoverycontact.FieldStatus,
		userrecoverycontact.FieldVersion,
		userrecoverycontact.FieldVerifiedAt,
	).ForUpdate().Only(ctx)
	if err != nil {
		if generate.IsNotFound(err) {
			return nil, ports.ErrAccountInvitationRejected
		}
		return nil, commonent.MapEntError(err, recoveryContactRepositoryName)
	}
	token, err := client.RecoveryContactVerificationToken.Query().Where(
		recoverycontactverificationtoken.ID(snapshot.ID),
		predicate,
		recoverycontactverificationtoken.PurposeEQ(
			recoverycontactverificationtoken.PurposeInvitation,
		),
	).Select(
		recoverycontactverificationtoken.FieldID,
		recoverycontactverificationtoken.FieldContactID,
		recoverycontactverificationtoken.FieldContactVersion,
		recoverycontactverificationtoken.FieldPurpose,
		recoverycontactverificationtoken.FieldStatus,
		recoverycontactverificationtoken.FieldExpiresAt,
		recoverycontactverificationtoken.FieldConsumedAt,
	).ForUpdate().Only(ctx)
	if err != nil {
		if generate.IsNotFound(err) {
			return nil, ports.ErrAccountInvitationRejected
		}
		return nil, commonent.MapEntError(err, recoveryContactRepositoryName)
	}
	if owner.SecurityVersion < 1 || token.ContactID != contact.ID ||
		token.ContactVersion != contact.Version ||
		contact.UserID != owner.ID ||
		contact.Purpose != userrecoverycontact.PurposeInvitation {
		return nil, ports.ErrAccountInvitationRejected
	}
	now = now.UTC()
	authority := &ports.AccountInvitationAuthority{
		TokenID: token.ID, ContactID: contact.ID, UserID: owner.ID,
		ContactVersion:      uint64(contact.Version),
		UserSecurityVersion: uint64(owner.SecurityVersion),
	}
	switch token.Status {
	case recoverycontactverificationtoken.StatusIssued:
		if owner.Status != user.StatusInvited ||
			contact.Status != userrecoverycontact.StatusPending ||
			!token.ExpiresAt.After(now) || token.ConsumedAt != nil ||
			contact.VerifiedAt != nil {
			return nil, ports.ErrAccountInvitationRejected
		}
		latest, latestErr := client.RecoveryContactVerificationToken.Query().Where(
			recoverycontactverificationtoken.ContactID(contact.ID),
			recoverycontactverificationtoken.ContactVersion(contact.Version),
			recoverycontactverificationtoken.StatusEQ(
				recoverycontactverificationtoken.StatusIssued,
			),
		).Order(
			recoverycontactverificationtoken.ByCreatedAt(sql.OrderDesc()),
			recoverycontactverificationtoken.ByID(sql.OrderDesc()),
		).Select(recoverycontactverificationtoken.FieldID).First(ctx)
		if latestErr != nil || latest.ID != token.ID {
			return nil, ports.ErrAccountInvitationRejected
		}
	case recoverycontactverificationtoken.StatusConsumed:
		if owner.Status != user.StatusActive ||
			contact.Status != userrecoverycontact.StatusVerified ||
			contact.VerifiedAt == nil || token.ConsumedAt == nil ||
			token.ConsumedAt.After(token.ExpiresAt) ||
			!contact.VerifiedAt.Equal(*token.ConsumedAt) {
			return nil, ports.ErrAccountInvitationRejected
		}
		authority.Accepted = true
	default:
		return nil, ports.ErrAccountInvitationRejected
	}
	return authority, nil
}

func (repository *RecoveryContactRepository) AcceptInvitation(
	ctx context.Context,
	authority ports.AccountInvitationAuthority,
	now time.Time,
) (*entity.User, error) {
	if repository == nil || repository.client == nil || ctx == nil ||
		authority.Accepted || authority.TokenID == uuid.Nil ||
		authority.ContactID == uuid.Nil || authority.UserID == uuid.Nil ||
		authority.ContactVersion == 0 || authority.ContactVersion > math.MaxInt64 ||
		authority.UserSecurityVersion == 0 || authority.UserSecurityVersion >= math.MaxInt64 ||
		now.IsZero() {
		return nil, ports.ErrAccountInvitationConflict
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := requireSecurityEmailTransaction(ctx); err != nil {
		return nil, err
	}
	client := repository.client.DB(ctx)
	hasPassword, err := activePasswordCredentialExists(ctx, client, authority.UserID)
	if err != nil {
		return nil, commonent.MapEntError(err, recoveryContactRepositoryName)
	}
	if !hasPassword {
		return nil, ports.ErrAccountInvitationConflict
	}
	now = now.UTC()
	updated, err := client.UserRecoveryContact.Update().Where(
		userrecoverycontact.ID(authority.ContactID),
		userrecoverycontact.UserID(authority.UserID),
		userrecoverycontact.PurposeEQ(userrecoverycontact.PurposeInvitation),
		userrecoverycontact.StatusEQ(userrecoverycontact.StatusPending),
		userrecoverycontact.Version(int64(authority.ContactVersion)),
	).SetStatus(userrecoverycontact.StatusVerified).
		SetVerifiedAt(now).
		Save(ctx)
	if err != nil {
		return nil, commonent.MapEntError(err, recoveryContactRepositoryName)
	}
	if updated != 1 {
		return nil, ports.ErrAccountInvitationConflict
	}
	updated, err = client.RecoveryContactVerificationToken.Update().Where(
		recoverycontactverificationtoken.ID(authority.TokenID),
		recoverycontactverificationtoken.ContactID(authority.ContactID),
		recoverycontactverificationtoken.ContactVersion(int64(authority.ContactVersion)),
		recoverycontactverificationtoken.PurposeEQ(
			recoverycontactverificationtoken.PurposeInvitation,
		),
		recoverycontactverificationtoken.StatusEQ(
			recoverycontactverificationtoken.StatusIssued,
		),
		recoverycontactverificationtoken.ExpiresAtGT(now),
	).SetStatus(recoverycontactverificationtoken.StatusConsumed).
		SetConsumedAt(now).
		AddAttemptCount(1).
		SetLastAttemptAt(now).
		Save(ctx)
	if err != nil {
		return nil, commonent.MapEntError(err, recoveryContactRepositoryName)
	}
	if updated != 1 {
		return nil, ports.ErrAccountInvitationConflict
	}
	owner, err := client.User.UpdateOneID(authority.UserID).Where(
		user.StatusEQ(user.StatusInvited),
		user.SecurityVersionEQ(int64(authority.UserSecurityVersion)),
		user.DeletedAtIsNil(),
	).SetStatus(user.StatusActive).
		SetSecurityVersion(int64(authority.UserSecurityVersion + 1)).
		Save(ctx)
	if err != nil {
		return nil, commonent.MapEntError(err, recoveryContactRepositoryName)
	}
	if owner == nil || owner.Status != user.StatusActive ||
		owner.SecurityVersion != int64(authority.UserSecurityVersion+1) {
		return nil, ports.ErrAccountInvitationConflict
	}
	if _, err := client.SecurityEmailDelivery.Update().Where(
		securityemaildelivery.VerificationTokenID(authority.TokenID),
		securityemaildelivery.StatusIn(
			securityemaildelivery.StatusReady,
			securityemaildelivery.StatusLeased,
		),
	).SetPayloadCiphertext([]byte{0}).
		SetStatus(securityemaildelivery.StatusCancelled).
		SetCancelledAt(now).
		SetCiphertextErasedAt(now).
		ClearLeaseOwner().
		ClearLeaseToken().
		ClearLeasedUntil().
		Save(ctx); err != nil {
		return nil, commonent.MapEntError(err, recoveryContactRepositoryName)
	}
	return ToUserEntity(owner), nil
}

func (repository *RecoveryContactRepository) RevokeInvitation(
	ctx context.Context,
	userID uuid.UUID,
	now time.Time,
) error {
	if repository == nil || repository.client == nil || ctx == nil ||
		userID == uuid.Nil || now.IsZero() {
		return ports.ErrAccountInvitationConflict
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := requireSecurityEmailTransaction(ctx); err != nil {
		return err
	}
	client := repository.client.DB(ctx)
	owner, err := client.User.Query().Where(
		user.ID(userID),
		user.StatusEQ(user.StatusInvited),
		user.DeletedAtIsNil(),
	).Select(user.FieldID).ForUpdate().Only(ctx)
	if err != nil || owner == nil {
		if generate.IsNotFound(err) {
			return ports.ErrAccountInvitationConflict
		}
		return commonent.MapEntError(err, recoveryContactRepositoryName)
	}
	contact, err := client.UserRecoveryContact.Query().Where(
		userrecoverycontact.UserID(userID),
		userrecoverycontact.PurposeEQ(userrecoverycontact.PurposeInvitation),
		userrecoverycontact.StatusEQ(userrecoverycontact.StatusPending),
	).ForUpdate().Only(ctx)
	if err != nil || contact == nil {
		if generate.IsNotFound(err) {
			return ports.ErrAccountInvitationConflict
		}
		return commonent.MapEntError(err, recoveryContactRepositoryName)
	}
	tokenIDs, err := client.RecoveryContactVerificationToken.Query().Where(
		recoverycontactverificationtoken.ContactID(contact.ID),
		recoverycontactverificationtoken.StatusEQ(
			recoverycontactverificationtoken.StatusIssued,
		),
	).ForUpdate().IDs(ctx)
	if err != nil {
		return commonent.MapEntError(err, recoveryContactRepositoryName)
	}
	now = now.UTC()
	if len(tokenIDs) > 0 {
		if _, err := client.RecoveryContactVerificationToken.Update().Where(
			recoverycontactverificationtoken.IDIn(tokenIDs...),
			recoverycontactverificationtoken.StatusEQ(
				recoverycontactverificationtoken.StatusIssued,
			),
		).SetStatus(recoverycontactverificationtoken.StatusSuperseded).
			SetSupersededAt(now).
			Save(ctx); err != nil {
			return commonent.MapEntError(err, recoveryContactRepositoryName)
		}
		if _, err := client.SecurityEmailDelivery.Update().Where(
			securityemaildelivery.VerificationTokenIDIn(tokenIDs...),
			securityemaildelivery.StatusIn(
				securityemaildelivery.StatusReady,
				securityemaildelivery.StatusLeased,
			),
		).SetPayloadCiphertext([]byte{0}).
			SetStatus(securityemaildelivery.StatusCancelled).
			SetCancelledAt(now).
			SetCiphertextErasedAt(now).
			ClearLeaseOwner().
			ClearLeaseToken().
			ClearLeasedUntil().
			Save(ctx); err != nil {
			return commonent.MapEntError(err, recoveryContactRepositoryName)
		}
	}
	updated, err := client.UserRecoveryContact.Update().Where(
		userrecoverycontact.ID(contact.ID),
		userrecoverycontact.StatusEQ(userrecoverycontact.StatusPending),
	).SetStatus(userrecoverycontact.StatusRevoked).
		SetRevokedAt(now).
		SetAddressCiphertext([]byte{0}).
		Save(ctx)
	if err != nil {
		return commonent.MapEntError(err, recoveryContactRepositoryName)
	}
	if updated != 1 {
		return ports.ErrAccountInvitationConflict
	}
	return nil
}

func (repository *RecoveryContactRepository) StartReplacement(
	ctx context.Context,
	command ports.RecoveryContactReplacementCommand,
) (bool, error) {
	if repository == nil || repository.client == nil || ctx == nil ||
		command.Command.Kind != ports.RecoveryContactCommandReplace ||
		!validRecoveryMutationCommand(command.Command) ||
		command.ExpectedCurrentVersion == 0 || command.ExpectedCurrentVersion >= math.MaxInt64 ||
		command.ReplacementWindowStart.IsZero() ||
		!command.ReplacementWindowStart.Before(command.Command.Now) ||
		command.MaximumReplacements < 1 || command.MaximumReplacements > 100 ||
		!validReplacementRecoveryIssue(command.Issue, command.Command.UserID, command.ExpectedCurrentVersion+1) {
		return false, fmt.Errorf("%s: invalid replacement command", recoveryContactRepositoryName)
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if err := requireSecurityEmailTransaction(ctx); err != nil {
		return false, err
	}
	client := repository.client.DB(ctx)
	replayed, err := lockRecoveryMutationAuthority(ctx, client, command.Command)
	if err != nil || replayed {
		return false, err
	}
	contacts, err := activeRecoveryContactsForUpdate(ctx, client, command.Command.UserID)
	if err != nil {
		return false, err
	}
	current, pending, err := selectActiveRecoveryContacts(contacts)
	if err != nil || current == nil || current.Version != int64(command.ExpectedCurrentVersion) {
		return false, ports.ErrRecoveryContactVersionConflict
	}
	if hmac.Equal(current.FingerprintHash, command.Issue.FingerprintHash) ||
		pending != nil && hmac.Equal(pending.FingerprintHash, command.Issue.FingerprintHash) {
		return false, ports.ErrRecoveryContactVersionConflict
	}
	count, err := client.UserRecoveryContact.Query().Where(
		userrecoverycontact.UserID(command.Command.UserID),
		userrecoverycontact.PurposeEQ(userrecoverycontact.PurposeReplacement),
		userrecoverycontact.CreatedAtGTE(command.ReplacementWindowStart.UTC()),
	).Count(ctx)
	if err != nil {
		return false, commonent.MapEntError(err, recoveryContactRepositoryName)
	}
	if count >= command.MaximumReplacements {
		return false, ports.ErrRecoveryContactRateLimited
	}
	if pending != nil {
		if err := retirePendingRecoveryContact(ctx, client, pending, command.Command.Now); err != nil {
			return false, err
		}
	}
	if err := createRecoveryContactIssue(ctx, client, command.Issue); err != nil {
		return false, err
	}
	if err := createRecoveryCommandReceipt(ctx, client, command.Command); err != nil {
		return false, err
	}
	return true, nil
}

func (repository *RecoveryContactRepository) ResendPending(
	ctx context.Context,
	command ports.RecoveryContactResendCommand,
) (bool, error) {
	if repository == nil || repository.client == nil || ctx == nil ||
		command.Command.Kind != ports.RecoveryContactCommandResend ||
		!validRecoveryMutationCommand(command.Command) || command.ContactID == uuid.Nil ||
		command.ExpectedPendingVersion == 0 || command.ExpectedPendingVersion > math.MaxInt64 ||
		command.TokenID == uuid.Nil || !validKeyedTokenHash(command.Token) ||
		command.IssuedAt.IsZero() || !command.ExpiresAt.After(command.IssuedAt) ||
		command.HourlyWindowStart.IsZero() || command.DailyWindowStart.IsZero() ||
		!command.DailyWindowStart.Before(command.HourlyWindowStart) ||
		!command.HourlyWindowStart.Before(command.Command.Now) ||
		command.MaximumPerHour < 1 || command.MaximumPerHour > 100 ||
		command.MaximumPerDay < command.MaximumPerHour || command.MaximumPerDay > 1000 ||
		command.Cooldown <= 0 || command.Cooldown > time.Hour {
		return false, fmt.Errorf("%s: invalid resend command", recoveryContactRepositoryName)
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if err := requireSecurityEmailTransaction(ctx); err != nil {
		return false, err
	}
	client := repository.client.DB(ctx)
	replayed, err := lockRecoveryMutationAuthority(ctx, client, command.Command)
	if err != nil || replayed {
		return false, err
	}
	contacts, err := activeRecoveryContactsForUpdate(ctx, client, command.Command.UserID)
	if err != nil {
		return false, err
	}
	_, pending, err := selectActiveRecoveryContacts(contacts)
	if err != nil || pending == nil || pending.ID != command.ContactID ||
		pending.Version != int64(command.ExpectedPendingVersion) {
		return false, ports.ErrRecoveryContactVersionConflict
	}
	tokens, err := client.RecoveryContactVerificationToken.Query().Where(
		recoverycontactverificationtoken.ContactID(command.ContactID),
		recoverycontactverificationtoken.ContactVersion(int64(command.ExpectedPendingVersion)),
		recoverycontactverificationtoken.Or(
			recoverycontactverificationtoken.CreatedAtGTE(command.DailyWindowStart.UTC()),
			recoverycontactverificationtoken.StatusEQ(recoverycontactverificationtoken.StatusIssued),
		),
	).Order(
		recoverycontactverificationtoken.ByCreatedAt(sql.OrderDesc()),
		recoverycontactverificationtoken.ByID(sql.OrderDesc()),
	).Limit(command.MaximumPerDay + 9).Modify(func(selector *sql.Selector) {
		selector.ForUpdate()
	}).All(ctx)
	if err != nil {
		return false, commonent.MapEntError(err, recoveryContactRepositoryName)
	}
	if len(tokens) == 0 {
		return false, ports.ErrRecoveryContactVersionConflict
	}
	latest := tokens[0]
	if command.Command.Now.Before(latest.CreatedAt.UTC().Add(command.Cooldown)) {
		return false, ports.ErrRecoveryContactRateLimited
	}
	hourly, daily := 0, 0
	issuedIDs := make([]uuid.UUID, 0, len(tokens))
	for _, token := range tokens {
		createdAt := token.CreatedAt.UTC()
		if !createdAt.Before(command.DailyWindowStart.UTC()) {
			daily++
		}
		if !createdAt.Before(command.HourlyWindowStart.UTC()) {
			hourly++
		}
		if token.Status == recoverycontactverificationtoken.StatusIssued {
			issuedIDs = append(issuedIDs, token.ID)
		}
	}
	if hourly >= command.MaximumPerHour || daily >= command.MaximumPerDay {
		return false, ports.ErrRecoveryContactRateLimited
	}
	if err := supersedeRecoveryVerificationTokens(ctx, client, issuedIDs, command.Command.Now); err != nil {
		return false, err
	}
	if _, err := client.RecoveryContactVerificationToken.Create().
		SetID(command.TokenID).
		SetContactID(command.ContactID).
		SetContactVersion(int64(command.ExpectedPendingVersion)).
		SetHashKeyID(command.Token.KeyID).
		SetTokenHash(append([]byte(nil), command.Token.Hash...)).
		SetPurpose(recoverycontactverificationtoken.Purpose(pending.Purpose)).
		SetStatus(recoverycontactverificationtoken.StatusIssued).
		SetExpiresAt(command.ExpiresAt.UTC()).
		SetCreatedAt(command.IssuedAt.UTC()).
		SetUpdatedAt(command.IssuedAt.UTC()).
		Save(ctx); err != nil {
		return false, commonent.MapEntError(err, recoveryContactRepositoryName)
	}
	if err := createRecoveryCommandReceipt(ctx, client, command.Command); err != nil {
		return false, err
	}
	return true, nil
}

func (repository *RecoveryContactRepository) CancelPending(
	ctx context.Context,
	command ports.RecoveryContactVersionCommand,
) (bool, error) {
	return repository.transitionRecoveryContact(ctx, command, true)
}

func (repository *RecoveryContactRepository) RemoveCurrent(
	ctx context.Context,
	command ports.RecoveryContactVersionCommand,
) (bool, error) {
	return repository.transitionRecoveryContact(ctx, command, false)
}

func (repository *RecoveryContactRepository) transitionRecoveryContact(
	ctx context.Context,
	command ports.RecoveryContactVersionCommand,
	pendingTransition bool,
) (bool, error) {
	expectedKind := ports.RecoveryContactCommandRemove
	if pendingTransition {
		expectedKind = ports.RecoveryContactCommandCancelPending
	}
	if repository == nil || repository.client == nil || ctx == nil ||
		command.Command.Kind != expectedKind || !validRecoveryMutationCommand(command.Command) ||
		command.ExpectedVersion == 0 || command.ExpectedVersion > math.MaxInt64 {
		return false, fmt.Errorf("%s: invalid versioned command", recoveryContactRepositoryName)
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if err := requireSecurityEmailTransaction(ctx); err != nil {
		return false, err
	}
	client := repository.client.DB(ctx)
	replayed, err := lockRecoveryMutationAuthority(ctx, client, command.Command)
	if err != nil || replayed {
		return false, err
	}
	contacts, err := activeRecoveryContactsForUpdate(ctx, client, command.Command.UserID)
	if err != nil {
		return false, err
	}
	current, pending, err := selectActiveRecoveryContacts(contacts)
	if err != nil {
		return false, err
	}
	if pendingTransition {
		if pending == nil || pending.Version != int64(command.ExpectedVersion) {
			return false, ports.ErrRecoveryContactVersionConflict
		}
		if err := retirePendingRecoveryContact(ctx, client, pending, command.Command.Now); err != nil {
			return false, err
		}
	} else {
		if current == nil || current.Version != int64(command.ExpectedVersion) || pending != nil {
			return false, ports.ErrRecoveryContactVersionConflict
		}
		hasPassword, passwordErr := activePasswordCredentialExists(ctx, client, command.Command.UserID)
		if passwordErr != nil {
			return false, commonent.MapEntError(passwordErr, recoveryContactRepositoryName)
		}
		if hasPassword {
			return false, ports.ErrRecoveryContactRemovalRejected
		}
		updated, updateErr := client.UserRecoveryContact.Update().Where(
			userrecoverycontact.ID(current.ID),
			userrecoverycontact.StatusEQ(userrecoverycontact.StatusVerified),
			userrecoverycontact.Version(current.Version),
		).SetStatus(userrecoverycontact.StatusRevoked).
			SetRevokedAt(command.Command.Now.UTC()).
			SetAddressCiphertext([]byte{0}).
			Save(ctx)
		if updateErr != nil {
			return false, commonent.MapEntError(updateErr, recoveryContactRepositoryName)
		}
		if updated != 1 {
			return false, ports.ErrRecoveryContactVersionConflict
		}
	}
	if err := createRecoveryCommandReceipt(ctx, client, command.Command); err != nil {
		return false, err
	}
	return true, nil
}

func lockRecoveryMutationAuthority(
	ctx context.Context,
	client *generate.Client,
	command ports.RecoveryContactMutationCommand,
) (bool, error) {
	owner, err := client.User.Query().Where(
		user.ID(command.UserID),
		user.StatusEQ(user.StatusActive),
		user.DeletedAtIsNil(),
	).ForUpdate().Only(ctx)
	if err != nil {
		if generate.IsNotFound(err) {
			return false, ports.ErrRecoveryContactRejected
		}
		return false, commonent.MapEntError(err, recoveryContactRepositoryName)
	}
	session, err := client.AuthSession.Query().Where(
		authsession.ID(command.SessionID),
		authsession.UserID(command.UserID),
		authsession.StatusEQ(authsession.StatusActive),
		authsession.IdleExpiresAtGT(command.Now.UTC()),
		authsession.AbsoluteExpiresAtGT(command.Now.UTC()),
	).ForUpdate().Only(ctx)
	if err != nil {
		if generate.IsNotFound(err) {
			return false, ports.ErrRecoveryContactRejected
		}
		return false, commonent.MapEntError(err, recoveryContactRepositoryName)
	}
	if owner.SecurityVersion < 1 || session.UserSecurityVersion != owner.SecurityVersion {
		return false, ports.ErrRecoveryContactRejected
	}
	receipt, err := client.RecoveryContactCommandReceipt.Query().Where(
		recoverycontactcommandreceipt.UserID(command.UserID),
		recoverycontactcommandreceipt.IdempotencyKey(command.IdempotencyKey),
	).ForUpdate().Only(ctx)
	if err == nil {
		if receipt.SessionID != command.SessionID || string(receipt.Kind) != string(command.Kind) ||
			!hmac.Equal(receipt.RequestFingerprint, command.RequestFingerprint) {
			return false, ports.ErrRecoveryContactIdempotencyConflict
		}
		return true, nil
	}
	if !generate.IsNotFound(err) {
		return false, commonent.MapEntError(err, recoveryContactRepositoryName)
	}
	if session.LastReauthenticatedAt.UTC().Before(command.RecentAfter.UTC()) {
		return false, ports.ErrRecoveryContactRecentAuthenticationRequired
	}
	return false, nil
}

func activeRecoveryContactsForUpdate(
	ctx context.Context,
	client *generate.Client,
	userID uuid.UUID,
) ([]*generate.UserRecoveryContact, error) {
	contacts, err := client.UserRecoveryContact.Query().Where(
		userrecoverycontact.UserID(userID),
		userrecoverycontact.KindEQ(userrecoverycontact.KindEmail),
		userrecoverycontact.StatusIn(userrecoverycontact.StatusPending, userrecoverycontact.StatusVerified),
	).Order(userrecoverycontact.ByID()).Limit(3).Modify(func(selector *sql.Selector) {
		selector.ForUpdate()
	}).All(ctx)
	if err != nil {
		return nil, commonent.MapEntError(err, recoveryContactRepositoryName)
	}
	if len(contacts) > 2 {
		return nil, fmt.Errorf("%s: invalid active contact cardinality", recoveryContactRepositoryName)
	}
	return contacts, nil
}

func selectActiveRecoveryContacts(
	contacts []*generate.UserRecoveryContact,
) (*generate.UserRecoveryContact, *generate.UserRecoveryContact, error) {
	var current, pending *generate.UserRecoveryContact
	for _, contact := range contacts {
		if contact == nil {
			return nil, nil, fmt.Errorf("%s: nil active contact", recoveryContactRepositoryName)
		}
		switch contact.Status {
		case userrecoverycontact.StatusVerified:
			if current != nil {
				return nil, nil, fmt.Errorf("%s: duplicate verified contact", recoveryContactRepositoryName)
			}
			current = contact
		case userrecoverycontact.StatusPending:
			if pending != nil {
				return nil, nil, fmt.Errorf("%s: duplicate pending contact", recoveryContactRepositoryName)
			}
			pending = contact
		default:
			return nil, nil, fmt.Errorf("%s: invalid active contact status", recoveryContactRepositoryName)
		}
	}
	return current, pending, nil
}

func retirePendingRecoveryContact(
	ctx context.Context,
	client *generate.Client,
	pending *generate.UserRecoveryContact,
	now time.Time,
) error {
	if pending == nil {
		return fmt.Errorf("%s: nil pending contact", recoveryContactRepositoryName)
	}
	tokens, err := client.RecoveryContactVerificationToken.Query().Where(
		recoverycontactverificationtoken.ContactID(pending.ID),
	).Order(recoverycontactverificationtoken.ByID()).Limit(257).Modify(func(selector *sql.Selector) {
		selector.ForUpdate()
	}).All(ctx)
	if err != nil {
		return commonent.MapEntError(err, recoveryContactRepositoryName)
	}
	if len(tokens) > 256 {
		return fmt.Errorf("%s: verification history exceeds bound", recoveryContactRepositoryName)
	}
	issuedIDs := make([]uuid.UUID, 0, len(tokens))
	for _, token := range tokens {
		if token.Status == recoverycontactverificationtoken.StatusIssued {
			issuedIDs = append(issuedIDs, token.ID)
		}
	}
	if err := supersedeRecoveryVerificationTokens(ctx, client, issuedIDs, now); err != nil {
		return err
	}
	updated, err := client.UserRecoveryContact.Update().Where(
		userrecoverycontact.ID(pending.ID),
		userrecoverycontact.StatusEQ(userrecoverycontact.StatusPending),
		userrecoverycontact.Version(pending.Version),
	).SetStatus(userrecoverycontact.StatusRevoked).
		SetRevokedAt(now.UTC()).
		SetAddressCiphertext([]byte{0}).
		Save(ctx)
	if err != nil {
		return commonent.MapEntError(err, recoveryContactRepositoryName)
	}
	if updated != 1 {
		return ports.ErrRecoveryContactVersionConflict
	}
	return nil
}

func supersedeRecoveryVerificationTokens(
	ctx context.Context,
	client *generate.Client,
	tokenIDs []uuid.UUID,
	now time.Time,
) error {
	if len(tokenIDs) == 0 {
		return nil
	}
	deliveries, err := client.SecurityEmailDelivery.Query().Where(
		securityemaildelivery.VerificationTokenIDIn(tokenIDs...),
		securityemaildelivery.StatusIn(securityemaildelivery.StatusReady, securityemaildelivery.StatusLeased),
	).Order(securityemaildelivery.ByID()).Limit(len(tokenIDs) + 1).Modify(func(selector *sql.Selector) {
		selector.ForUpdate()
	}).All(ctx)
	if err != nil {
		return commonent.MapEntError(err, recoveryContactRepositoryName)
	}
	if len(deliveries) > len(tokenIDs) {
		return fmt.Errorf("%s: invalid delivery cardinality", recoveryContactRepositoryName)
	}
	deliveryIDs := make([]uuid.UUID, 0, len(deliveries))
	for _, delivery := range deliveries {
		deliveryIDs = append(deliveryIDs, delivery.ID)
	}
	updated, err := client.RecoveryContactVerificationToken.Update().Where(
		recoverycontactverificationtoken.IDIn(tokenIDs...),
		recoverycontactverificationtoken.StatusIn(
			recoverycontactverificationtoken.StatusIssued,
			recoverycontactverificationtoken.StatusExpired,
		),
	).SetStatus(recoverycontactverificationtoken.StatusSuperseded).
		SetSupersededAt(now.UTC()).
		Save(ctx)
	if err != nil {
		return commonent.MapEntError(err, recoveryContactRepositoryName)
	}
	if updated != len(tokenIDs) {
		return fmt.Errorf("%s: verification token supersession fence failed", recoveryContactRepositoryName)
	}
	if len(deliveryIDs) > 0 {
		if _, err := client.SecurityEmailDelivery.Update().Where(
			securityemaildelivery.IDIn(deliveryIDs...),
			securityemaildelivery.StatusIn(securityemaildelivery.StatusReady, securityemaildelivery.StatusLeased),
		).SetPayloadCiphertext([]byte{0}).
			SetStatus(securityemaildelivery.StatusCancelled).
			SetCancelledAt(now.UTC()).
			SetCiphertextErasedAt(now.UTC()).
			ClearLeaseOwner().ClearLeaseToken().ClearLeasedUntil().
			Save(ctx); err != nil {
			return commonent.MapEntError(err, recoveryContactRepositoryName)
		}
	}
	return nil
}

func createRecoveryContactIssue(
	ctx context.Context,
	client *generate.Client,
	issue ports.RecoveryContactIssue,
) error {
	contact := issue.Contact
	if _, err := client.UserRecoveryContact.Create().
		SetID(contact.ID).
		SetUserID(contact.UserID).
		SetKind(userrecoverycontact.KindEmail).
		SetPurpose(userrecoverycontact.Purpose(contact.Purpose)).
		SetLookupHash(append([]byte(nil), issue.LookupHash...)).
		SetFingerprintHash(append([]byte(nil), issue.FingerprintHash...)).
		SetAddressCiphertext(append([]byte(nil), contact.AddressCiphertext...)).
		SetKeyID(contact.KeyID).
		SetStatus(userrecoverycontact.StatusPending).
		SetVersion(int64(contact.Version)).
		SetCreatedAt(issue.IssuedAt.UTC()).
		SetUpdatedAt(issue.IssuedAt.UTC()).
		Save(ctx); err != nil {
		return commonent.MapEntError(err, recoveryContactRepositoryName)
	}
	if _, err := client.RecoveryContactVerificationToken.Create().
		SetID(issue.TokenID).
		SetContactID(contact.ID).
		SetContactVersion(int64(contact.Version)).
		SetHashKeyID(issue.Token.KeyID).
		SetTokenHash(append([]byte(nil), issue.Token.Hash...)).
		SetPurpose(recoverycontactverificationtoken.Purpose(contact.Purpose)).
		SetStatus(recoverycontactverificationtoken.StatusIssued).
		SetExpiresAt(issue.ExpiresAt.UTC()).
		SetCreatedAt(issue.IssuedAt.UTC()).
		SetUpdatedAt(issue.IssuedAt.UTC()).
		Save(ctx); err != nil {
		return commonent.MapEntError(err, recoveryContactRepositoryName)
	}
	return nil
}

func createRecoveryCommandReceipt(
	ctx context.Context,
	client *generate.Client,
	command ports.RecoveryContactMutationCommand,
) error {
	if _, err := client.RecoveryContactCommandReceipt.Create().
		SetID(command.ReceiptID).
		SetUserID(command.UserID).
		SetSessionID(command.SessionID).
		SetIdempotencyKey(command.IdempotencyKey).
		SetKind(recoverycontactcommandreceipt.Kind(command.Kind)).
		SetRequestFingerprint(append([]byte(nil), command.RequestFingerprint...)).
		SetExpiresAt(command.ReceiptExpiresAt.UTC()).
		SetCreatedAt(command.Now.UTC()).
		SetUpdatedAt(command.Now.UTC()).
		Save(ctx); err != nil {
		return commonent.MapEntError(err, recoveryContactRepositoryName)
	}
	return nil
}

func validRecoveryMutationCommand(command ports.RecoveryContactMutationCommand) bool {
	if command.ReceiptID == uuid.Nil || command.UserID == uuid.Nil || command.SessionID == uuid.Nil ||
		command.IdempotencyKey == uuid.Nil || len(command.RequestFingerprint) != sha256.Size ||
		command.Now.IsZero() || command.RecentAfter.IsZero() || command.ReceiptExpiresAt.IsZero() ||
		command.RecentAfter.After(command.Now) || !command.ReceiptExpiresAt.After(command.Now) {
		return false
	}
	switch command.Kind {
	case ports.RecoveryContactCommandReplace, ports.RecoveryContactCommandResend,
		ports.RecoveryContactCommandCancelPending, ports.RecoveryContactCommandRemove:
		return true
	default:
		return false
	}
}

func validReplacementRecoveryIssue(
	issue ports.RecoveryContactIssue,
	userID uuid.UUID,
	version uint64,
) bool {
	contact := issue.Contact
	return contact.ID != uuid.Nil && contact.UserID == userID && contact.Kind == recoveryContactEmailKind &&
		contact.Purpose == entity.RecoveryContactPurposeReplacement &&
		contact.Status == entity.RecoveryContactStatusPending && contact.Version == version &&
		version <= math.MaxInt64 && len(contact.AddressCiphertext) > 1 &&
		len(contact.AddressCiphertext) <= maximumStoredContactCiphertextBytes &&
		validSecurityEmailText(contact.KeyID, 128) && len(issue.LookupHash) == sha256.Size &&
		len(issue.FingerprintHash) == sha256.Size && issue.TokenID != uuid.Nil &&
		validKeyedTokenHash(issue.Token) && !issue.IssuedAt.IsZero() &&
		issue.ExpiresAt.After(issue.IssuedAt) && contact.CreatedAt.Equal(issue.IssuedAt) &&
		contact.UpdatedAt.Equal(issue.IssuedAt)
}

func validRecoveryContactIssue(issue ports.RecoveryContactIssue) bool {
	contact := issue.Contact
	return contact.ID != uuid.Nil && contact.UserID != uuid.Nil &&
		contact.Kind == recoveryContactEmailKind &&
		(contact.Purpose == entity.RecoveryContactPurposeRegistration ||
			contact.Purpose == entity.RecoveryContactPurposeInvitation) &&
		contact.Status == entity.RecoveryContactStatusPending && contact.Version == 1 &&
		len(contact.AddressCiphertext) > 1 && len(contact.AddressCiphertext) <= maximumStoredContactCiphertextBytes &&
		validSecurityEmailText(contact.KeyID, 128) && len(issue.LookupHash) == 32 &&
		len(issue.FingerprintHash) == 32 && issue.TokenID != uuid.Nil &&
		validKeyedTokenHash(issue.Token) && !issue.IssuedAt.IsZero() &&
		issue.ExpiresAt.After(issue.IssuedAt) &&
		contact.CreatedAt.Equal(issue.IssuedAt) && contact.UpdatedAt.Equal(issue.IssuedAt)
}

func recoveryVerificationCandidatesPredicate(
	candidates []entity.KeyedTokenHash,
) predicate.RecoveryContactVerificationToken {
	predicates := make([]predicate.RecoveryContactVerificationToken, 0, len(candidates))
	for _, candidate := range candidates {
		predicates = append(predicates, recoverycontactverificationtoken.And(
			recoverycontactverificationtoken.HashKeyID(candidate.KeyID),
			recoverycontactverificationtoken.TokenHashEQ(append([]byte(nil), candidate.Hash...)),
		))
	}
	return recoverycontactverificationtoken.Or(predicates...)
}

var _ ports.RecoveryContactRepository = (*RecoveryContactRepository)(nil)
var _ ports.AccountInvitationRepository = (*RecoveryContactRepository)(nil)
var _ ports.RecoveryContactManagementRepository = (*RecoveryContactRepository)(nil)
var _ ports.RecoveryContactMutationRepository = (*RecoveryContactRepository)(nil)
var _ ports.RecoveryContactStore = (*RecoveryContactRepository)(nil)
