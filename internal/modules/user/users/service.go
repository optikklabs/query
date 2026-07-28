package users

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/optikklabs/query/internal/modules/user/auth"
	"github.com/optikklabs/query/internal/modules/user/shared"
	"github.com/optikklabs/query/internal/shared/errorcode"
)

type repository interface {
	CreateUser(context.Context, string, string, string, int64, string, time.Time) (int64, error)
	FindUserByID(context.Context, int64, int64) (shared.UserRecord, error)
	ListUsersByTenantID(context.Context, int64) ([]shared.UserRecord, error)
	UpdateUserRole(context.Context, int64, int64, string) error
	CountActiveAdmins(context.Context, int64) (int, error)
	DeactivateUser(context.Context, int64, int64) error
}

type Service struct {
	repo        repository
	authService *auth.Service
}

func NewService(repo repository, authService *auth.Service) *Service {
	return &Service{repo: repo, authService: authService}
}

func (s *Service) CreateUser(ctx context.Context, req CreateUserRequest, tenantID int64) (UserResponse, error) {
	role := req.Role
	if role == "" {
		role = shared.RoleMember
	}
	if !shared.IsValidRole(role) {
		return UserResponse{}, errorcode.ValidationError{Msg: "role must be 'admin' or 'member'"}
	}

	hashStr := ""
	if req.Password != "" {
		if len(req.Password) < shared.MinPasswordLength {
			return UserResponse{}, errorcode.ValidationError{Msg: "Password must be at least 8 characters"}
		}
		hash, err := shared.HashPassword(req.Password)
		if err != nil {
			return UserResponse{}, fmt.Errorf("Failed to hash password: %w", err)
		}
		hashStr = hash
	}

	userID, err := s.repo.CreateUser(ctx, req.Email, hashStr, req.Name, tenantID, role, time.Now().UTC())
	if err != nil {
		return UserResponse{}, fmt.Errorf("Failed to create user: %w", err)
	}

	created, err := s.repo.FindUserByID(ctx, userID, tenantID)
	if err != nil {
		return UserResponse{}, fmt.Errorf("Failed to load created user: %w", err)
	}

	if req.Password == "" {
		if err := s.authService.ForgotPassword(ctx, req.Email); err != nil {

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

func (s *Service) ListUsers(ctx context.Context, tenantID int64) ([]UserResponse, error) {
	records, err := s.repo.ListUsersByTenantID(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("Failed to list users: %w", err)
	}
	responses := make([]UserResponse, 0, len(records))
	for _, record := range records {
		responses = append(responses, s.buildUserResponse(record))
	}
	return responses, nil
}

func (s *Service) SetUserRole(ctx context.Context, userID, tenantID int64, role string) (UserResponse, error) {
	if !shared.IsValidRole(role) {
		return UserResponse{}, errorcode.ValidationError{Msg: "role must be 'admin' or 'member'"}
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
		return UserResponse{}, fmt.Errorf("Failed to update user role: %w", err)
	}
	user.Role = role
	return s.buildUserResponse(user), nil
}

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
		return fmt.Errorf("Failed to remove user: %w", err)
	}
	return nil
}

func (s *Service) findInTenant(ctx context.Context, userID, tenantID int64) (shared.UserRecord, error) {
	user, err := s.repo.FindUserByID(ctx, userID, tenantID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return shared.UserRecord{}, errorcode.NotFoundError{Msg: "User not found"}
		}
		return shared.UserRecord{}, fmt.Errorf("Failed to load user: %w", err)
	}
	return user, nil
}

func (s *Service) guardLastAdmin(ctx context.Context, tenantID int64) error {
	admins, err := s.repo.CountActiveAdmins(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("Failed to count admins: %w", err)
	}
	if admins <= 1 {
		return errorcode.ConflictError{Msg: "Cannot remove or demote the last admin of the tenant"}
	}
	return nil
}
