package monitors

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	models "github.com/optikklabs/query/internal/modules/alerting/shared/models"
)

// Service owns CRUD validation and JSON marshaling for monitors.
type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

var ErrNotFound = errors.New("monitor not found")

type ErrValidation struct{ Msg string }

func (e ErrValidation) Error() string { return e.Msg }

func (s *Service) Create(ctx context.Context, tenantID, userID int64, req CreateMonitorRequest) (MonitorResponse, error) {
	args, err := buildInsertArgs(tenantID, userID, req)
	if err != nil {
		return MonitorResponse{}, err
	}
	id, err := s.repo.Create(ctx, args)
	if err != nil {
		return MonitorResponse{}, err
	}
	return s.GetByID(ctx, tenantID, id)
}

func (s *Service) Update(ctx context.Context, tenantID, userID, id int64, req UpdateMonitorRequest) (MonitorResponse, error) {
	args, err := buildInsertArgs(tenantID, userID, req)
	if err != nil {
		return MonitorResponse{}, err
	}
	if err := s.repo.Update(ctx, id, tenantID, args); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return MonitorResponse{}, ErrNotFound
		}
		return MonitorResponse{}, err
	}
	return s.GetByID(ctx, tenantID, id)
}

func (s *Service) Delete(ctx context.Context, tenantID, id int64) error {
	if err := s.repo.Delete(ctx, id, tenantID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	return nil
}

func (s *Service) GetByID(ctx context.Context, tenantID, id int64) (MonitorResponse, error) {
	row, state, err := s.repo.GetByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return MonitorResponse{}, ErrNotFound
		}
		return MonitorResponse{}, err
	}
	return toResponse(row, state), nil
}

func (s *Service) List(ctx context.Context, tenantID int64, q ListQuery) (MonitorListResponse, error) {
	rows, states, err := s.repo.List(ctx, tenantID, q)
	if err != nil {
		return MonitorListResponse{}, err
	}
	items := make([]MonitorResponse, 0, len(rows))
	counts := StatusCounts{Total: len(rows)}
	for i, row := range rows {
		state := states[i]
		items = append(items, toResponse(row, state))
		switch state.Status {
		case "alert":
			counts.Alert++
		case "warn":
			counts.Warn++
		case "ok":
			counts.OK++
		case "no_data", "":
			counts.NoData++
		}
		if row.MutedUntil.Valid {
			counts.Muted++
		}
	}
	return MonitorListResponse{Items: items, Counts: counts}, nil
}

func buildInsertArgs(tenantID, userID int64, req CreateMonitorRequest) (insertArgs, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return insertArgs{}, ErrValidation{Msg: "name is required"}
	}
	if !models.IsValidType(req.Type) {
		return insertArgs{}, ErrValidation{Msg: fmt.Sprintf("type must be one of %v", models.SupportedMonitorTypes)}
	}
	priority := req.Priority
	if priority == "" {
		priority = "P2"
	}
	if !models.IsValidPriority(priority) {
		return insertArgs{}, ErrValidation{Msg: fmt.Sprintf("priority must be one of %v", models.SupportedPriorities)}
	}
	if err := validateQueryForType(req.Type, req.Query); err != nil {
		return insertArgs{}, err
	}
	if err := validateConditions(req.Conditions); err != nil {
		return insertArgs{}, err
	}
	evalEvery := req.EvalEverySec
	if evalEvery <= 0 {
		evalEvery = 300
	}
	scopeJSON, _ := json.Marshal(req.Scope)
	queryJSON, _ := json.Marshal(req.Query)
	condJSON, _ := json.Marshal(req.Conditions)
	notifyJSON, _ := json.Marshal(req.Notify)
	tagsJSON, _ := json.Marshal(req.Tags)
	if len(req.Tags) == 0 {
		tagsJSON = []byte("[]")
	}

	args := insertArgs{
		TenantID:       tenantID,
		Name:           name,
		Type:           req.Type,
		Priority:       priority,
		ScopeJSON:      scopeJSON,
		QueryJSON:      queryJSON,
		ConditionsJSON: condJSON,
		NotifyJSON:     notifyJSON,
		TagsJSON:       tagsJSON,
		EvalEverySec:   evalEvery,
	}
	if msg := strings.TrimSpace(req.MessageBody); msg != "" {
		args.MessageBody = sql.NullString{Valid: true, String: msg}
	}
	if url := strings.TrimSpace(req.RunbookURL); url != "" {
		args.RunbookURL = sql.NullString{Valid: true, String: url}
	}
	if req.RenotifyEverySec != nil && *req.RenotifyEverySec > 0 {
		args.RenotifyEverySec = sql.NullInt64{Valid: true, Int64: int64(*req.RenotifyEverySec)}
	}
	if userID > 0 {
		args.CreatedByUserID = sql.NullInt64{Valid: true, Int64: userID}
	}
	return args, nil
}

func validateQueryForType(t string, q models.MonitorQuery) error {
	switch t {
	case "metric":
		if q.Metric == nil || strings.TrimSpace(q.Metric.Metric) == "" {
			return ErrValidation{Msg: "metric query requires query.metric.metric"}
		}
	case "apm":
		if q.APM == nil || strings.TrimSpace(q.APM.Service) == "" {
			return ErrValidation{Msg: "apm query requires query.apm.service"}
		}
		if q.APM.Track == "" {
			return ErrValidation{Msg: "apm query requires query.apm.track"}
		}
	case "log":
		if q.Log == nil || strings.TrimSpace(q.Log.Query) == "" {
			return ErrValidation{Msg: "log query requires query.log.query"}
		}
	}
	return nil
}

func validateConditions(c models.Conditions) error {
	switch c.Comparator {
	case "above", "below", "equal":
	case "":
		return ErrValidation{Msg: "conditions.comparator is required"}
	default:
		return ErrValidation{Msg: "conditions.comparator must be above, below, or equal"}
	}
	if c.AlertThreshold == nil {
		return ErrValidation{Msg: "conditions.alertThreshold is required"}
	}
	switch c.NoDataAs {
	case "no_data", "alert", "ok":
	case "":

	default:
		return ErrValidation{Msg: "conditions.noDataAs must be no_data, alert, or ok"}
	}
	return nil
}
