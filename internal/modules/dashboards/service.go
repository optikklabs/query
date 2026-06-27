package dashboards

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Service owns CRUD validation, JSON marshaling, and the endpoint allowlist
// enforcement for dashboard pages and their widgets.
type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

var ErrNotFound = errors.New("dashboard not found")

type ErrValidation struct{ Msg string }

func (e ErrValidation) Error() string { return e.Msg }

// --- Pages ---

func (s *Service) CreatePage(ctx context.Context, teamID, userID int64, req CreatePageRequest) (DashboardPageResponse, error) {
	args, err := buildPageArgs(teamID, userID, req)
	if err != nil {
		return DashboardPageResponse{}, err
	}
	id, err := s.repo.CreatePage(ctx, args)
	if err != nil {
		return DashboardPageResponse{}, err
	}
	return s.GetPage(ctx, teamID, id)
}

func (s *Service) UpdatePage(ctx context.Context, teamID, userID, id int64, req UpdatePageRequest) (DashboardPageResponse, error) {
	args, err := buildPageArgs(teamID, userID, req)
	if err != nil {
		return DashboardPageResponse{}, err
	}
	if err := s.repo.UpdatePage(ctx, id, teamID, args); err != nil {
		return DashboardPageResponse{}, mapNotFound(err)
	}
	return s.GetPage(ctx, teamID, id)
}

func (s *Service) DeletePage(ctx context.Context, teamID, id int64) error {
	return mapNotFound(s.repo.DeletePage(ctx, id, teamID))
}

func (s *Service) GetPage(ctx context.Context, teamID, id int64) (DashboardPageResponse, error) {
	row, err := s.repo.GetPageByID(ctx, id, teamID)
	if err != nil {
		return DashboardPageResponse{}, mapNotFound(err)
	}
	return toPageResponse(row), nil
}

func (s *Service) GetPageDetail(ctx context.Context, teamID, id int64) (DashboardPageDetailResponse, error) {
	page, err := s.GetPage(ctx, teamID, id)
	if err != nil {
		return DashboardPageDetailResponse{}, err
	}
	widgets, err := s.ListWidgets(ctx, teamID, id)
	if err != nil {
		return DashboardPageDetailResponse{}, err
	}
	return DashboardPageDetailResponse{DashboardPageResponse: page, Widgets: widgets}, nil
}

func (s *Service) ListPages(ctx context.Context, teamID int64, q ListPagesQuery) (DashboardPageListResponse, error) {
	rows, total, err := s.repo.ListPages(ctx, teamID, q)
	if err != nil {
		return DashboardPageListResponse{}, err
	}
	items := make([]DashboardPageResponse, 0, len(rows))
	for _, row := range rows {
		items = append(items, toPageResponse(row))
	}
	return DashboardPageListResponse{Items: items, Total: total}, nil
}

// --- Widgets ---

func (s *Service) ListWidgets(ctx context.Context, teamID, pageID int64) ([]WidgetResponse, error) {
	rows, err := s.repo.ListWidgets(ctx, pageID, teamID)
	if err != nil {
		return nil, err
	}
	out := make([]WidgetResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, toWidgetResponse(row))
	}
	return out, nil
}

func (s *Service) CreateWidget(ctx context.Context, teamID, pageID int64, req CreateWidgetRequest) (WidgetResponse, error) {
	if err := s.ensurePage(ctx, pageID, teamID); err != nil {
		return WidgetResponse{}, err
	}
	count, err := s.repo.CountWidgets(ctx, pageID, teamID)
	if err != nil {
		return WidgetResponse{}, err
	}
	if count >= maxWidgetsPerPage {
		return WidgetResponse{}, ErrValidation{Msg: fmt.Sprintf("page already has the maximum of %d widgets", maxWidgetsPerPage)}
	}
	args, err := buildWidgetArgs(teamID, pageID, req)
	if err != nil {
		return WidgetResponse{}, err
	}
	id, err := s.repo.CreateWidget(ctx, args)
	if err != nil {
		return WidgetResponse{}, err
	}
	return s.getWidget(ctx, teamID, pageID, id)
}

func (s *Service) UpdateWidget(ctx context.Context, teamID, pageID, widgetID int64, req UpdateWidgetRequest) (WidgetResponse, error) {
	args, err := buildWidgetArgs(teamID, pageID, req)
	if err != nil {
		return WidgetResponse{}, err
	}
	args.PageID = pageID
	if err := s.repo.UpdateWidget(ctx, widgetID, args); err != nil {
		return WidgetResponse{}, mapNotFound(err)
	}
	return s.getWidget(ctx, teamID, pageID, widgetID)
}

func (s *Service) DeleteWidget(ctx context.Context, teamID, pageID, widgetID int64) error {
	return mapNotFound(s.repo.DeleteWidget(ctx, widgetID, pageID, teamID))
}

func (s *Service) getWidget(ctx context.Context, teamID, pageID, widgetID int64) (WidgetResponse, error) {
	row, err := s.repo.GetWidgetByID(ctx, widgetID, pageID, teamID)
	if err != nil {
		return WidgetResponse{}, mapNotFound(err)
	}
	return toWidgetResponse(row), nil
}

func (s *Service) ensurePage(ctx context.Context, pageID, teamID int64) error {
	ok, err := s.repo.PageExists(ctx, pageID, teamID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotFound
	}
	return nil
}

// --- Validation + serialization ---

