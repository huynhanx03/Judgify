package http

import contract "github.com/huynhanx03/judgify/internal/contest/contract"

func toContestDraftRequest(value contestDraftRequest) contract.ContestDraftRequest {
	problems := make([]contract.ContestDraftProblemRequest, len(value.Problems))
	for i, p := range value.Problems {
		problems[i] = contract.ContestDraftProblemRequest{ProblemID: p.ProblemID, Alias: p.Alias, Points: p.Points, VisibleBeforeStart: p.VisibleBeforeStart}
	}
	return contract.ContestDraftRequest{Reason: value.Reason, Title: value.Title, Description: value.Description, StartTime: value.StartTime, EndTime: value.EndTime, RegistrationMode: value.RegistrationMode, RegistrationOpensAt: value.RegistrationOpensAt, RegistrationClosesAt: value.RegistrationClosesAt, MaxParticipants: value.MaxParticipants, FreezeStandingsAt: value.FreezeStandingsAt, Rated: value.Rated, Problems: problems}
}
func toCreateContestRequest(value *createContestRequest) *contract.CreateContestRequest {
	if value == nil {
		return nil
	}
	return &contract.CreateContestRequest{ContestDraftRequest: toContestDraftRequest(value.contestDraftRequest)}
}
func toUpdateContestRequest(value *updateContestRequest) *contract.UpdateContestRequest {
	if value == nil {
		return nil
	}
	return &contract.UpdateContestRequest{ID: value.ID, ExpectedVersion: value.ExpectedVersion, ContestDraftRequest: toContestDraftRequest(value.contestDraftRequest)}
}
func toDeleteContestRequest(value *deleteContestRequest) *contract.DeleteContestRequest {
	if value == nil {
		return nil
	}
	return &contract.DeleteContestRequest{ID: value.ID, Reason: value.Reason}
}
