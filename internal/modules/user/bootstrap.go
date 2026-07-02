package user

import (
	"time"

	"golang.org/x/crypto/bcrypt"
)

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
		return NewInternalError("Failed to hash admin password", err)
	}
	emptyTeams := "[]"
	_, err = s.repo.CreateUser(email, string(hash), "Platform Admin", nil, &emptyTeams, true, time.Now().UTC())
	return err
}
