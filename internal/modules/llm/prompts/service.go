package prompts

import (
	"cmp"
	"context"
	"database/sql"
	"encoding/json"
	"slices"
	"strings"

	"github.com/optikklabs/query/internal/shared/nullable"

	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/shared/errorcode"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

var (
	ErrNotFound  = errorcode.NotFoundError{Msg: "prompt not found"}
	errDuplicate = errorcode.ConflictError{Msg: "a prompt with this name already exists"}
)

// versionStatuses are the statuses a version can be set to. "production" is
// not stored on the version: it points the prompt's production_version_id at
// it.
var versionStatuses = []string{"draft", "production", "archived"}

func (s *Service) List(ctx context.Context, tenantID int64) ([]PromptSummary, error) {
	rows, err := s.repo.ListPrompts(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	out := make([]PromptSummary, 0, len(rows))
	for _, row := range rows {
		sum := toSummary(row.promptRow)
		sum.VersionCount = row.VersionCount
		sum.ProductionVersion = row.ProductionVersion
		out = append(out, sum)
	}
	return out, nil
}

func (s *Service) Get(ctx context.Context, tenantID int64, name string) (PromptDetail, error) {
	prompt, err := s.repo.GetPromptByName(ctx, tenantID, name)
	if err != nil {
		return PromptDetail{}, dbutil.NoRowsAs(err, ErrNotFound)
	}
	versions, err := s.repo.ListVersions(ctx, prompt.ID)
	if err != nil {
		return PromptDetail{}, err
	}
	detail := PromptDetail{PromptSummary: toSummary(prompt), Versions: make([]PromptVersion, 0, len(versions))}
	detail.VersionCount = len(versions)
	for _, v := range versions {
		if v.Status == "production" {
			detail.ProductionVersion = &v.Version
		}
		detail.Versions = append(detail.Versions, toVersion(v))
	}
	return detail, nil
}

func (s *Service) Create(ctx context.Context, tenantID, userID int64, req CreatePromptRequest) (PromptDetail, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return PromptDetail{}, errorcode.ValidationError{Msg: "name is required"}
	}
	ptype := cmp.Or(req.Type, "chat")
	if ptype != "chat" && ptype != "text" {
		return PromptDetail{}, errorcode.ValidationError{Msg: "type must be chat or text"}
	}
	if len(req.Template) == 0 {
		return PromptDetail{}, errorcode.ValidationError{Msg: "template is required"}
	}
	p := promptInsertArgs{
		TenantID:  tenantID,
		Name:      name,
		Type:      ptype,
		Tags:      req.Tags,
		CreatedBy: sql.NullInt64{Valid: true, Int64: userID},
	}
	if d := strings.TrimSpace(req.Description); d != "" {
		p.Description = sql.NullString{Valid: true, String: d}
	}
	v := versionInsertArgs{
		TenantID:     tenantID,
		TemplateJSON: []byte(req.Template),
		Variables:    req.Variables,
		CreatedBy:    p.CreatedBy,
	}
	if n := strings.TrimSpace(req.Notes); n != "" {
		v.Notes = sql.NullString{Valid: true, String: n}
	}
	if _, err := s.repo.CreatePrompt(ctx, p, v); err != nil {
		return PromptDetail{}, dbutil.DuplicateAs(err, errDuplicate)
	}
	return s.Get(ctx, tenantID, name)
}

func (s *Service) AddVersion(ctx context.Context, tenantID, userID int64, name string, req CreateVersionRequest) (PromptDetail, error) {
	if len(req.Template) == 0 {
		return PromptDetail{}, errorcode.ValidationError{Msg: "template is required"}
	}
	prompt, err := s.repo.GetPromptByName(ctx, tenantID, name)
	if err != nil {
		return PromptDetail{}, dbutil.NoRowsAs(err, ErrNotFound)
	}
	v := versionInsertArgs{
		PromptID:     prompt.ID,
		TenantID:     tenantID,
		TemplateJSON: []byte(req.Template),
		Variables:    req.Variables,
		Production:   req.Production,
		CreatedBy:    sql.NullInt64{Valid: true, Int64: userID},
	}
	if n := strings.TrimSpace(req.Notes); n != "" {
		v.Notes = sql.NullString{Valid: true, String: n}
	}
	if _, err := s.repo.CreateVersion(ctx, v); err != nil {
		return PromptDetail{}, err
	}
	return s.Get(ctx, tenantID, name)
}

func (s *Service) SetVersionStatus(ctx context.Context, tenantID int64, name string, version int, req UpdateVersionRequest) (PromptDetail, error) {
	if !slices.Contains(versionStatuses, req.Status) {
		return PromptDetail{}, errorcode.ValidationError{Msg: "status must be draft, production or archived"}
	}
	prompt, err := s.repo.GetPromptByName(ctx, tenantID, name)
	if err != nil {
		return PromptDetail{}, dbutil.NoRowsAs(err, ErrNotFound)
	}
	if err := s.repo.SetVersionStatus(ctx, prompt.ID, version, req.Status); err != nil {
		return PromptDetail{}, dbutil.NoRowsAs(err, ErrNotFound)
	}
	return s.Get(ctx, tenantID, name)
}

func toSummary(row promptRow) PromptSummary {
	return PromptSummary{
		ID:          row.ID,
		Name:        row.Name,
		Type:        row.Type,
		Description: row.Description.String,
		Tags:        nullable.OrEmpty(row.Tags),
		UpdatedAt:   cmp.Or(row.UpdatedAt.Time, row.CreatedAt),
	}
}

func toVersion(v versionRow) PromptVersion {
	return PromptVersion{
		Version:   v.Version,
		Template:  json.RawMessage(v.TemplateJSON),
		Variables: nullable.OrEmpty(v.Variables),
		Notes:     v.Notes.String,
		Status:    v.Status,
		CreatedAt: v.CreatedAt,
	}
}
