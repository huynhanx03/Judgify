package dto

type UpdateProfileRequest struct {
	FirstName string `json:"first_name" validate:"required,max=50"`
	LastName  string `json:"last_name" validate:"required,max=50"`
	Gender    int    `json:"gender" validate:"oneof=0 1 2"`
	Birthday  string `json:"birthday" validate:"required,datetime=2006-01-02"`
}

type GetProfileRequest struct{}

type ProfileResponse struct {
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Gender    int    `json:"gender"`
	Birthday  string `json:"birthday"`
}

type CreateUserRequest struct {
	Username  string `json:"username" validate:"required,min=3,max=50"`
	Password  string `json:"password" validate:"required,min=6"`
	RoleID    int    `json:"role_id"`
	FirstName string `json:"first_name" validate:"required,max=50"`
	LastName  string `json:"last_name" validate:"required,max=50"`
	Gender    int    `json:"gender" validate:"oneof=0 1 2"`
	Birthday  string `json:"birthday" validate:"required,datetime=2006-01-02"`
}

// DeleteUserRequest represents the request to delete a user.
type DeleteUserRequest struct {
	ID int `uri:"id" validate:"required"`
}
