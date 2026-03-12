package dto

// CreatePermissionRequest represents request to create a permission.
type CreatePermissionRequest struct {
	RoleID      int     `json:"role_id" validate:"required"`
	ResourceID  int     `json:"resource_id" validate:"required"`
	Description *string `json:"description" validate:"omitempty,max=255"`
	Scopes      int     `json:"scopes"`
}

// UpdatePermissionRequest represents request to update a permission.
type UpdatePermissionRequest struct {
	ID          int     `json:"-" uri:"id"`
	Description *string `json:"description" validate:"omitempty,max=255"`
	Scopes      *int    `json:"scopes"`
}

// GetPermissionRequest represents request to get a permission by ID.
type GetPermissionRequest struct {
	ID int `uri:"id" validate:"required"`
}

// DeletePermissionRequest represents request to delete a permission.
type DeletePermissionRequest struct {
	ID int `uri:"id" validate:"required"`
}

// PermissionResponse represents permission data in API response.
type PermissionResponse struct {
	ID          int     `json:"id"`
	RoleID      int     `json:"role_id"`
	ResourceID  int     `json:"resource_id"`
	Description *string `json:"description"`
	Scopes      int     `json:"scopes"`
}
