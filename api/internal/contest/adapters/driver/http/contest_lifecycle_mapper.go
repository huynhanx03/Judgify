package http

import contract "github.com/huynhanx03/judgify/internal/contest/contract"

func toContestLifecycleResponse(value contract.ContestLifecycleResponse) contestLifecycleResponse {
	return contestLifecycleResponse{ContestID: value.ContestID.String(), From: value.From, To: value.To, Version: value.Version, OccurredAt: value.OccurredAt, CommandID: value.CommandID.String(), IdempotentReplay: value.IdempotentReplay, CID: value.CID}
}
