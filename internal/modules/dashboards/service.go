package dashboards

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/optikklabs/query/internal/shared/errorcode"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

var ErrNotFound = errorcode.NotFoundError{Msg: "dashboard not found"}

func (s *Service) CreatePage(ctx context.Context, tenantID, userID int64, req CreatePageRequest) (DashboardPageResponse, error) {
	args, err := buildPageArgs(tenantID, userID, req)
	if err != nil {
		return DashboardPageResponse{}, err
	}
	id, err := s.repo.CreatePage(ctx, args)
	if err != nil {
		return DashboardPageResponse{}, err
	}
	return s.GetPage(ctx, tenantID, id)
}

func (s *Service) UpdatePage(ctx context.Context, tenantID, userID, id int64, req UpdatePageRequest) (DashboardPageResponse, error) {
	args, err := buildPageArgs(tenantID, userID, req)
	if err != nil {
		return DashboardPageResponse{}, err
	}
	if err := s.repo.UpdatePage(ctx, id, tenantID, args); err != nil {
		return DashboardPageResponse{}, mapNotFound(err)
	}
	return s.GetPage(ctx, tenantID, id)
}

func (s *Service) DeletePage(ctx context.Context, tenantID, id int64) error {
	return mapNotFound(s.repo.DeletePage(ctx, id, tenantID))
}

func (s *Service) GetPage(ctx context.Context, tenantID, id int64) (DashboardPageResponse, error) {
	row, err := s.repo.GetPageByID(ctx, id, tenantID)
	if err != nil {
		return DashboardPageResponse{}, mapNotFound(err)
	}
	return toPageResponse(row), nil
}

func (s *Service) GetPageDetail(ctx context.Context, tenantID, id int64) (DashboardPageDetailResponse, error) {
	page, err := s.GetPage(ctx, tenantID, id)
	if err != nil {
		return DashboardPageDetailResponse{}, err
	}
	widgets, err := s.ListWidgets(ctx, tenantID, id)
	if err != nil {
		return DashboardPageDetailResponse{}, err
	}
	return DashboardPageDetailResponse{DashboardPageResponse: page, Widgets: widgets}, nil
}

func (s *Service) ListPages(ctx context.Context, tenantID int64, q ListPagesQuery) (DashboardPageListResponse, error) {
	rows, total, err := s.repo.ListPages(ctx, tenantID, q)
	if err != nil {
		return DashboardPageListResponse{}, err
	}
	items := make([]DashboardPageResponse, 0, len(rows))
	for _, row := range rows {
		items = append(items, toPageResponse(row))
	}
	return DashboardPageListResponse{Items: items, Total: total}, nil
}

func (s *Service) ListWidgets(ctx context.Context, tenantID, pageID int64) ([]WidgetResponse, error) {
	rows, err := s.repo.ListWidgets(ctx, pageID, tenantID)
	if err != nil {
		return nil, err
	}
	out := make([]WidgetResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, toWidgetResponse(row))
	}
	return out, nil
}

func (s *Service) CreateWidget(ctx context.Context, tenantID, pageID int64, req CreateWidgetRequest) (WidgetResponse, error) {
	if err := s.ensurePage(ctx, pageID, tenantID); err != nil {
		return WidgetResponse{}, err
	}
	count, err := s.repo.CountWidgets(ctx, pageID, tenantID)
	if err != nil {
		return WidgetResponse{}, err
	}
	if count >= maxWidgetsPerPage {
		return WidgetResponse{}, errorcode.ValidationError{Msg: fmt.Sprintf("page already has the maximum of %d widgets", maxWidgetsPerPage)}
	}
	args, err := buildWidgetArgs(tenantID, pageID, req)
	if err != nil {
		return WidgetResponse{}, err
	}
	id, err := s.repo.CreateWidget(ctx, args)
	if err != nil {
		return WidgetResponse{}, err
	}
	return s.getWidget(ctx, tenantID, pageID, id)
}

func (s *Service) UpdateWidget(ctx context.Context, tenantID, pageID, widgetID int64, req UpdateWidgetRequest) (WidgetResponse, error) {
	args, err := buildWidgetArgs(tenantID, pageID, req)
	if err != nil {
		return WidgetResponse{}, err
	}
	args.PageID = pageID
	if err := s.repo.UpdateWidget(ctx, widgetID, args); err != nil {
		return WidgetResponse{}, mapNotFound(err)
	}
	return s.getWidget(ctx, tenantID, pageID, widgetID)
}

