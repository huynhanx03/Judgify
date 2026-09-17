package db

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	commonent "github.com/huynhanx03/go-common/pkg/database/ent"

	dbent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/ent/generate/authsessionissuancereceipt"
	entity "github.com/huynhanx03/judgify/internal/identity/domain"
	"github.com/huynhanx03/judgify/internal/ports"
)

const issuanceReceiptRepoName = "session issuance receipt"

type SessionIssuanceReceiptRepository struct {
	client *dbent.EntClient
}

func NewSessionIssuanceReceiptRepository(client *dbent.EntClient) ports.SessionIssuanceReceiptRepository {
	return &SessionIssuanceReceiptRepository{client: client}
}

func (repository *SessionIssuanceReceiptRepository) Get(
	ctx context.Context,
	selector ports.IssuanceReceiptSelector,
) (*entity.SessionIssuanceReceipt, error) {
	if repository == nil || repository.client == nil || ctx == nil ||
		selector.IdempotencyKey == uuid.Nil {
		return nil, fmt.Errorf("%s: invalid selector", issuanceReceiptRepoName)
	}
	query := repository.client.DB(ctx).AuthSessionIssuanceReceipt.Query().Where(
		authsessionissuancereceipt.PurposeEQ(
			authsessionissuancereceipt.Purpose(selector.Purpose),
		),
		authsessionissuancereceipt.IdempotencyKey(selector.IdempotencyKey),
	)
	if selector.Purpose == entity.SessionIssuancePasswordLogin {
		if selector.UserID == uuid.Nil || selector.PreSessionID == uuid.Nil {
			return nil, fmt.Errorf("%s: incomplete password-login selector", issuanceReceiptRepoName)
		}
		query.Where(
			authsessionissuancereceipt.UserID(selector.UserID),
			authsessionissuancereceipt.PreSessionID(selector.PreSessionID),
		)
	} else {
		if selector.RequestID == uuid.Nil {
			return nil, fmt.Errorf("%s: incomplete external selector", issuanceReceiptRepoName)
		}
		query.Where(authsessionissuancereceipt.RequestOrTicketID(selector.RequestID))
	}
	record, err := query.Only(ctx)
	if err != nil {
		return nil, commonent.MapEntError(err, issuanceReceiptRepoName)
	}
	return issuanceReceiptEntity(record), nil
}

func (repository *SessionIssuanceReceiptRepository) Create(
	ctx context.Context,
	receipt *entity.SessionIssuanceReceipt,
) error {
	registration := receipt != nil && (receipt.Purpose == entity.SessionIssuancePasswordRegistration ||
		receipt.Purpose == entity.SessionIssuanceOAuthRegistration)
	hasOffer := receipt != nil && receipt.TraitOfferID != nil &&
		*receipt.TraitOfferID != uuid.Nil && len(receipt.OfferChecksum) == 32
	if repository == nil || repository.client == nil || ctx == nil || receipt == nil ||
		receipt.ID == uuid.Nil || receipt.IdempotencyKey == uuid.Nil ||
		receipt.SessionID == uuid.Nil || receipt.RefreshTokenID == uuid.Nil ||
		len(receipt.RecoveryCiphertext) == 0 || receipt.KeyID == "" || receipt.ExpiresAt.IsZero() ||
		registration != hasOffer {
		return fmt.Errorf("%s: invalid create input", issuanceReceiptRepoName)
	}
	create := repository.client.DB(ctx).AuthSessionIssuanceReceipt.Create().
		SetID(receipt.ID).
		SetPurpose(authsessionissuancereceipt.Purpose(receipt.Purpose)).
		SetNillableRequestOrTicketID(receipt.RequestOrTicketID).
		SetNillableUserID(receipt.UserID).
		SetNillablePreSessionID(receipt.PreSessionID).
		SetNillableFingerprintKeyID(receipt.FingerprintKeyID).
		SetIdempotencyKey(receipt.IdempotencyKey).
		SetSessionID(receipt.SessionID).
		SetRefreshTokenID(receipt.RefreshTokenID).
		SetNillableTraitOfferID(receipt.TraitOfferID).
		SetRecoveryCiphertext(append([]byte(nil), receipt.RecoveryCiphertext...)).
		SetKeyID(receipt.KeyID).
		SetExpiresAt(receipt.ExpiresAt.UTC()).
		SetNillableRecoveredAt(utcTimePointer(receipt.RecoveredAt)).
		SetNillableErasedAt(utcTimePointer(receipt.ErasedAt))
	if len(receipt.OfferChecksum) > 0 {
		create.SetOfferChecksum(append([]byte(nil), receipt.OfferChecksum...))
	}
	if len(receipt.RequestFingerprint) > 0 {
		create.SetRequestFingerprint(append([]byte(nil), receipt.RequestFingerprint...))
	}
	if !receipt.CreatedAt.IsZero() {
		create.SetCreatedAt(receipt.CreatedAt.UTC())
	}
	if !receipt.UpdatedAt.IsZero() {
		create.SetUpdatedAt(receipt.UpdatedAt.UTC())
	}
	record, err := create.Save(ctx)
	if err != nil {
		return commonent.MapEntError(err, issuanceReceiptRepoName)
	}
	mapIssuanceReceipt(record, receipt)
	return nil
}

