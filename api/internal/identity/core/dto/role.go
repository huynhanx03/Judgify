package dto

// CreateRoleRequest represents request to create a role.
type CreateRoleRequest struct {
	Name     string `json:"name" validate:"required,min=2,max=50"`
	Level    int    `json:"level" validate:"min=0,max=100"`
	ParentID *int   `json:"parent_id"`
}

// UpdateRoleRequest represents request to update a role.
type UpdateRoleRequest struct {
	ID       int     `json:"-" uri:"id"`
	Name     *string `json:"name" validate:"omitempty,min=2,max=50"`
	Level    *int    `json:"level" validate:"omitempty,min=0,max=100"`
	ParentID *int    `json:"parent_id"`
}

// GetRoleRequest represents request to get a role by ID.
type GetRoleRequest struct {
	ID int `uri:"id" validate:"required"`
}

// DeleteRoleRequest represents request to delete a role.
type DeleteRoleRequest struct {
	ID int `uri:"id" validate:"required"`
}

// RoleResponse represents role data in API response.
type RoleResponse struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Level int    `json:"level"`
}
