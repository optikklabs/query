package prompts

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
)

// Service owns prompt validation, version lifecycle rules, and row mapping.
type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

var ErrNotFound = errors.New("prompt not found")

type ErrValidation struct{ Msg string }

func (e ErrValidation) Error() string { return e.Msg }

var validVersionStatus = map[string]struct{}{
	"draft": {}, "production": {}, "archived": {},
}

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
		return PromptDetail{}, mapNotFound(err)
	}
	versions, err := s.repo.ListVersions(ctx, prompt.ID)
	if err != nil {
		return PromptDetail{}, err
	}
	detail := PromptDetail{PromptSummary: toSummary(prompt)}
	detail.VersionCount = len(versions)
	for _, v := range versions {
		if v.Status == "production" {
			ver := v.Version
			detail.ProductionVersion = &ver
		}
		detail.Versions = append(detail.Versions, toVersion(v))
	}
	return detail, nil
}

func (s *Service) Create(ctx context.Context, tenantID, userID int64, req CreatePromptRequest) (PromptDetail, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return PromptDetail{}, ErrValidation{Msg: "name is required"}
	}
	ptype := req.Type
	if ptype == "" {
		ptype = "chat"
	}
	if ptype != "chat" && ptype != "text" {
		return PromptDetail{}, ErrValidation{Msg: "type must be chat or text"}
	}
	if len(req.Template) == 0 {
		return PromptDetail{}, ErrValidation{Msg: "template is required"}
	}
	p := promptInsertArgs{
		TenantID: tenantID,
		Name:     name,
		Type:     ptype,
		TagsJSON: marshalStrings(req.Tags),
	}
	if d := strings.TrimSpace(req.Description); d != "" {
		p.Description = sql.NullString{Valid: true, String: d}
	}
	if userID > 0 {
		p.CreatedBy = sql.NullInt64{Valid: true, Int64: userID}
	}
	v := versionInsertArgs{
		TenantID:      tenantID,
		TemplateJSON:  []byte(req.Template),
		VariablesJSON: marshalStrings(req.Variables),
		CreatedBy:     p.CreatedBy,
	}
	if n := strings.TrimSpace(req.Notes); n != "" {
		v.Notes = sql.NullString{Valid: true, String: n}
	}
	if _, err := s.repo.CreatePrompt(ctx, p, v); err != nil {
		return PromptDetail{}, err
	}
	return s.Get(ctx, tenantID, name)
}

func (s *Service) AddVersion(ctx context.Context, tenantID, userID int64, name string, req CreateVersionRequest) (PromptDetail, error) {
	if len(req.Template) == 0 {
		return PromptDetail{}, ErrValidation{Msg: "template is required"}
	}
	prompt, err := s.repo.GetPromptByName(ctx, tenantID, name)
	if err != nil {
		return PromptDetail{}, mapNotFound(err)
	}
	v := versionInsertArgs{
		PromptID:      prompt.ID,
		TenantID:      tenantID,
		TemplateJSON:  []byte(req.Template),
		VariablesJSON: marshalStrings(req.Variables),
		Production:    req.Production,
	}
	if n := strings.TrimSpace(req.Notes); n != "" {
		v.Notes = sql.NullString{Valid: true, String: n}
	}
	if userID > 0 {
		v.CreatedBy = sql.NullInt64{Valid: true, Int64: userID}
	}
	if _, err := s.repo.CreateVersion(ctx, v); err != nil {
		return PromptDetail{}, err
	}
	return s.Get(ctx, tenantID, name)
}

func (s *Service) SetVersionStatus(ctx context.Context, tenantID int64, name string, version int, req UpdateVersionRequest) (PromptDetail, error) {
	if _, ok := validVersionStatus[req.Status]; !ok {
		return PromptDetail{}, ErrValidation{Msg: "status must be draft, production or archived"}
	}
	prompt, err := s.repo.GetPromptByName(ctx, tenantID, name)
	if err != nil {
		return PromptDetail{}, mapNotFound(err)
	}
	if err := s.repo.SetVersionStatus(ctx, prompt.ID, version, req.Status); err != nil {
		return PromptDetail{}, mapNotFound(err)
	}
	return s.Get(ctx, tenantID, name)
}

// --- mapping helpers ---

func toSummary(row promptRow) PromptSummary {
	sum := PromptSummary{
		ID:   row.ID,
		Name: row.Name,
		Type: row.Type,
		Tags: unmarshalStrings(row.TagsJSON),
	}
	if row.Description != nil {
		sum.Description = *row.Description
	}
	sum.UpdatedAt = row.CreatedAt
	if row.UpdatedAt != nil {
		sum.UpdatedAt = *row.UpdatedAt
	}
	return sum
}

func toVersion(v versionRow) PromptVersion {
	pv := PromptVersion{
		Version:   v.Version,
		Template:  json.RawMessage(v.TemplateJSON),
		Variables: unmarshalStrings(v.VariablesJSON),
		Status:    v.Status,
		CreatedAt: v.CreatedAt,
	}
	if v.Notes != nil {
		pv.Notes = *v.Notes
	}
	return pv
}

func marshalStrings(in []string) []byte {
	if len(in) == 0 {
		return []byte("[]")
	}
	b, err := json.Marshal(in)
	if err != nil {
		return []byte("[]")
	}
	return b
}

func unmarshalStrings(raw []byte) []string {
	out := []string{}
	if len(raw) == 0 {
		return out
	}
	_ = json.Unmarshal(raw, &out)
	return out
}

func mapNotFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