func (repository *SessionIssuanceReceiptRepository) MarkRecovered(
	ctx context.Context,
	id uuid.UUID,
	recoveredAt time.Time,
) error {
	if repository == nil || repository.client == nil || ctx == nil || id == uuid.Nil || recoveredAt.IsZero() {
		return fmt.Errorf("%s: invalid recovery marker", issuanceReceiptRepoName)
	}
	_, err := repository.client.DB(ctx).AuthSessionIssuanceReceipt.Update().
		Where(
			authsessionissuancereceipt.ID(id),
			authsessionissuancereceipt.ErasedAtIsNil(),
			authsessionissuancereceipt.ExpiresAtGT(recoveredAt.UTC()),
		).
		SetRecoveredAt(recoveredAt.UTC()).
		Save(ctx)
	if err != nil {
		return commonent.MapEntError(err, issuanceReceiptRepoName)
	}
	return nil
}

func issuanceReceiptEntity(record *generate.AuthSessionIssuanceReceipt) *entity.SessionIssuanceReceipt {
	if record == nil {
		return nil
	}
	result := &entity.SessionIssuanceReceipt{}
	mapIssuanceReceipt(record, result)
	return result
}

func mapIssuanceReceipt(record *generate.AuthSessionIssuanceReceipt, result *entity.SessionIssuanceReceipt) {
	if record == nil || result == nil {
		return
	}
	result.ID = record.ID
	result.Purpose = entity.SessionIssuancePurpose(record.Purpose)
	result.RequestOrTicketID = copyUUIDPointer(record.RequestOrTicketID)
	result.UserID = copyUUIDPointer(record.UserID)
	result.PreSessionID = copyUUIDPointer(record.PreSessionID)
	if record.RequestFingerprint != nil {
		result.RequestFingerprint = append([]byte(nil), (*record.RequestFingerprint)...)
	}
	result.FingerprintKeyID = copyStringPointer(record.FingerprintKeyID)
	result.IdempotencyKey = record.IdempotencyKey
	result.SessionID = record.SessionID
	result.RefreshTokenID = record.RefreshTokenID
	result.TraitOfferID = copyUUIDPointer(record.TraitOfferID)
	if record.OfferChecksum != nil {
		result.OfferChecksum = append([]byte(nil), (*record.OfferChecksum)...)
	}
	result.RecoveryCiphertext = append([]byte(nil), record.RecoveryCiphertext...)
	result.KeyID = record.KeyID
	result.ExpiresAt = record.ExpiresAt.UTC()
	result.RecoveredAt = utcTimePointer(record.RecoveredAt)
	result.ErasedAt = utcTimePointer(record.ErasedAt)
	result.CreatedAt = record.CreatedAt.UTC()
	result.UpdatedAt = record.UpdatedAt.UTC()
}

var _ ports.SessionIssuanceReceiptRepository = (*SessionIssuanceReceiptRepository)(nil)