func (s *Service) DeleteWidget(ctx context.Context, tenantID, pageID, widgetID int64) error {
	return mapNotFound(s.repo.DeleteWidget(ctx, widgetID, pageID, tenantID))
}

func (s *Service) getWidget(ctx context.Context, tenantID, pageID, widgetID int64) (WidgetResponse, error) {
	row, err := s.repo.GetWidgetByID(ctx, widgetID, pageID, tenantID)
	if err != nil {
		return WidgetResponse{}, mapNotFound(err)
	}
	return toWidgetResponse(row), nil
}

func (s *Service) ensurePage(ctx context.Context, pageID, tenantID int64) error {
	ok, err := s.repo.PageExists(ctx, pageID, tenantID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotFound
	}
	return nil
}

func buildPageArgs(tenantID, userID int64, req CreatePageRequest) (pageInsertArgs, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return pageInsertArgs{}, errorcode.ValidationError{Msg: "name is required"}
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
		TenantID:   tenantID,
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

func buildWidgetArgs(tenantID, pageID int64, req CreateWidgetRequest) (widgetInsertArgs, error) {
	if err := validateWidget(req); err != nil {
		return widgetInsertArgs{}, err
	}
	args := widgetInsertArgs{
		PageID:     pageID,
		TenantID:   tenantID,
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

func validateWidget(req CreateWidgetRequest) error {
	if !isValidPanelType(req.PanelType) {
		return errorcode.ValidationError{Msg: fmt.Sprintf("panel_type %q is not a supported dashboard panel", req.PanelType)}
	}
	if lv := strings.TrimSpace(req.LayoutVariant); lv != "" && !isValidLayoutVariant(lv) {
		return errorcode.ValidationError{Msg: fmt.Sprintf("layout_variant %q is not supported", lv)}
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
		return errorcode.ValidationError{Msg: "layout must be a {x,y,w,h} object"}
	}
	if l.X == nil || l.Y == nil || l.W == nil || l.H == nil {
		return errorcode.ValidationError{Msg: "layout requires x, y, w and h"}
	}
	if *l.W <= 0 || *l.H <= 0 {
		return errorcode.ValidationError{Msg: "layout w and h must be positive"}
	}
	if *l.X < 0 || *l.Y < 0 {
		return errorcode.ValidationError{Msg: "layout x and y must not be negative"}
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

func validateQuery(spec json.RawMessage) error {
	var probe querySpecProbe
	if err := json.Unmarshal(spec, &probe); err != nil {
		return errorcode.ValidationError{Msg: "spec must be a valid panel spec object"}
	}
	if probe.Query == nil {
		return errorcode.ValidationError{Msg: "spec.query is required"}
	}
	if probe.Query.Kind == "metrics" {
		return validateBuilderQuery(probe.Query.Queries)
	}
	if strings.TrimSpace(probe.Query.Endpoint) == "" {
		return errorcode.ValidationError{Msg: "spec.query.endpoint is required"}
	}
	if !isAllowedEndpoint(probe.Query.Endpoint) {
		return errorcode.ValidationError{Msg: fmt.Sprintf("spec.query.endpoint %q is not a dashboard-safe endpoint", probe.Query.Endpoint)}
	}
	return nil
}

func validateBuilderQuery(queries []builderQueryProbe) error {
	if len(queries) == 0 {
		return errorcode.ValidationError{Msg: "spec.query.queries must have at least one query"}
	}
	for _, q := range queries {
		if strings.TrimSpace(q.MetricName) == "" {
			return errorcode.ValidationError{Msg: "spec.query.queries[].metricName is required"}
		}
		if !isValidBuilderAggregation(q.Aggregation) {
			return errorcode.ValidationError{Msg: fmt.Sprintf("aggregation %q is not supported", q.Aggregation)}
		}
		for _, f := range q.Where {
			if !isValidBuilderOperator(f.Operator) {
				return errorcode.ValidationError{Msg: fmt.Sprintf("filter operator %q is not supported", f.Operator)}
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
