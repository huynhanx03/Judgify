package http

import contract "github.com/huynhanx03/judgify/internal/cultivation/contract"

func toRankingEntryResponse(entry *contract.RankingEntry) *rankingEntryResponse {
	if entry == nil {
		return nil
	}
	return &rankingEntryResponse{
		Rank: entry.Rank, PublicID: entry.PublicID.String(),
		DisplayName: entry.DisplayName, DisplayMode: entry.DisplayMode,
		Rating: entry.Rating, RankTitle: entry.RankTitle, TotalExp: entry.TotalExp,
		LevelName: entry.LevelName, Level: entry.Level,
	}
}

func toRankingPageResponse(response *contract.RankingPageResponse) *rankingPageResponse {
	if response == nil {
		return nil
	}
	entries := make([]*rankingEntryResponse, len(response.Entries))
	for index, entry := range response.Entries {
		entries[index] = toRankingEntryResponse(entry)
	}
	return &rankingPageResponse{
		Entries: entries, NextCursor: response.NextCursor,
		ProjectionRevision: response.ProjectionRevision,
	}
}

func toRankingPrivacyResponse(response *contract.RankingPrivacyResponse) *rankingPrivacyResponse {
	if response == nil {
		return nil
	}
	return &rankingPrivacyResponse{
		Visibility: response.Visibility, Version: response.Version,
		ProjectionRevision: response.ProjectionRevision, Replayed: response.Replayed,
	}
}
