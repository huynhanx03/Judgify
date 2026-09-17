package db

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	entity "github.com/huynhanx03/judgify/internal/contest/domain"
	"github.com/huynhanx03/judgify/internal/ports"
)

func TestContentCommandReceiptRejectsIncompleteResult(t *testing.T) {
	receipt := ports.ContentCommandReceipt{ContestID: uuid.New(), Kind: ports.ContentCommandClarificationAnswer, IdempotencyKey: uuid.New(), ActorUserID: uuid.New(), ActorSessionID: uuid.New()}
	receipt.RequestHash[0] = 1
	require.True(t, validContentReceipt(receipt))
	require.False(t, validCompletedContentReceipt(receipt))
	receipt.ResultOwner = entity.ContentOwnerClarification
	receipt.ResultContentID = uuid.New()
	receipt.ResultVersion = 2
	receipt.ResultProjection = []byte(`{}`)
	require.True(t, validCompletedContentReceipt(receipt))
}

func TestContentCommandKindsFailClosed(t *testing.T) {
	receipt := ports.ContentCommandReceipt{ContestID: uuid.New(), Kind: ports.ContentCommandKind("unknown"), IdempotencyKey: uuid.New(), ActorUserID: uuid.New(), ActorSessionID: uuid.New()}
	receipt.RequestHash[0] = 1
	require.False(t, validContentReceipt(receipt))
}
