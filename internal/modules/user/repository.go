package user

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/optikklabs/query/internal/app/registry"
	dbutil "github.com/optikklabs/query/internal/infra/database"
)

// Repository implements the *Repository interface for MySQL.
type Repository struct {
	db *sqlx.DB
}

func NewRepository(db *sql.DB, appConfig registry.AppConfig) *Repository {
	return &Repository{
		db: sqlx.NewDb(db, "mysql"),
	}
}

func (r *Repository) FindActiveUserByID(userID int64) (UserRecord, error) {
	var u UserRecord
	err := dbutil.GetSQL(context.Background(), r.db, "user.FindActiveUserByID", &u, `
		SELECT id, email, name, avatar_url, teams, active, last_login_at, created_at
		FROM users
		WHERE id = ? AND active = 1
		LIMIT 1
	`, userID)
	return u, err
}

func (r *Repository) FindActiveUserByEmail(email string) (AuthUser, error) {
	var u AuthUser
	err := dbutil.GetSQL(context.Background(), r.db, "user.FindActiveUserByEmail", &u, `
		SELECT id, email, password_hash, name, avatar_url, teams, is_admin
		FROM users
		WHERE email = ? AND active = 1
		LIMIT 1
	`, strings.TrimSpace(email))
	return u, err
}

func (r *Repository) UpdateUserLastLogin(userID int64, at time.Time) error {
	_, err := dbutil.ExecSQL(context.Background(), r.db, "user.UpdateUserLastLogin", `
		UPDATE users SET last_login_at = ? WHERE id = ?
	`, at, userID)
	return err
}

func (r *Repository) InsertRefreshToken(userID int64, familyID, tokenHash string, expiresAt time.Time) error {
	_, err := dbutil.ExecSQL(context.Background(), r.db, "user.InsertRefreshToken", `
		INSERT INTO refresh_tokens (user_id, family_id, token_hash, expires_at)
		VALUES (?, ?, ?, ?)
	`, userID, familyID, tokenHash, expiresAt)
	return err
}

func (r *Repository) FindRefreshTokenByHash(tokenHash string) (RefreshTokenRecord, error) {
	var t RefreshTokenRecord
	err := dbutil.GetSQL(context.Background(), r.db, "user.FindRefreshTokenByHash", &t, `
		SELECT id, user_id, family_id, token_hash, expires_at, revoked_at, created_at
		FROM refresh_tokens
		WHERE token_hash = ?
		LIMIT 1
	`, tokenHash)
	return t, err
}

func (r *Repository) RevokeRefreshToken(tokenHash string) error {
	_, err := dbutil.ExecSQL(context.Background(), r.db, "user.RevokeRefreshToken", `
		UPDATE refresh_tokens SET revoked_at = ? WHERE token_hash = ? AND revoked_at IS NULL
	`, time.Now().UTC(), tokenHash)
	return err
}

func (r *Repository) RevokeRefreshTokenFamily(familyID string) error {
	_, err := dbutil.ExecSQL(context.Background(), r.db, "user.RevokeRefreshTokenFamily", `
		UPDATE refresh_tokens SET revoked_at = ? WHERE family_id = ? AND revoked_at IS NULL
	`, time.Now().UTC(), familyID)
	return err
}

func (r *Repository) InsertDeviceCode(ctx context.Context, deviceCode, userCode string, expiresAt time.Time) error {
	_, err := dbutil.ExecSQL(ctx, r.db, "user.InsertDeviceCode", `
		INSERT INTO device_codes (device_code, user_code, expires_at)
		VALUES (?, ?, ?)
	`, deviceCode, userCode, expiresAt)
	return err
}

func (r *Repository) FindDeviceCode(ctx context.Context, deviceCode string) (DeviceCodeRecord, error) {
	var d DeviceCodeRecord
	err := dbutil.GetSQL(ctx, r.db, "user.FindDeviceCode", &d, `
		SELECT id, device_code, user_code, user_id, approved_at, consumed_at, last_polled_at, expires_at, created_at
		FROM device_codes
		WHERE device_code = ?
		LIMIT 1
	`, deviceCode)
	return d, err
}

func (r *Repository) FindDeviceCodeByUserCode(ctx context.Context, userCode string) (DeviceCodeRecord, error) {
	var d DeviceCodeRecord
	err := dbutil.GetSQL(ctx, r.db, "user.FindDeviceCodeByUserCode", &d, `
		SELECT id, device_code, user_code, user_id, approved_at, consumed_at, last_polled_at, expires_at, created_at
		FROM device_codes
		WHERE user_code = ?
		LIMIT 1
	`, userCode)
	return d, err
}

