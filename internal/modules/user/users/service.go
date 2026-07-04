package users

import (
	"time"

	"github.com/optikklabs/query/internal/modules/user/shared"
	"golang.org/x/crypto/bcrypt"
)

// Service provisions users: admin-created accounts and the platform super-admin.
type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) CreateUser(req CreateUserRequest) (UserResponse, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return UserResponse{}, shared.NewInternalError("Failed to hash password", err)
	}

	role := req.Role
	if role == "" {
		role = "member"
	}

	userID, err := s.repo.CreateUser(req.Email, string(hash), req.Name, req.TenantID, false, time.Now().UTC())
	if err != nil {
		return UserResponse{}, shared.NewInternalError("Failed to create user", err)
	}

	created, err := s.repo.FindUserByID(userID)
	if err != nil {
		return UserResponse{}, shared.NewInternalError("Failed to load created user", err)
	}
	return s.buildUserResponse(created, role), nil
}

// EnsureSuperAdmin seeds the platform super-admin if it does not already exist.
// Idempotent: a no-op when the email is unset or the user is already present.
func (s *Service) EnsureSuperAdmin(email, password string) error {
	if email == "" || password == "" {
		return nil
	}
	if _, err := s.repo.FindActiveUserByEmail(email); err == nil {
		return nil // already seeded
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return shared.NewInternalError("Failed to hash admin password", err)
	}
	// Super admin needs a valid tenant id, assuming 1 or passing 0 if allowed
	_, err = s.repo.CreateUser(email, string(hash), "Platform Admin", 1, true, time.Now().UTC())
	return err
}

func (s *Service) buildUserResponse(user shared.UserRecord, role string) UserResponse {
	return UserResponse{
		ID:        user.ID,
		Email:     user.Email,
		Name:      user.Name,
		Active:    user.Active,
		CreatedAt: user.CreatedAt,
		TenantID:  user.TenantID,
		Role:      role,
	}
}

// ListUsers returns all active users belonging to the given tenant.
func (s *Service) ListUsers(tenantID int64) ([]UserResponse, error) {
	records, err := s.repo.ListUsersByTenantID(tenantID)
	if err != nil {
		return nil, shared.NewInternalError("Failed to list users", err)
	}
	responses := make([]UserResponse, 0, len(records))
	for _, record := range records {
		resp := s.buildUserResponse(record, "member") // Default to member for now
		responses = append(responses, resp)
	}
	return responses, nil
}

// RemoveUser soft-deletes a user by setting active = 0.
func (s *Service) RemoveUser(userID int64) error {
	if err := s.repo.DeactivateUser(userID); err != nil {
		return shared.NewInternalError("Failed to remove user", err)
	}
	return nil
}
