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
	spec, err := validateWidget(req.Spec)
	if err != nil {
		return widgetInsertArgs{}, err
	}
	args := widgetInsertArgs{
		PageID:     pageID,
		TenantID:   tenantID,
		PanelType:  spec.PanelType,
		SpecJSON:   []byte(req.Spec),
		LayoutJSON: []byte(spec.Layout),
		Position:   req.Position,
	}
	if title := strings.TrimSpace(spec.Title); title != "" {
		args.Title = sql.NullString{Valid: true, String: title}
	}
	if lv := strings.TrimSpace(spec.LayoutVariant); lv != "" {
		args.LayoutVariant = sql.NullString{Valid: true, String: lv}
	}
	return args, nil
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