func (r *Repository) TouchDeviceCodePolled(ctx context.Context, deviceCode string, at time.Time) error {
	_, err := dbutil.ExecSQL(ctx, r.db, "user.TouchDeviceCodePolled", `
		UPDATE device_codes SET last_polled_at = ? WHERE device_code = ?
	`, at, deviceCode)
	return err
}

func (r *Repository) ApproveDeviceCode(ctx context.Context, userCode string, userID int64, at time.Time) error {
	_, err := dbutil.ExecSQL(ctx, r.db, "user.ApproveDeviceCode", `
		UPDATE device_codes SET approved_at = ?, user_id = ? WHERE user_code = ? AND approved_at IS NULL
	`, at, userID, userCode)
	return err
}

func (r *Repository) ConsumeDeviceCode(ctx context.Context, deviceCode string, at time.Time) error {
	_, err := dbutil.ExecSQL(ctx, r.db, "user.ConsumeDeviceCode", `
		UPDATE device_codes SET consumed_at = ? WHERE device_code = ? AND consumed_at IS NULL
	`, at, deviceCode)
	return err
}

func (r *Repository) FindTeamByID(teamID int64) (TeamRecord, error) {
	var t TeamRecord
	err := dbutil.GetSQL(context.Background(), r.db, "user.FindTeamByID", &t, `
		SELECT id, org_name, name, slug, description, active, color, icon, api_key, created_at
		FROM teams
		WHERE id = ?
		LIMIT 1
	`, teamID)
	return t, err
}

func (r *Repository) FindTeamBySlug(orgName, slug string) (TeamRecord, error) {
	var t TeamRecord
	err := dbutil.GetSQL(context.Background(), r.db, "user.FindTeamBySlug", &t, `
		SELECT id, org_name, name, slug, description, active, color, icon, api_key, created_at
		FROM teams
		WHERE org_name = ? AND slug = ?
		LIMIT 1
	`, orgName, slug)
	return t, err
}

func (r *Repository) FindTeamByOrgAndName(orgName, teamName string) (TeamRecord, error) {
	var t TeamRecord
	err := dbutil.GetSQL(context.Background(), r.db, "user.FindTeamByOrgAndName", &t, `
		SELECT id, org_name, name, slug, description, active, color, icon, api_key, created_at
		FROM teams
		WHERE org_name = ? AND name = ? AND active = 1
		LIMIT 1
	`, orgName, teamName)
	return t, err
}

func (r *Repository) ListActiveTeamsByOrganization(orgName string) ([]TeamRecord, error) {
	var records []TeamRecord
	err := dbutil.SelectSQL(context.Background(), r.db, "user.ListActiveTeamsByOrganization", &records, `
		SELECT id, org_name, name, slug, description, active, color, icon, api_key, created_at
		FROM teams
		WHERE org_name = ? AND active = 1
		ORDER BY created_at DESC
	`, orgName)
	return records, err
}

func (r *Repository) ListActiveTeamsByIDs(teamIDs []int64) ([]TeamRecord, error) {
	if len(teamIDs) == 0 {
		return []TeamRecord{}, nil
	}
	query, args, err := sqlx.In(`
		SELECT id, org_name, name, slug, description, active, color, icon, api_key, created_at
		FROM teams
		WHERE id IN (?) AND active = 1
		ORDER BY created_at DESC
	`, teamIDs)
	if err != nil {
		return nil, err
	}
	query = r.db.Rebind(query)
	var records []TeamRecord
	if err := dbutil.SelectSQL(context.Background(), r.db, "user.ListActiveTeamsByIDs", &records, query, args...); err != nil {
		return nil, err
	}
	return records, nil
}

