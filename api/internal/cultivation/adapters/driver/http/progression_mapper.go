package http

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/huynhanx03/judgify/internal/ports"
)

func parseProgressionUUID(value string) (uuid.UUID, error) {
	id, err := uuid.Parse(value)
	if err != nil {
		return uuid.Nil, fmt.Errorf("parse progression id: %w", err)
	}
	return id, nil
}
func parseOptionalProgressionUUID(value *string) (*uuid.UUID, error) {
	if value == nil {
		return nil, nil
	}
	id, err := parseProgressionUUID(*value)
	if err != nil {
		return nil, err
	}
	return &id, nil
}
func stringProgressionUUID(value *uuid.UUID) *string {
	if value == nil {
		return nil
	}
	result := value.String()
	return &result
}
func toProgressionLedgerPageResponse(page ports.ProgressionLedgerPage) *progressionLedgerPageResponse {
	entries := make([]progressionLedgerEntryResponse, len(page.Entries))
	for i, e := range page.Entries {
		entries[i] = progressionLedgerEntryResponse{ID: e.ID.String(), Kind: e.Kind, SourceType: e.SourceType, SourceAggregateID: e.SourceAggregateID.String(), SourceVersion: e.SourceVersion, ReversedEntryID: stringProgressionUUID(e.ReversedEntryID), RewardRuleRevisionID: stringProgressionUUID(e.RewardRuleRevisionID), RewardProfileRevisionID: stringProgressionUUID(e.RewardProfileRevisionID), SourceChecksum: fmt.Sprintf("%x", e.SourceChecksum), Delta: e.Delta, BalanceBefore: e.BalanceBefore, BalanceAfter: e.BalanceAfter, ProgressionVersion: e.ProgressionVersion, IncidentID: e.IncidentID, Reason: e.Reason, CorrelationID: e.CorrelationID, OccurredAt: e.OccurredAt}
	}
	return &progressionLedgerPageResponse{Entries: entries, NextCursor: stringProgressionUUID(page.NextCursor)}
}
