package monitors

import (
	"context"
	"database/sql"
	"time"

	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/modules/alerting/shared/expr"
	models "github.com/optikklabs/query/internal/modules/alerting/shared/models"
	"github.com/optikklabs/query/internal/modules/alerting/shared/query"
	"github.com/optikklabs/query/internal/shared/errorcode"
	"github.com/optikklabs/query/internal/shared/filterutil"
)

var ErrNotAlerting = errorcode.ConflictError{Msg: "monitor is not currently alerting"}

func (s *Service) Ack(ctx context.Context, tenantID, userID, id int64) error {
	return dbutil.NoRowsAs(s.repo.Ack(ctx, id, tenantID, userID, time.Now().UTC()), ErrNotAlerting)
}

// Mute silences notifications for durationSec.
func (s *Service) Mute(ctx context.Context, tenantID, id int64, durationSec int) error {
	if durationSec <= 0 {
		return errorcode.ValidationError{Msg: "durationSec must be positive"}
	}
	until := sql.NullTime{Valid: true, Time: time.Now().UTC().Add(time.Duration(durationSec) * time.Second)}
	return dbutil.NoRowsAs(s.repo.Mute(ctx, id, tenantID, until), ErrNotFound)
}

func (s *Service) Unmute(ctx context.Context, tenantID, id int64) error {
	return dbutil.NoRowsAs(s.repo.Mute(ctx, id, tenantID, sql.NullTime{}), ErrNotFound)
}

// validWindow checks a trailing chart window in milliseconds.
func validWindow(windowMs int64) error {
	if windowMs <= 0 || windowMs > filterutil.MaxTimeRangeMs {
		return errorcode.ValidationError{Msg: "windowMs must be positive and at most 30 days"}
	}
	return nil
}

type TestResult struct {
	Value         float64 `json:"value"`
	HasData       bool    `json:"hasData"`
	WouldDecideAs string  `json:"wouldDecideAs"`
	Threshold     float64 `json:"threshold"`
}

// Test evaluates the monitor now and reports the status it would move to,
// without persisting anything.
func (s *Service) Test(ctx context.Context, tenantID, id int64, queries query.Registry) (TestResult, error) {
	row, state, err := s.repo.GetByID(ctx, id, tenantID)
	if err != nil {
		return TestResult{}, dbutil.NoRowsAs(err, ErrNotFound)
	}
	backend, err := queries.For(row.Type)
	if err != nil {
		return TestResult{}, err
	}
	now := time.Now().UTC()
	res, err := backend.Scalar(ctx, row, now)
	if err != nil {
		return TestResult{}, err
	}
	d := expr.Decide(state, row.Conditions, res.Value, res.HasData, row.RenotifyEverySec.Int64, now)
	return TestResult{
		Value:         res.Value,
		HasData:       res.HasData,
		WouldDecideAs: d.NewStatus,
		Threshold:     *row.Conditions.AlertThreshold,
	}, nil
}

type SeriesResponse struct {
	Points            []query.Point `json:"points"`
	AlertThreshold    *float64      `json:"alertThreshold,omitempty"`
	WarnThreshold     *float64      `json:"warnThreshold,omitempty"`
	RecoveryThreshold *float64      `json:"recoveryThreshold,omitempty"`
}

func (s *Service) Series(ctx context.Context, tenantID, id int64, queries query.Registry, windowMs int64) (SeriesResponse, error) {
	if err := validWindow(windowMs); err != nil {
		return SeriesResponse{}, err
	}
	row, _, err := s.repo.GetByID(ctx, id, tenantID)
	if err != nil {
		return SeriesResponse{}, dbutil.NoRowsAs(err, ErrNotFound)
	}
	backend, err := queries.For(row.Type)
	if err != nil {
		return SeriesResponse{}, err
	}
	points, err := backend.Series(ctx, row, windowMs, time.Now().UTC())
	if err != nil {
		return SeriesResponse{}, err
	}
	cond := row.Conditions
	return SeriesResponse{
		Points:            points,
		AlertThreshold:    cond.AlertThreshold,
		WarnThreshold:     cond.WarnThreshold,
		RecoveryThreshold: cond.RecoveryThreshold,
	}, nil
}

func (s *Service) Events(ctx context.Context, tenantID, id int64, limit int) ([]MonitorEventResponse, error) {
	rows, err := s.repo.Events(ctx, id, tenantID, limit)
	if err != nil {
		return nil, err
	}
	return toEventResponses(rows), nil
}

// Activity lists events since sinceMs; 0 means the last hour.
func (s *Service) Activity(ctx context.Context, tenantID int64, sinceMs int64, limit int) ([]MonitorEventResponse, error) {
	if sinceMs < 0 {
		return nil, errorcode.ValidationError{Msg: "since must not be negative"}
	}
	since := time.Now().UTC().Add(-time.Hour)
	if sinceMs > 0 {
		since = time.UnixMilli(sinceMs).UTC()
	}
	rows, err := s.repo.Activity(ctx, tenantID, since, limit)
	if err != nil {
		return nil, err
	}
	return toEventResponses(rows), nil
}

type StatusTimelineResponse struct {
	Bands     []StatusBand `json:"bands"`
	StartedAt time.Time    `json:"startedAt"`
	EndedAt   time.Time    `json:"endedAt"`
}

type StatusBand struct {
	Status    string    `json:"status"`
	StartedAt time.Time `json:"startedAt"`
	EndedAt   time.Time `json:"endedAt"`
}

// StatusTimeline folds the monitor's events over the trailing window into
// contiguous status bands. The window opens in the status the last earlier
// event left the monitor in.
func (s *Service) StatusTimeline(ctx context.Context, tenantID, id int64, windowMs int64) (StatusTimelineResponse, error) {
	if err := validWindow(windowMs); err != nil {
		return StatusTimelineResponse{}, err
	}
	now := time.Now().UTC()
	since := now.Add(-time.Duration(windowMs) * time.Millisecond)
	prevKind, err := s.repo.LastEventKindBefore(ctx, id, tenantID, since)
	if err != nil {
		return StatusTimelineResponse{}, err
	}
	rows, err := s.repo.StatusTimelineRows(ctx, id, tenantID, since)
	if err != nil {
		return StatusTimelineResponse{}, err
	}
	return StatusTimelineResponse{
		Bands:     buildBands(eventStatus(prevKind), rows, since, now),
		StartedAt: since,
		EndedAt:   now,
	}, nil
}

// eventStatus is the band status an event leaves the monitor in; "" (no
// event yet) reads as ok.
func eventStatus(kind string) string {
	if kind == models.EventTriggered {
		return models.StatusAlert
	}
	return models.StatusOK
}

func buildBands(initial string, events []models.MonitorEventRow, start, end time.Time) []StatusBand {
	bands := make([]StatusBand, 0, len(events)+1)
	cursor := start
	current := initial
	for _, e := range events {
		if e.StartedAt.Before(cursor) {
			continue
		}
		if e.StartedAt.After(cursor) {
			bands = append(bands, StatusBand{Status: current, StartedAt: cursor, EndedAt: e.StartedAt})
		}
		current = eventStatus(e.Kind)
		cursor = e.StartedAt
	}
	if cursor.Before(end) {
		bands = append(bands, StatusBand{Status: current, StartedAt: cursor, EndedAt: end})
	}
	return bands
}
