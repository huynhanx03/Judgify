package http

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/huynhanx03/judgify/internal/ports"
)

func TestToPublishMaterialLifecycleRequestParsesHTTPIDs(t *testing.T) {
	materialID := uuid.New()
	commandID := uuid.New()
	revisionID := uuid.New()

	request, err := toPublishMaterialLifecycleRequest(&publishMaterialRequest{
		ID:              materialID.String(),
		CommandID:       commandID.String(),
		DraftRevisionID: revisionID.String(),
		ExpectedVersion: 4,
		Reason:          "publish approved revision",
	})

	require.NoError(t, err)
	require.Equal(t, ports.MaterialLifecyclePublish, request.Action)
	require.Equal(t, materialID, request.MaterialID)
	require.Equal(t, commandID, request.CommandID)
	require.Equal(t, revisionID, request.TargetRevisionID)
	require.Equal(t, int64(4), request.ExpectedVersion)
}

func TestToArchiveMaterialLifecycleRequestRejectsInvalidHTTPID(t *testing.T) {
	_, err := toArchiveMaterialLifecycleRequest(
		ports.MaterialLifecycleArchive,
		&archiveMaterialRequest{ID: "not-a-uuid"},
	)

	require.Error(t, err)
}
