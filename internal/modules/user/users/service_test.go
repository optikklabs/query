package users

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/optikklabs/query/internal/modules/user/shared"
)

// fakeRepo is an in-memory stand-in for the users repository, scoped by tenant.
type fakeRepo struct {
	users  map[int64]shared.UserRecord
	nextID int64
}

func newFakeRepo(seed ...shared.UserRecord) *fakeRepo {
	f := &fakeRepo{users: map[int64]shared.UserRecord{}, nextID: 100}
	for _, u := range seed {
		f.users[u.ID] = u
	}
	return f
}

func (f *fakeRepo) CreateUser(_ context.Context, _, _, _ string, tenantID int64, role string, _ time.Time) (int64, error) {
	f.nextID++
	f.users[f.nextID] = shared.UserRecord{ID: f.nextID, TenantID: tenantID, Role: role, Active: true}
	return f.nextID, nil
}

func (f *fakeRepo) FindUserByID(_ context.Context, userID, tenantID int64) (shared.UserRecord, error) {
	u, ok := f.users[userID]
	if !ok || u.TenantID != tenantID || !u.Active {
		return shared.UserRecord{}, sql.ErrNoRows
	}
	return u, nil
}

func (f *fakeRepo) ListUsersByTenantID(_ context.Context, tenantID int64) ([]shared.UserRecord, error) {
	var out []shared.UserRecord
	for _, u := range f.users {
		if u.TenantID == tenantID && u.Active {
			out = append(out, u)
		}
	}
	return out, nil
}

func (f *fakeRepo) UpdateUserRole(_ context.Context, userID, tenantID int64, role string) error {
	u := f.users[userID]
	u.Role = role
	f.users[userID] = u
	return nil
}

func (f *fakeRepo) CountActiveAdmins(_ context.Context, tenantID int64) (int, error) {
	n := 0
	for _, u := range f.users {
		if u.TenantID == tenantID && u.Active && u.Role == shared.RoleAdmin {
			n++
		}
	}
	return n, nil
}

func (f *fakeRepo) DeactivateUser(_ context.Context, userID, tenantID int64) error {
	u := f.users[userID]
	u.Active = false
	f.users[userID] = u
	return nil
}

func admin(id, tenant int64) shared.UserRecord {
	return shared.UserRecord{ID: id, TenantID: tenant, Role: shared.RoleAdmin, Active: true}
}
func member(id, tenant int64) shared.UserRecord {
	return shared.UserRecord{ID: id, TenantID: tenant, Role: shared.RoleMember, Active: true}
}

func TestCreateUserRejectsInvalidRole(t *testing.T) {
	s := NewService(newFakeRepo(), nil)
	_, err := s.CreateUser(context.Background(), CreateUserRequest{Email: "a@b.com", Name: "A", Password: "pw", Role: "superuser"}, 1)
	if err == nil {
		t.Fatal("expected invalid role to be rejected")
	}
}

func TestCreateUserDefaultsToMemberInCallerTenant(t *testing.T) {
	f := newFakeRepo()
	s := NewService(f, nil)
	u, err := s.CreateUser(context.Background(), CreateUserRequest{Email: "a@b.com", Name: "A", Password: "pw"}, 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.Role != shared.RoleMember || u.TenantID != 42 {
		t.Fatalf("got role=%q tenant=%d, want member/42", u.Role, u.TenantID)
	}
}

func TestSetUserRoleBlocksDemotingLastAdmin(t *testing.T) {
	f := newFakeRepo(admin(1, 7))
	s := NewService(f, nil)
	if _, err := s.SetUserRole(context.Background(), 1, 7, shared.RoleMember); err == nil {
		t.Fatal("expected last-admin demotion to be blocked")
	}
	if f.users[1].Role != shared.RoleAdmin {
		t.Fatal("role must be unchanged after blocked demotion")
	}
}

func TestSetUserRoleAllowsDemotionWhenAnotherAdminExists(t *testing.T) {
	f := newFakeRepo(admin(1, 7), admin(2, 7))
	s := NewService(f, nil)
	if _, err := s.SetUserRole(context.Background(), 1, 7, shared.RoleMember); err != nil {
		t.Fatalf("expected demotion allowed, got %v", err)
	}
	if f.users[1].Role != shared.RoleMember {
		t.Fatal("role should be member after demotion")
	}
}

func TestSetUserRoleOtherTenantIsNotFound(t *testing.T) {
	f := newFakeRepo(admin(1, 7), member(2, 7))
	s := NewService(f, nil)
	// User 1 belongs to tenant 7; caller is tenant 9.
	if _, err := s.SetUserRole(context.Background(), 1, 9, shared.RoleMember); err == nil {
		t.Fatal("expected cross-tenant role change to be not-found")
	}
}

func TestRemoveUserBlocksLastAdmin(t *testing.T) {
	f := newFakeRepo(admin(1, 7))
	s := NewService(f, nil)
	if err := s.RemoveUser(context.Background(), 1, 7); err == nil {
		t.Fatal("expected removing last admin to be blocked")
	}
	if !f.users[1].Active {
		t.Fatal("admin must remain active after blocked removal")
	}
}

func TestRemoveUserAllowsMember(t *testing.T) {
	f := newFakeRepo(admin(1, 7), member(2, 7))
	s := NewService(f, nil)
	if err := s.RemoveUser(context.Background(), 2, 7); err != nil {
		t.Fatalf("expected member removal to succeed, got %v", err)
	}
	if f.users[2].Active {
		t.Fatal("member should be deactivated")
	}
}

func TestRemoveUserOtherTenantIsNotFound(t *testing.T) {
	f := newFakeRepo(member(2, 7))
	s := NewService(f, nil)
	if err := s.RemoveUser(context.Background(), 2, 9); err == nil {
		t.Fatal("expected cross-tenant removal to be not-found")
	}
}
