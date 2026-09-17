package http

import contract "github.com/huynhanx03/judgify/internal/contest/contract"

func toStandingResponse(response *contract.StandingResponse) *standingResponse {
	if response == nil {
		return nil
	}
	return &standingResponse{
		Rank:             response.Rank,
		UserID:           response.UserID.String(),
		Username:         response.Username,
		SolvedCount:      response.SolvedCount,
		Penalty:          response.Penalty,
		ProblemResults:   response.ProblemResults,
		PendingAttempts:  response.PendingAttempts,
		PendingByProblem: response.PendingByProblem,
	}
}

func toStandingResponses(responses []*contract.StandingResponse) []*standingResponse {
	items := make([]*standingResponse, len(responses))
	for i, response := range responses {
		items[i] = toStandingResponse(response)
	}
	return items
}

func toStandingsResponse(response *contract.StandingsResponse) *standingsResponse {
	if response == nil {
		return nil
	}
	return &standingsResponse{
		ContestID:         response.ContestID.String(),
		View:              response.View,
		ProjectionVersion: response.ProjectionVersion,
		LastCursor:        response.LastCursor,
		FrozenAt:          response.FrozenAt,
		Items:             toStandingResponses(response.Items),
	}
}

func toSelfStandingResponse(response *contract.SelfStandingResponse) *selfStandingResponse {
	if response == nil {
		return nil
	}
	return &selfStandingResponse{
		ContestID:         response.ContestID.String(),
		ProjectionVersion: response.ProjectionVersion,
		LastCursor:        response.LastCursor,
		Item:              toStandingResponse(response.Item),
	}
}
