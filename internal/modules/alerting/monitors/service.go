package monitors

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
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
	// Counts back the status tabs, so they ignore the tab filters.
	tabless := q
	tabless.Statuses, tabless.Muted = nil, nil
	counts, err := s.repo.Count(ctx, tenantID, tabless)
	if err != nil {
		return MonitorListResponse{}, err
	}
	items := make([]MonitorResponse, 0, len(rows))
	for _, row := range rows {
		items = append(items, toResponse(row.MonitorRow, row.MonitorStateRow))
	}
	return MonitorListResponse{Items: items, Counts: counts}, nil
}

func buildMonitorRow(tenantID, userID int64, req CreateMonitorRequest) (models.MonitorRow, error) {
	name, priority, evalEvery, err := validateCreateRequest(req)
	if err != nil {
		return models.MonitorRow{}, err
	}
	row := models.MonitorRow{
		TenantID:        tenantID,
		Name:            name,
		Type:            req.Type,
		Priority:        priority,
		Scope:           req.Scope,
		Query:           req.Query,
		Conditions:      req.Conditions,
		Notify:          req.Notify,
		Tags:            req.Tags,
		EvalEverySec:    evalEvery,
		CreatedByUserID: sql.NullInt64{Valid: true, Int64: userID},
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
	if req.EvalEverySec <= 0 {
		return "", "", 0, errorcode.ValidationError{Msg: "evalEverySec must be positive"}
	}
	return name, priority, req.EvalEverySec, nil
}

func validateQueryForType(t string, q models.MonitorQuery) error {
	switch t {
	case "metric":
		return validateMetricQuery(q.Metric)
	case "apm":
		return validateAPMQuery(q.APM)
	default: // "log"
		if q.Log == nil || strings.TrimSpace(q.Log.Query) == "" {
			return errorcode.ValidationError{Msg: "log query requires query.log.query"}
		}
		return validateWindow(q.Log.WindowSec)
	}
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
	return validateWindow(query.WindowSec)
}

func validateAPMQuery(query *models.APMQuery) error {
	if query == nil || strings.TrimSpace(query.Service) == "" {
		return errorcode.ValidationError{Msg: "apm query requires query.apm.service"}
	}
	switch query.Track {
	case "errors", "hits", "latency":
	default:
		return errorcode.ValidationError{Msg: "unsupported apm track"}
	}
	return validateWindow(query.WindowSec)
}

// maxWindowSec bounds the evaluation window a monitor queries.
const maxWindowSec = 24 * 60 * 60

func validateWindow(windowSec int) error {
	if windowSec <= 0 || windowSec > maxWindowSec {
		return errorcode.ValidationError{Msg: fmt.Sprintf("query windowSec must be between 1 and %d", maxWindowSec)}
	}
	return nil
}

func validateConditions(c models.Conditions) error {
	if !models.IsValidComparator(c.Comparator) {
		return errorcode.ValidationError{Msg: "conditions.comparator must be above, below, or equal"}
	}
	if c.AlertThreshold == nil {
		return errorcode.ValidationError{Msg: "conditions.alertThreshold is required"}
	}
	if c.NoDataAfterSec < 0 {
		return errorcode.ValidationError{Msg: "conditions.noDataAfterSec must not be negative"}
	}
	if !slices.Contains(models.NoDataResolutions, c.NoDataAs) {
		return errorcode.ValidationError{Msg: "conditions.noDataAs must be no_data, alert, or ok"}
	}
	return nil
}
