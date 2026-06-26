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
		SELECT id, email, password_hash, name, avatar_url, teams
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

func (r *Repository) CreateUser(email, passwordHash, name string, avatarURL, teamsJSON *string, createdAt time.Time) (int64, error) {
	res, err := dbutil.ExecSQL(context.Background(), r.db, "user.CreateUser", `
		INSERT INTO users (email, password_hash, name, avatar_url, teams, active, created_at)
		VALUES (?, ?, ?, ?, ?, 1, ?)
	`, email, NullableString(passwordHash), name, avatarURL, teamsJSON, createdAt)
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
