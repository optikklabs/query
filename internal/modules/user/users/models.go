package users

import "time"

type CreateUserRequest struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Role     string `json:"role"`
	Password string `json:"password"`
}

type UpdateRoleRequest struct {
	Role string `json:"role"`
}

type UserResponse struct {
	ID        int64     `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"createdAt"`
	TenantID  int64     `json:"tenantId"`
	Role      string    `json:"role"`
}
