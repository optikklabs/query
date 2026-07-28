package users

type CreateUserRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Name     string `json:"name" validate:"required"`
	Role     string `json:"role"`
	Password string `json:"password"`
}

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
