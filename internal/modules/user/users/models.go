package users

// CreateUserRequest adds a user to the caller's tenant. The tenant is taken
// from the authenticated context, so it is intentionally not part of the body.
type CreateUserRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Name     string `json:"name" validate:"required"`
	Role     string `json:"role"`
	Password string `json:"password" validate:"required"`
}

// UpdateRoleRequest promotes or demotes a user (admin|member).
type UpdateRoleRequest struct {
	Role string `json:"role" validate:"required"`
}

type UserResponse struct {
	ID        int64  `json:"id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	Active    bool   `json:"active"`
	CreatedAt any    `json:"createdAt"`
	TenantID  int64  `json:"tenantId"`
	Role      string `json:"role"`
}
