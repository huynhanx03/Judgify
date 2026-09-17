package usecase

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/correlation"

	"github.com/huynhanx03/judgify/internal/authctx"
	entity "github.com/huynhanx03/judgify/internal/material/domain"
	"github.com/huynhanx03/judgify/internal/material/viewer"
	"github.com/huynhanx03/judgify/internal/platform/eventing"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	"github.com/huynhanx03/judgify/internal/ports"
)

const materialViewHMACDomain = "judgify/material/view-dedupe/v1"

type viewService struct {
	repository   ports.MaterialViewRepository
	transactions platformtx.UnitOfWork
	outbox       eventing.OutboxAppender
	key          [sha256.Size]byte
	now          func() time.Time
}

type ViewDependencies struct {
	Repository   ports.MaterialViewRepository
	Transactions platformtx.UnitOfWork
	Outbox       eventing.OutboxAppender
	Secret       []byte
}

func NewViewService(dependencies ViewDependencies) (ports.MaterialViewService, error) {
	if dependencies.Repository == nil || dependencies.Transactions == nil ||
		dependencies.Outbox == nil || len(dependencies.Secret) < 32 {
		return nil, entity.ErrMaterialInvalidCommand
	}
	key := sha256.Sum256(append(
		[]byte(materialViewHMACDomain+"\x00"),
		dependencies.Secret...,
	))
	return &viewService{
		repository:   dependencies.Repository,
		transactions: dependencies.Transactions,
		outbox:       dependencies.Outbox,
		key:          key,
		now:          func() time.Time { return time.Now().UTC() },
	}, nil
}

func (service *viewService) Record(ctx context.Context, materialID uuid.UUID) (bool, error) {
	if materialID == uuid.Nil {
		return false, entity.ErrMaterialInvalidCommand
	}
	viewerKind := byte(1)
	viewerFingerprint, anonymous := viewer.AnonymousFingerprint(ctx)
	viewerValue := viewerFingerprint[:]
	authenticated := false
	if sessionID, ok := authctx.SessionID(ctx); ok && sessionID != uuid.Nil {
		viewerKind = 2
		viewerValue = sessionID[:]
		authenticated = true
	}
	if !authenticated && !anonymous {
		return false, entity.ErrMaterialInvalidCommand
	}
	day := service.now().UTC().Truncate(24 * time.Hour)
	mac := hmac.New(sha256.New, service.key[:])
	_, _ = mac.Write([]byte(materialViewHMACDomain))
	_, _ = mac.Write(materialID[:])
	_, _ = mac.Write([]byte{viewerKind})
	var dayBytes [8]byte
	binary.BigEndian.PutUint64(dayBytes[:], uint64(day.Unix()))
	_, _ = mac.Write(dayBytes[:])
	_, _ = mac.Write(viewerValue)
	var fingerprint [sha256.Size]byte
	copy(fingerprint[:], mac.Sum(nil))
	var counted bool
	ctx = correlation.EnsureContext(ctx)
	err := service.transactions.Within(ctx, func(txCtx context.Context) error {
		receipt, err := service.repository.RecordDailyView(
			txCtx, materialID, day, fingerprint, authenticated,
		)
		if err != nil {
			return err
		}
		if !receipt.Inserted {
			return nil
		}
		payload, err := json.Marshal(entity.DailyViewRecorded{
			ReceiptID:  receipt.ID,
			MaterialID: receipt.MaterialID,
			ViewDay:    receipt.ViewDay.UTC(),
		})
		if err != nil {
			return err
		}
		eventID := uuid.NewSHA1(
			uuid.NameSpaceURL,
			[]byte(entity.MaterialViewRecordedV1+"\x00"+receipt.ID.String()),
		)
		if err := service.outbox.Append(txCtx, eventing.Event{
			ID: eventID, Type: entity.MaterialViewRecordedV1,
			AggregateType: "material_view", AggregateID: receipt.ID,
			AggregateVersion: 1, Payload: payload, OccurredAt: service.now().UTC(),
		}); err != nil {
			return err
		}
		counted = true
		return nil
	})
	return counted, err
}

var _ ports.MaterialViewService = (*viewService)(nil)
