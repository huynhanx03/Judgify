package http

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/common/apperr"

	"github.com/huynhanx03/judgify/internal/authctx"
	entity "github.com/huynhanx03/judgify/internal/cultivation/domain"
	"github.com/huynhanx03/judgify/internal/ports"
)

type cultivationCommandIDContextKey struct{}

type ProgressionHandler interface {
	PreviewAdjustment(context.Context, *previewProgressionAdjustmentRequest) (*progressionAdjustmentPreviewResponse, error)
	ApplyAdjustment(context.Context, *applyProgressionAdjustmentRequest) (*progressionAdjustmentResponse, error)
	ListLedger(context.Context, *progressionLedgerRequest) (*progressionLedgerPageResponse, error)
	ListMyLedger(context.Context, *myProgressionLedgerRequest) (*progressionLedgerPageResponse, error)
}

type progressionHandler struct {
	service ports.ProgressionService
}

func NewProgressionHandler(service ports.ProgressionService) ProgressionHandler {
	return &progressionHandler{service: service}
}

func (handler *progressionHandler) PreviewAdjustment(
	ctx context.Context,
	request *previewProgressionAdjustmentRequest,
) (*progressionAdjustmentPreviewResponse, error) {
	userID, err := parseProgressionUUID(request.UserID)
	if err != nil {
		return nil, err
	}
	reversedEntryID, err := parseOptionalProgressionUUID(request.ReversedEntryID)
	if err != nil {
		return nil, err
	}
	preview, err := handler.service.PreviewAdjustment(
		ctx, userID, request.Delta, reversedEntryID,
	)
	if err != nil {
		return nil, err
	}
	return &progressionAdjustmentPreviewResponse{
		UserID: preview.UserID.String(), Delta: preview.Delta,
		BalanceBefore: preview.BalanceBefore, BalanceAfter: preview.BalanceAfter,
		ExpectedVersion: preview.ExpectedVersion,
		ReversedEntryID: stringProgressionUUID(preview.ReversedEntryID),
		PreviewChecksum: hex.EncodeToString(preview.Checksum[:]),
	}, nil
}

func (handler *progressionHandler) ApplyAdjustment(
	ctx context.Context,
	request *applyProgressionAdjustmentRequest,
) (*progressionAdjustmentResponse, error) {
	userID, err := parseProgressionUUID(request.UserID)
	if err != nil {
		return nil, err
	}
	reversedEntryID, err := parseOptionalProgressionUUID(request.ReversedEntryID)
	if err != nil {
		return nil, err
	}
	commandID, ok := ctx.Value(cultivationCommandIDContextKey{}).(uuid.UUID)
	checksum, checksumOK := decodeCanonicalChecksum(request.PreviewChecksum)
	if !ok || commandID == uuid.Nil || !checksumOK {
		return nil, invalidCultivationCommand()
	}
	result, err := handler.service.ApplyAdjustment(ctx, entity.AdjustmentCommand{
		ID: commandID, UserID: userID, Delta: request.Delta,
		ExpectedBalance: request.ExpectedBalance,
		ExpectedVersion: request.ExpectedVersion,
		IncidentID:      request.IncidentID, Reason: request.Reason,
		PreviewChecksum: checksum, ReversedEntryID: reversedEntryID,
	})
	if err != nil {
		return nil, err
	}
	return &progressionAdjustmentResponse{
		LedgerEntryID: result.LedgerEntryID.String(), UserID: result.UserID.String(),
		Delta: result.Delta, BalanceBefore: result.BalanceBefore,
		BalanceAfter: result.BalanceAfter, Version: result.Version,
		Replayed: result.Replayed, CreatedAt: result.CreatedAt,
	}, nil
}

func (handler *progressionHandler) ListLedger(
	ctx context.Context,
	request *progressionLedgerRequest,
) (*progressionLedgerPageResponse, error) {
	userID, err := parseProgressionUUID(request.UserID)
	if err != nil {
		return nil, err
	}
	cursor, err := parseOptionalProgressionUUID(request.Cursor)
	if err != nil {
		return nil, err
	}
	page, err := handler.service.ListLedger(ctx, userID, cursor, request.Limit)
	if err != nil {
		return nil, err
	}
	return toProgressionLedgerPageResponse(page), nil
}

func (handler *progressionHandler) ListMyLedger(
	ctx context.Context,
	request *myProgressionLedgerRequest,
) (*progressionLedgerPageResponse, error) {
	userID, ok := authctx.UserID(ctx)
	if !ok || userID == uuid.Nil {
		return nil, apperr.New(apperr.CodeUnauthorized, "unauthorized", nil)
	}
	cursor, err := parseOptionalProgressionUUID(request.Cursor)
	if err != nil {
		return nil, err
	}
	page, err := handler.service.ListLedger(ctx, userID, cursor, request.Limit)
	if err != nil {
		return nil, err
	}
	return toProgressionLedgerPageResponse(page), nil
}

func decodeCanonicalChecksum(value string) ([sha256.Size]byte, bool) {
	decoded, err := hex.DecodeString(value)
	if err != nil || len(decoded) != sha256.Size || hex.EncodeToString(decoded) != value {
		return [sha256.Size]byte{}, false
	}
	var checksum [sha256.Size]byte
	copy(checksum[:], decoded)
	return checksum, checksum != ([sha256.Size]byte{})
}

func invalidCultivationCommand() error {
	return apperr.New(apperr.CodeBadRequest, "valid canonical Idempotency-Key and preview checksum are required", nil)
}
