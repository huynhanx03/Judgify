package dto

// GachaRollRequest represents a gacha roll request.
type GachaRollRequest struct{}

// GachaRollResponse represents gacha roll results.
type GachaRollResponse struct {
	RootBones []*TraitResponse `json:"root_bones"`
	Talents   []*TraitResponse `json:"talents"`
}