func (r *Repository) CreateTeam(orgName, name, slug string, description, icon *string, color, apiKey string, createdAt time.Time) (int64, error) {
	res, err := dbutil.ExecSQL(context.Background(), r.db, "user.CreateTeam", `
		INSERT INTO teams (org_name, name, slug, description, icon, active, color, api_key, created_at)
		VALUES (?, ?, ?, ?, ?, 1, ?, ?, ?)
	`, orgName, name, slug, description, icon, color, apiKey, createdAt)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// NewSignupTeam carries the prepared fields for an atomic signup insert.
type NewSignupTeam struct {
	OrgName, Name, Slug, Color, APIKey string
	Email, UserName, PasswordHash      string
	CreatedAt                          time.Time
}

// SignupTeamAndAdmin inserts the org team and its first admin user in one
// transaction; a failed user insert (e.g. duplicate email) rolls back the
// team so signup never orphans a team. Returns the new team and user IDs.
func (r *Repository) SignupTeamAndAdmin(ctx context.Context, t NewSignupTeam) (int64, int64, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = tx.Rollback() }() // no-op once committed

	res, err := tx.ExecContext(ctx, `
		INSERT INTO teams (org_name, name, slug, active, color, api_key, created_at)
		VALUES (?, ?, ?, 1, ?, ?, ?)
	`, t.OrgName, t.Name, t.Slug, t.Color, t.APIKey, t.CreatedAt)
	if err != nil {
		return 0, 0, err
	}
	teamID, err := res.LastInsertId()
	if err != nil {
		return 0, 0, err
	}

	teamsJSON, err := BuildTeamMembershipsJSON([]TeamMembership{{TeamID: teamID, Role: "admin"}})
	if err != nil {
		return 0, 0, err
	}

	res, err = tx.ExecContext(ctx, `
		INSERT INTO users (email, password_hash, name, teams, active, is_admin, created_at)
		VALUES (?, ?, ?, ?, 1, 0, ?)
	`, t.Email, NullableString(t.PasswordHash), t.UserName, teamsJSON, t.CreatedAt)
	if err != nil {
		return 0, 0, err
	}
	userID, err := res.LastInsertId()
	if err != nil {
		return 0, 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, 0, err
	}
	return teamID, userID, nil
}

func (r *Repository) UpdateTeamAPIKey(ctx context.Context, teamID int64, apiKey string) error {
	_, err := dbutil.ExecSQL(ctx, r.db, "user.UpdateTeamAPIKey", `
		UPDATE teams SET api_key = ? WHERE id = ?
	`, apiKey, teamID)
	return err
}

func (r *Repository) FindTeamIDByAPIKey(ctx context.Context, apiKey string) (int64, error) {
	var teamID int64
	err := dbutil.GetSQL(ctx, r.db, "user.FindTeamIDByAPIKey", &teamID, `
		SELECT id FROM teams WHERE api_key = ? AND active = 1 LIMIT 1
	`, apiKey)
	return teamID, err
}

func (r *Repository) FindUserByID(userID int64) (UserRecord, error) {
	var u UserRecord
	err := dbutil.GetSQL(context.Background(), r.db, "user.FindUserByID", &u, `
		SELECT id, email, name, avatar_url, teams, active, last_login_at, created_at
		FROM users
		WHERE id = ?
		LIMIT 1
	`, userID)
	return u, err
}

func (r *Repository) ListActiveUsersByTeamIDs(teamIDs []int64, limit, offset int) ([]UserRecord, error) {
	if len(teamIDs) == 0 {
		return []UserRecord{}, nil
	}

	conditions := make([]string, 0, len(teamIDs))
	args := make([]any, 0, len(teamIDs)+2)
	for _, teamID := range teamIDs {
		conditions = append(conditions, `JSON_CONTAINS(teams, ?)`)
		args = append(args, fmt.Sprintf(`{"team_id":%d}`, teamID))
	}
	args = append(args, limit, offset)

	var records []UserRecord
	err := dbutil.SelectSQL(context.Background(), r.db, "user.ListActiveUsersByTeamIDs", &records, fmt.Sprintf(`
		SELECT id, email, name, avatar_url, teams, active, last_login_at, created_at
		FROM users
		WHERE (%s) AND active = 1
		ORDER BY id
		LIMIT ? OFFSET ?
	`, strings.Join(conditions, " OR ")), args...)
	return records, err
}

func (r *Repository) CreateUser(email, passwordHash, name string, avatarURL, teamsJSON *string, isAdmin bool, createdAt time.Time) (int64, error) {
	res, err := dbutil.ExecSQL(context.Background(), r.db, "user.CreateUser", `
		INSERT INTO users (email, password_hash, name, avatar_url, teams, active, is_admin, created_at)
		VALUES (?, ?, ?, ?, ?, 1, ?, ?)
	`, email, NullableString(passwordHash), name, avatarURL, teamsJSON, isAdmin, createdAt)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (r *Repository) UpdateUserProfile(userID int64, name, avatarURL *string) error {
	_, err := dbutil.ExecSQL(context.Background(), r.db, "user.UpdateUserProfile", `
		UPDATE users
		SET name = COALESCE(?, name), avatar_url = COALESCE(?, avatar_url)
		WHERE id = ?
	`, name, avatarURL, userID)
	return err
}

func (r *Repository) UpdateUserTeams(userID int64, teamsJSON string) error {
	_, err := dbutil.ExecSQL(context.Background(), r.db, "user.UpdateUserTeams", `
		UPDATE users SET teams = ? WHERE id = ?
	`, teamsJSON, userID)
	return err
}