func buildPageArgs(teamID, userID int64, req CreatePageRequest) (pageInsertArgs, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return pageInsertArgs{}, ErrValidation{Msg: "name is required"}
	}
	icon := strings.TrimSpace(req.Icon)
	if icon == "" {
		icon = "layout-grid"
	}
	iconColor := strings.TrimSpace(req.IconColor)
	if iconColor == "" {
		iconColor = "primary"
	}
	tagsJSON, err := json.Marshal(req.Tags)
	if err != nil || len(req.Tags) == 0 {
		tagsJSON = []byte("[]")
	}
	args := pageInsertArgs{
		TeamID:     teamID,
		Name:       name,
		Icon:       icon,
		IconColor:  iconColor,
		TagsJSON:   tagsJSON,
		IsFavorite: req.IsFavorite,
	}
	if desc := strings.TrimSpace(req.Description); desc != "" {
		args.Description = sql.NullString{Valid: true, String: desc}
	}
	if userID > 0 {
		args.CreatedByUserID = sql.NullInt64{Valid: true, Int64: userID}
	}
	return args, nil
}

func buildWidgetArgs(teamID, pageID int64, req CreateWidgetRequest) (widgetInsertArgs, error) {
	if err := validateWidget(req); err != nil {
		return widgetInsertArgs{}, err
	}
	args := widgetInsertArgs{
		PageID:     pageID,
		TeamID:     teamID,
		PanelType:  req.PanelType,
		SpecJSON:   []byte(req.Spec),
		LayoutJSON: []byte(req.Layout),
		Position:   req.Position,
	}
	if title := strings.TrimSpace(req.Title); title != "" {
		args.Title = sql.NullString{Valid: true, String: title}
	}
	if lv := strings.TrimSpace(req.LayoutVariant); lv != "" {
		args.LayoutVariant = sql.NullString{Valid: true, String: lv}
	}
	return args, nil
}

// validateWidget enforces the dashboard-safe contract: a known panel type, a
// well-formed grid layout, and a query that targets an allowlisted endpoint.
func validateWidget(req CreateWidgetRequest) error {
	if !isValidPanelType(req.PanelType) {
		return ErrValidation{Msg: fmt.Sprintf("panel_type %q is not a supported dashboard panel", req.PanelType)}
	}
	if lv := strings.TrimSpace(req.LayoutVariant); lv != "" && !isValidLayoutVariant(lv) {
		return ErrValidation{Msg: fmt.Sprintf("layout_variant %q is not supported", lv)}
	}
	if err := validateLayout(req.Layout); err != nil {
		return err
	}
	return validateQuery(req.Spec)
}

type layoutProbe struct {
	X *float64 `json:"x"`
	Y *float64 `json:"y"`
	W *float64 `json:"w"`
	H *float64 `json:"h"`
}

func validateLayout(raw json.RawMessage) error {
	var l layoutProbe
	if err := json.Unmarshal(raw, &l); err != nil {
		return ErrValidation{Msg: "layout must be a {x,y,w,h} object"}
	}
	if l.X == nil || l.Y == nil || l.W == nil || l.H == nil {
		return ErrValidation{Msg: "layout requires x, y, w and h"}
	}
	if *l.W <= 0 || *l.H <= 0 {
		return ErrValidation{Msg: "layout w and h must be positive"}
	}
	if *l.X < 0 || *l.Y < 0 {
		return ErrValidation{Msg: "layout x and y must not be negative"}
	}
	return nil
}

type builderFilterProbe struct {
	Operator string `json:"operator"`
}

type builderQueryProbe struct {
	MetricName  string               `json:"metricName"`
	Aggregation string               `json:"aggregation"`
	Where       []builderFilterProbe `json:"where"`
}

type querySpecProbe struct {
	Query *struct {
		Kind     string              `json:"kind"`
		Endpoint string              `json:"endpoint"`
		Queries  []builderQueryProbe `json:"queries"`
	} `json:"query"`
}

// validateQuery accepts either a curated-endpoint widget or a metrics
// query-builder widget. The endpoint variant stays restricted to the safe
// allowlist; the builder variant is validated structurally — its query is
// replayed against the team-scoped metrics engine, so no SQL is persisted.
func validateQuery(spec json.RawMessage) error {
	var probe querySpecProbe
	if err := json.Unmarshal(spec, &probe); err != nil {
		return ErrValidation{Msg: "spec must be a valid panel spec object"}
	}
	if probe.Query == nil {
		return ErrValidation{Msg: "spec.query is required"}
	}
	if probe.Query.Kind == "metrics" {
		return validateBuilderQuery(probe.Query.Queries)
	}
	if strings.TrimSpace(probe.Query.Endpoint) == "" {
		return ErrValidation{Msg: "spec.query.endpoint is required"}
	}
	if !isAllowedEndpoint(probe.Query.Endpoint) {
		return ErrValidation{Msg: fmt.Sprintf("spec.query.endpoint %q is not a dashboard-safe endpoint", probe.Query.Endpoint)}
	}
	return nil
}

func validateBuilderQuery(queries []builderQueryProbe) error {
	if len(queries) == 0 {
		return ErrValidation{Msg: "spec.query.queries must have at least one query"}
	}
	for _, q := range queries {
		if strings.TrimSpace(q.MetricName) == "" {
			return ErrValidation{Msg: "spec.query.queries[].metricName is required"}
		}
		if !isValidBuilderAggregation(q.Aggregation) {
			return ErrValidation{Msg: fmt.Sprintf("aggregation %q is not supported", q.Aggregation)}
		}
		for _, f := range q.Where {
			if !isValidBuilderOperator(f.Operator) {
				return ErrValidation{Msg: fmt.Sprintf("filter operator %q is not supported", f.Operator)}
			}
		}
	}
	return nil
}

func mapNotFound(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
