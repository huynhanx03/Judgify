package http

import (
	"time"

	"github.com/huynhanx03/judgify/internal/platform/operation"
)

type startRankingReconciliationRequest struct {
	Reason           string    `json:"reason" validate:"required,min=3,max=1024"`
	SourceCutoff     time.Time `json:"source_cutoff" validate:"required"`
	ExpectedRevision int64     `json:"expected_revision" validate:"min=0"`
	SourceCount      int64     `json:"source_count" validate:"min=0"`
	RepairCount      int64     `json:"repair_count" validate:"min=0"`
	PreviewChecksum  string    `json:"preview_checksum" validate:"required,len=64"`
}

type rankingReconciliationPreviewResponse struct {
	SourceCutoff       time.Time `json:"source_cutoff"`
	ProjectionRevision int64     `json:"projection_revision"`
	SourceCount        int64     `json:"source_count"`
	RepairCount        int64     `json:"repair_count"`
	PreviewChecksum    string    `json:"preview_checksum"`
}

type rankingReconciliationStartResponse struct {
	Operation          operation.Projection `json:"operation"`
	SourceCutoff       time.Time            `json:"source_cutoff"`
	SourceCount        int64                `json:"source_count"`
	RepairCount        int64                `json:"repair_count"`
	ProjectionRevision int64                `json:"projection_revision"`
	IdempotentReplay   bool                 `json:"idempotent_replay"`
}

type rankingReconciliationStatusResponse struct {
	ProjectionRevision int64                 `json:"projection_revision"`
	LastReconciledAt   *time.Time            `json:"last_reconciled_at,omitempty"`
	LatestOperation    *operation.Projection `json:"latest_operation,omitempty"`
}
