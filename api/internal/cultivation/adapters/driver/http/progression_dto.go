package http

import "time"

type previewProgressionAdjustmentRequest struct {
	UserID          string  `json:"user_id" validate:"required,uuid"`
	Delta           int64   `json:"delta" validate:"min=-1000000,max=1000000"`
	ReversedEntryID *string `json:"reversed_entry_id" validate:"omitempty,uuid"`
}
type applyProgressionAdjustmentRequest struct {
	UserID          string  `json:"user_id" validate:"required,uuid"`
	Delta           int64   `json:"delta" validate:"required,min=-1000000,max=1000000"`
	ExpectedBalance int64   `json:"expected_balance" validate:"min=0,max=9007199254740991"`
	ExpectedVersion int64   `json:"expected_version" validate:"min=0"`
	IncidentID      string  `json:"incident_id" validate:"required,min=3,max=128"`
	Reason          string  `json:"reason" validate:"required,min=3,max=1024"`
	PreviewChecksum string  `json:"preview_checksum" validate:"required,len=64"`
	ReversedEntryID *string `json:"reversed_entry_id" validate:"omitempty,uuid"`
}
type progressionLedgerRequest struct {
	UserID string  `form:"user_id" validate:"required,uuid"`
	Cursor *string `form:"cursor" validate:"omitempty,uuid"`
	Limit  int     `form:"limit" validate:"omitempty,min=1,max=100"`
}
type myProgressionLedgerRequest struct {
	Cursor *string `form:"cursor" validate:"omitempty,uuid"`
	Limit  int     `form:"limit" validate:"omitempty,min=1,max=100"`
}
type progressionAdjustmentPreviewResponse struct {
	UserID          string  `json:"user_id"`
	Delta           int64   `json:"delta"`
	BalanceBefore   int64   `json:"balance_before"`
	BalanceAfter    int64   `json:"balance_after"`
	ExpectedVersion int64   `json:"expected_version"`
	ReversedEntryID *string `json:"reversed_entry_id,omitempty"`
	PreviewChecksum string  `json:"preview_checksum"`
}
type progressionAdjustmentResponse struct {
	LedgerEntryID string    `json:"ledger_entry_id"`
	UserID        string    `json:"user_id"`
	Delta         int64     `json:"delta"`
	BalanceBefore int64     `json:"balance_before"`
	BalanceAfter  int64     `json:"balance_after"`
	Version       int64     `json:"version"`
	Replayed      bool      `json:"replayed"`
	CreatedAt     time.Time `json:"created_at"`
}
type progressionLedgerEntryResponse struct {
	ID                      string    `json:"id"`
	Kind                    string    `json:"kind"`
	SourceType              string    `json:"source_type"`
	SourceAggregateID       string    `json:"source_aggregate_id"`
	SourceVersion           int64     `json:"source_version"`
	ReversedEntryID         *string   `json:"reversed_entry_id,omitempty"`
	RewardRuleRevisionID    *string   `json:"reward_rule_revision_id,omitempty"`
	RewardProfileRevisionID *string   `json:"reward_profile_revision_id,omitempty"`
	SourceChecksum          string    `json:"source_checksum"`
	Delta                   int64     `json:"delta"`
	BalanceBefore           int64     `json:"balance_before"`
	BalanceAfter            int64     `json:"balance_after"`
	ProgressionVersion      int64     `json:"progression_version"`
	IncidentID              string    `json:"incident_id,omitempty"`
	Reason                  string    `json:"reason,omitempty"`
	CorrelationID           string    `json:"correlation_id"`
	OccurredAt              time.Time `json:"occurred_at"`
}
type progressionLedgerPageResponse struct {
	Entries    []progressionLedgerEntryResponse `json:"entries"`
	NextCursor *string                          `json:"next_cursor,omitempty"`
}
