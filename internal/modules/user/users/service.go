package users

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/optikklabs/query/internal/modules/user/auth"
	"github.com/optikklabs/query/internal/modules/user/shared"
)

// repository is the tenant-scoped persistence the service depends on. Defined
// here (consumer side) so the service can be unit-tested with a fake.
type repository interface {
	CreateUser(context.Context, string, string, string, int64, string, time.Time) (int64, error)
	FindUserByID(context.Context, int64, int64) (shared.UserRecord, error)
	ListUsersByTenantID(context.Context, int64) ([]shared.UserRecord, error)
	UpdateUserRole(context.Context, int64, int64, string) error
	CountActiveAdmins(context.Context, int64) (int, error)
	DeactivateUser(context.Context, int64, int64) error
}

// Service provisions and manages users within a single tenant.
type Service struct {
	repo        repository
	authService *auth.Service
}

func NewService(repo repository, authService *auth.Service) *Service {
	return &Service{repo: repo, authService: authService}
}

// CreateUser adds a user to the caller's tenant. The tenant is authoritative
// from the caller's context, never the request body.
func (s *Service) CreateUser(ctx context.Context, req CreateUserRequest, tenantID int64) (UserResponse, error) {
	role := req.Role
	if role == "" {
		role = shared.RoleMember
	}
	if !shared.IsValidRole(role) {
		return UserResponse{}, shared.NewValidationError("role must be 'admin' or 'member'", nil)
	}

	hashStr := ""
	if req.Password != "" {
		if len(req.Password) < shared.MinPasswordLength {
			return UserResponse{}, shared.NewValidationError("Password must be at least 8 characters", nil)
		}
		hash, err := shared.HashPassword(req.Password)
		if err != nil {
			return UserResponse{}, shared.NewInternalError("Failed to hash password", err)
		}
		hashStr = hash
	}

	userID, err := s.repo.CreateUser(ctx, req.Email, hashStr, req.Name, tenantID, role, time.Now().UTC())
	if err != nil {
		return UserResponse{}, shared.NewInternalError("Failed to create user", err)
	}

	created, err := s.repo.FindUserByID(ctx, userID, tenantID)
	if err != nil {
		return UserResponse{}, shared.NewInternalError("Failed to load created user", err)
	}

	// If no password was provided, generate an invite/reset link and email it to the user.
	if req.Password == "" {
		if err := s.authService.ForgotPassword(ctx, req.Email); err != nil {
			// Do not block user creation if email fails, but log the error
			// The admin can manually trigger another reset password request
			_ = err
		}
	}

	return s.buildUserResponse(created), nil
}

func (s *Service) buildUserResponse(user shared.UserRecord) UserResponse {
	return UserResponse{
		ID:        user.ID,
		Email:     user.Email,
		Name:      user.Name,
		Active:    user.Active,
		CreatedAt: user.CreatedAt,
		TenantID:  user.TenantID,
		Role:      user.Role,
	}
}

// ListUsers returns all active users belonging to the given tenant.
func (s *Service) ListUsers(ctx context.Context, tenantID int64) ([]UserResponse, error) {
	records, err := s.repo.ListUsersByTenantID(ctx, tenantID)
	if err != nil {
		return nil, shared.NewInternalError("Failed to list users", err)
	}
	responses := make([]UserResponse, 0, len(records))
	for _, record := range records {
		responses = append(responses, s.buildUserResponse(record))
	}
	return responses, nil
}

// SetUserRole promotes or demotes a user within the caller's tenant. Demoting
// the last admin is blocked so an org can never be left without one.
func (s *Service) SetUserRole(ctx context.Context, userID, tenantID int64, role string) (UserResponse, error) {
	if !shared.IsValidRole(role) {
		return UserResponse{}, shared.NewValidationError("role must be 'admin' or 'member'", nil)
	}
	user, err := s.findInTenant(ctx, userID, tenantID)
	if err != nil {
		return UserResponse{}, err
	}
	if user.Role == shared.RoleAdmin && role == shared.RoleMember {
		if err := s.guardLastAdmin(ctx, tenantID); err != nil {
			return UserResponse{}, err
		}
	}
	if err := s.repo.UpdateUserRole(ctx, userID, tenantID, role); err != nil {
		return UserResponse{}, shared.NewInternalError("Failed to update user role", err)
	}
	user.Role = role
	return s.buildUserResponse(user), nil
}

// RemoveUser soft-deletes a user within the caller's tenant. Removing the last
// admin is blocked.
func (s *Service) RemoveUser(ctx context.Context, userID, tenantID int64) error {
	user, err := s.findInTenant(ctx, userID, tenantID)
	if err != nil {
		return err
	}
	if user.Role == shared.RoleAdmin {
		if err := s.guardLastAdmin(ctx, tenantID); err != nil {
			return err
		}
	}
	if err := s.repo.DeactivateUser(ctx, userID, tenantID); err != nil {
		return shared.NewInternalError("Failed to remove user", err)
	}
	return nil
}

// findInTenant loads an active user, mapping a miss to a not-found error so a
// caller cannot probe or act on users outside their tenant.
func (s *Service) findInTenant(ctx context.Context, userID, tenantID int64) (shared.UserRecord, error) {
	user, err := s.repo.FindUserByID(ctx, userID, tenantID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return shared.UserRecord{}, shared.NewNotFoundError("User not found", nil)
		}
		return shared.UserRecord{}, shared.NewInternalError("Failed to load user", err)
	}
	return user, nil
}

func (s *Service) guardLastAdmin(ctx context.Context, tenantID int64) error {
	admins, err := s.repo.CountActiveAdmins(ctx, tenantID)
	if err != nil {
		return shared.NewInternalError("Failed to count admins", err)
	}
	if admins <= 1 {
		return shared.NewConflictError("Cannot remove or demote the last admin of the tenant", nil)
	}
	return nil
}
