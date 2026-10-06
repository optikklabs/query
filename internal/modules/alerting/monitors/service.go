package monitors

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	dbutil "github.com/optikklabs/query/internal/infra/database"
	models "github.com/optikklabs/query/internal/modules/alerting/shared/models"
	alertquery "github.com/optikklabs/query/internal/modules/alerting/shared/query"
	"github.com/optikklabs/query/internal/shared/errorcode"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

var ErrNotFound = errorcode.NotFoundError{Msg: "monitor not found"}

func (s *Service) Create(ctx context.Context, tenantID, userID int64, req CreateMonitorRequest) (MonitorResponse, error) {
	row, err := buildMonitorRow(tenantID, userID, req)
	if err != nil {
		return MonitorResponse{}, err
	}
	id, err := s.repo.Create(ctx, row)
	if err != nil {
		return MonitorResponse{}, err
	}
	return s.GetByID(ctx, tenantID, id)
}

func (s *Service) Update(ctx context.Context, tenantID, userID, id int64, req UpdateMonitorRequest) (MonitorResponse, error) {
	row, err := buildMonitorRow(tenantID, userID, req)
	if err != nil {
		return MonitorResponse{}, err
	}
	if err := s.repo.Update(ctx, id, row); err != nil {
		return MonitorResponse{}, dbutil.NoRowsAs(err, ErrNotFound)
	}
	return s.GetByID(ctx, tenantID, id)
}

func (s *Service) Delete(ctx context.Context, tenantID, id int64) error {
	return dbutil.NoRowsAs(s.repo.Delete(ctx, id, tenantID), ErrNotFound)
}

func (s *Service) GetByID(ctx context.Context, tenantID, id int64) (MonitorResponse, error) {
	row, state, err := s.repo.GetByID(ctx, id, tenantID)
	if err != nil {
		return MonitorResponse{}, dbutil.NoRowsAs(err, ErrNotFound)
	}
	return toResponse(row, state), nil
}

func (s *Service) List(ctx context.Context, tenantID int64, q ListQuery) (MonitorListResponse, error) {
	rows, err := s.repo.List(ctx, tenantID, q)
	if err != nil {
		return MonitorListResponse{}, err
	}
	counts, err := s.repo.Count(ctx, tenantID, q)
	if err != nil {
		return MonitorListResponse{}, err
	}
	items := make([]MonitorResponse, 0, len(rows))
	for _, row := range rows {
		items = append(items, toResponse(row.Split()))
	}
	return MonitorListResponse{Items: items, Counts: counts}, nil
}

func buildMonitorRow(tenantID, userID int64, req CreateMonitorRequest) (models.MonitorRow, error) {
	name, priority, evalEvery, err := validateCreateRequest(req)
	if err != nil {
		return models.MonitorRow{}, err
	}
	row := models.MonitorRow{
		TenantID:     tenantID,
		Name:         name,
		Type:         req.Type,
		Priority:     priority,
		Scope:        req.Scope,
		Query:        req.Query,
		Conditions:   req.Conditions,
		Notify:       req.Notify,
		Tags:         req.Tags,
		EvalEverySec: evalEvery,
	}
	if msg := strings.TrimSpace(req.MessageBody); msg != "" {
		row.MessageBody = sql.NullString{Valid: true, String: msg}
	}
	if url := strings.TrimSpace(req.RunbookURL); url != "" {
		row.RunbookURL = sql.NullString{Valid: true, String: url}
	}
	if req.RenotifyEverySec != nil && *req.RenotifyEverySec > 0 {
		row.RenotifyEverySec = sql.NullInt64{Valid: true, Int64: int64(*req.RenotifyEverySec)}
	}
	if userID > 0 {
		row.CreatedByUserID = sql.NullInt64{Valid: true, Int64: userID}
	}
	return row, nil
}

func validateCreateRequest(req CreateMonitorRequest) (name, priority string, evalEvery int, err error) {
	name = strings.TrimSpace(req.Name)
	if name == "" {
		return "", "", 0, errorcode.ValidationError{Msg: "name is required"}
	}
	if !models.IsValidType(req.Type) {
		return "", "", 0, errorcode.ValidationError{Msg: fmt.Sprintf("type must be one of %v", models.SupportedMonitorTypes)}
	}
	priority = req.Priority
	if priority == "" {
		priority = "P2"
	}
	if !models.IsValidPriority(priority) {
		return "", "", 0, errorcode.ValidationError{Msg: fmt.Sprintf("priority must be one of %v", models.SupportedPriorities)}
	}
	if err := validateQueryForType(req.Type, req.Query); err != nil {
		return "", "", 0, err
	}
	if _, _, err := alertquery.CompileScope(req.Type, req.Scope, nil); err != nil {
		return "", "", 0, errorcode.ValidationError{Msg: err.Error()}
	}
	if err := validateConditions(req.Conditions); err != nil {
		return "", "", 0, err
	}
	evalEvery = req.EvalEverySec
	if evalEvery <= 0 {
		evalEvery = 300
	}
	return name, priority, evalEvery, nil
}

func validateQueryForType(t string, q models.MonitorQuery) error {
	switch t {
	case "metric":
		return validateMetricQuery(q.Metric)
	case "apm":
		return validateAPMQuery(q.APM)
	case "log":
		if q.Log == nil || strings.TrimSpace(q.Log.Query) == "" {
			return errorcode.ValidationError{Msg: "log query requires query.log.query"}
		}
	}
	return nil
}

func validateMetricQuery(query *models.MetricQuery) error {
	if query == nil || strings.TrimSpace(query.Metric) == "" {
		return errorcode.ValidationError{Msg: "metric query requires query.metric.metric"}
	}
	switch query.Aggregation {
	case "avg", "sum", "min", "max", "p50", "p95", "p99":
	default:
		return errorcode.ValidationError{Msg: "unsupported metric aggregation"}
	}
	return nil
}

func validateAPMQuery(query *models.APMQuery) error {
	if query == nil || strings.TrimSpace(query.Service) == "" {
		return errorcode.ValidationError{Msg: "apm query requires query.apm.service"}
	}
	switch query.Track {
	case "errors", "hits", "latency":
		return nil
	default:
		return errorcode.ValidationError{Msg: "unsupported apm track"}
	}
}

func validateConditions(c models.Conditions) error {
	switch c.Comparator {
	case "above", "below", "equal":
	case "":
		return errorcode.ValidationError{Msg: "conditions.comparator is required"}
	default:
		return errorcode.ValidationError{Msg: "conditions.comparator must be above, below, or equal"}
	}
	if c.AlertThreshold == nil {
		return errorcode.ValidationError{Msg: "conditions.alertThreshold is required"}
	}
	if c.NoDataAfterSec < 0 {
		return errorcode.ValidationError{Msg: "conditions.noDataAfterSec must not be negative"}
	}
	switch c.NoDataAs {
	case "no_data", "alert", "ok", "":
	default:
		return errorcode.ValidationError{Msg: "conditions.noDataAs must be no_data, alert, or ok"}
	}
	return nil
}
