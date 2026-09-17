package contract

import (
	"time"

	"github.com/google/uuid"
)

const (
	EventRankingReconciliationStartedV1 = "cultivation.ranking_reconciliation.started.v1"
	AggregateRankingReconciliation      = "cultivation_ranking_reconciliation"
)

var rankingReconciliationStartedEventNamespace = uuid.MustParse(
	"019f8fbf-f05d-75ca-9adb-21b06045c8e5",
)

type RankingReconciliationStartedV1 struct {
	OperationID        uuid.UUID `json:"operation_id"`
	SourceCutoff       time.Time `json:"source_cutoff"`
	SourceCount        int64     `json:"source_count"`
	RepairCount        int64     `json:"repair_count"`
	ProjectionRevision int64     `json:"projection_revision"`
}

func RankingReconciliationStartedEventID(operationID uuid.UUID) uuid.UUID {
	if operationID == uuid.Nil {
		return uuid.Nil
	}
	return uuid.NewSHA1(rankingReconciliationStartedEventNamespace, operationID[:])
}
