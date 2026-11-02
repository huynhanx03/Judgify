package http

type userTraitIDRequest struct {
	ID string `uri:"id" validate:"required,uuid"`
}
type userTraitResponse struct {
	ID      string `json:"id"`
	UserID  string `json:"user_id"`
	TraitID string `json:"trait_id"`
}
