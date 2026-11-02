package http

import contract "github.com/huynhanx03/judgify/internal/cultivation/contract"

func toRankingReconciliationPreviewResponse(response *contract.RankingReconciliationPreviewResponse) *rankingReconciliationPreviewResponse {
	if response == nil {
		return nil
	}
	return &rankingReconciliationPreviewResponse{
		SourceCutoff: response.SourceCutoff, ProjectionRevision: response.ProjectionRevision,
		SourceCount: response.SourceCount, RepairCount: response.RepairCount,
		PreviewChecksum: response.PreviewChecksum,
	}
}

func toRankingReconciliationStartResponse(response *contract.RankingReconciliationStartResponse) *rankingReconciliationStartResponse {
	if response == nil {
		return nil
	}
	return &rankingReconciliationStartResponse{
		Operation: response.Operation, SourceCutoff: response.SourceCutoff,
		SourceCount: response.SourceCount, RepairCount: response.RepairCount,
		ProjectionRevision: response.ProjectionRevision, IdempotentReplay: response.IdempotentReplay,
	}
}

func toRankingReconciliationStatusResponse(response *contract.RankingReconciliationStatusResponse) *rankingReconciliationStatusResponse {
	if response == nil {
		return nil
	}
	return &rankingReconciliationStatusResponse{
		ProjectionRevision: response.ProjectionRevision, LastReconciledAt: response.LastReconciledAt,
		LatestOperation: response.LatestOperation,
	}
}
