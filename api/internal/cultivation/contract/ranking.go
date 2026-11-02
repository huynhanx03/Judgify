package contract

import (
	"time"

	"github.com/google/uuid"

	"github.com/huynhanx03/judgify/internal/platform/operation"
)

type RankingEntry struct {
	Rank        int64
	PublicID    uuid.UUID
	DisplayName *string
	DisplayMode string
	Rating      int
	RankTitle   string
	TotalExp    int64
	LevelName   string
	Level       int
}

type RankingPageResponse struct {
	Entries            []*RankingEntry
	NextCursor         string
	ProjectionRevision int64
}

type RankingPrivacyResponse struct {
	Visibility         string
	Version            int64
	ProjectionRevision int64
	Replayed           bool
}

type RankingReconciliationPreviewResponse struct {
	SourceCutoff       time.Time
	ProjectionRevision int64
	SourceCount        int64
	RepairCount        int64
	PreviewChecksum    string
}

type RankingReconciliationStartResponse struct {
	Operation          operation.Projection
	SourceCutoff       time.Time
	SourceCount        int64
	RepairCount        int64
	ProjectionRevision int64
	IdempotentReplay   bool
}

type RankingReconciliationStatusResponse struct {
	ProjectionRevision int64
	LastReconciledAt   *time.Time
	LatestOperation    *operation.Projection
}
