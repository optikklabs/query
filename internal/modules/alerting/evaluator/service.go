package evaluator

import (
	"context"
	"database/sql"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"

	"github.com/optikklabs/query/internal/infra/metrics"
	"github.com/optikklabs/query/internal/modules/alerting/dispatch"
	"github.com/optikklabs/query/internal/modules/alerting/shared/expr"
	models "github.com/optikklabs/query/internal/modules/alerting/shared/models"
	"github.com/optikklabs/query/internal/modules/alerting/shared/query"
	tmpl "github.com/optikklabs/query/internal/modules/alerting/shared/template"
)

const (
	// claimBatch caps how many due monitors one tick claims.
	claimBatch = 500
	// evalConcurrency caps how many monitors one tick evaluates at once.
	evalConcurrency = 16
)

type Service struct {
	repo       *Repository
	queries    query.Registry
	dispatcher *dispatch.Dispatcher
}

func NewService(repo *Repository, queries query.Registry, dispatcher *dispatch.Dispatcher) *Service {
	return &Service{repo: repo, queries: queries, dispatcher: dispatcher}
}

// Tick claims the monitors due at now and evaluates them concurrently.
func (s *Service) Tick(ctx context.Context, now time.Time) error {
	due, err := s.repo.ClaimDue(ctx, uuid.NewString(), now, claimBatch)
	if err != nil {
		return err
	}
	var g errgroup.Group
	g.SetLimit(evalConcurrency)
	for _, m := range due {
		g.Go(func() error {
			s.evalOne(ctx, m, now)
			return nil
		})
	}
	return g.Wait()
}

func (s *Service) evalOne(ctx context.Context, due DueMonitor, now time.Time) {
	m, state := due.Monitor, due.State
	backend, err := s.queries.For(m.Type)
	if err != nil {
		slog.WarnContext(ctx, "alerting: no query backend for type", slog.String("type", m.Type), slog.Int64("monitor_id", m.ID))
		return
	}
	res, err := backend.Scalar(ctx, m, now)
	if err != nil {
		slog.WarnContext(ctx, "alerting: scalar eval failed", slog.Int64("monitor_id", m.ID), slog.Any("error", err))
		s.updateState(ctx, m, rescheduleOnly(m, state, now))
		return
	}

	d := expr.Decide(state, m.Conditions, res.Value, res.HasData, m.RenotifyEverySec.Int64, now)
	if !s.updateState(ctx, m, buildUpdateArgs(m, state, d, res, now)) {
		return
	}

	if d.Transition && (d.NewStatus == "alert" || d.NewStatus == "warn") {
		s.recordEvent(ctx, m, "triggered", res, now)
	}
	if d.IsRecovery {
		s.recordEvent(ctx, m, "recovered", res, now)
	}
	if d.ShouldNotify && !isMuted(m, now) {
		s.dispatchAll(ctx, m, buildPayload(m, statusOrNoData(state), res, d), now)
	}
}

func (s *Service) updateState(ctx context.Context, m models.MonitorRow, args UpdateStateArgs) bool {
	if err := s.repo.UpdateState(ctx, args); err != nil {
		slog.WarnContext(ctx, "alerting: update state failed", slog.Int64("monitor_id", m.ID), slog.Any("error", err))
		return false
	}
	return true
}

func statusOrNoData(state models.MonitorStateRow) string {
	if state.Status == "" {
		return "no_data"
	}
	return state.Status
}

func nextEvaluation(m models.MonitorRow, now time.Time) time.Time {
	return now.Add(time.Duration(m.EvalEverySec) * time.Second)
}

// rescheduleOnly keeps the monitor's state as it is and only schedules the
// next evaluation, for ticks where the query itself failed.
func rescheduleOnly(m models.MonitorRow, state models.MonitorStateRow, now time.Time) UpdateStateArgs {
	status := statusOrNoData(state)
	return UpdateStateArgs{
		MonitorID:          m.ID,
		PrevStatus:         status,
		NewStatus:          status,
		CurrentValue:       state.CurrentValue,
		LastEvaluatedAt:    now,
		NextEvaluationAt:   nextEvaluation(m, now),
		TriggeredAt:        state.TriggeredAt,
		NoDataSince:        state.NoDataSince,
		IncrementEvalCount: true,
	}
}

func buildUpdateArgs(m models.MonitorRow, state models.MonitorStateRow, d expr.Decision, res query.ScalarResult, now time.Time) UpdateStateArgs {
	args := UpdateStateArgs{
		MonitorID:          m.ID,
		PrevStatus:         statusOrNoData(state),
		NewStatus:          d.NewStatus,
		CurrentValue:       sql.NullFloat64{Valid: res.HasData, Float64: res.Value},
		LastEvaluatedAt:    now,
		NextEvaluationAt:   nextEvaluation(m, now),
		NoDataSince:        d.NoDataSince,
		IncrementEvalCount: true,
	}
	if d.NewStatus == "alert" || d.NewStatus == "warn" {
		args.TriggeredAt = state.TriggeredAt
		if !args.TriggeredAt.Valid {
			args.TriggeredAt = sql.NullTime{Valid: true, Time: now}
		}
	}
	if d.ShouldNotify {
		args.LastNotifiedAt = sql.NullTime{Valid: true, Time: now}
	}
	return args
}

func (s *Service) recordEvent(ctx context.Context, m models.MonitorRow, kind string, res query.ScalarResult, now time.Time) {
	threshold, hasThreshold := m.Conditions.PrimaryThreshold()
	err := s.repo.InsertEvent(ctx, models.MonitorEventRow{
		MonitorID: m.ID, TenantID: m.TenantID, Kind: kind,
		Value:     sql.NullFloat64{Valid: true, Float64: res.Value},
		Threshold: sql.NullFloat64{Valid: hasThreshold, Float64: threshold},
		StartedAt: now,
	})
	if err != nil {
		metrics.AlertingAuditWriteFailures.WithLabelValues("event").Inc()
		slog.WarnContext(ctx, "alerting: insert event failed",
			slog.Int64("monitor_id", m.ID), slog.String("kind", kind), slog.Any("error", err))
	}
}

func isMuted(m models.MonitorRow, now time.Time) bool {
	return m.MutedUntil.Valid && m.MutedUntil.Time.After(now)
}

func (s *Service) dispatchAll(ctx context.Context, m models.MonitorRow, payload dispatch.Payload, now time.Time) {
	if len(m.Notify.ChannelIDs) == 0 {
		return
	}
	channels, err := s.repo.GetChannelsByIDs(ctx, m.TenantID, m.Notify.ChannelIDs)
	if err != nil {
		slog.WarnContext(ctx, "alerting: load channels failed", slog.Int64("monitor_id", m.ID), slog.Any("error", err))
		return
	}
	for _, ch := range channels {
		err := s.dispatcher.Dispatch(ctx, ch, payload)
		errText := sql.NullString{}
		if err != nil {
			errText = sql.NullString{Valid: true, String: err.Error()}
			metrics.AlertingDispatchFailures.WithLabelValues(ch.Type).Inc()
			slog.WarnContext(ctx, "alerting: dispatch failed",
				slog.Int64("monitor_id", m.ID),
				slog.Int64("channel_id", ch.ID),
				slog.String("channel_type", ch.Type),
				slog.Any("error", err))
		}
		if err := s.repo.MarkChannelDelivered(ctx, ch.ID, now, errText); err != nil {
			metrics.AlertingAuditWriteFailures.WithLabelValues("delivery").Inc()
			slog.WarnContext(ctx, "alerting: mark delivered failed",
				slog.Int64("monitor_id", m.ID), slog.Int64("channel_id", ch.ID), slog.Any("error", err))
		}
	}
}

func buildPayload(m models.MonitorRow, prevStatus string, res query.ScalarResult, d expr.Decision) dispatch.Payload {
	threshold, _ := m.Conditions.PrimaryThreshold()
	scopeSummary := summarizeScope(m.Scope)
	return dispatch.Payload{
		MonitorName:  m.Name,
		Priority:     m.Priority,
		Transition:   prevStatus + "->" + d.NewStatus,
		Status:       d.NewStatus,
		Value:        res.Value,
		Threshold:    threshold,
		ScopeSummary: scopeSummary,
		Message:      renderMessageBody(m, res.Value, threshold, scopeSummary, d),
		IsAlert:      d.NewStatus == "alert",
		IsWarning:    d.NewStatus == "warn",
		IsRecovery:   d.IsRecovery,
	}
}

func summarizeScope(scope models.Scope) string {
	parts := make([]string, 0, len(scope.Tags))
	for _, t := range scope.Tags {
		parts = append(parts, t.Key+":"+t.Value)
	}
	return strings.Join(parts, " ")
}

func renderMessageBody(m models.MonitorRow, value, threshold float64, scopeSummary string, d expr.Decision) string {
	body := m.MessageBody.String
	if strings.TrimSpace(body) == "" {
		return defaultMessage(m, value, threshold, d)
	}
	return tmpl.Render(body, tmpl.Vars{
		Values: map[string]string{
			"value":        tmpl.FormatFloat(value),
			"threshold":    tmpl.FormatFloat(threshold),
			"service.name": serviceFromScope(m.Scope),
			"monitor.name": m.Name,
			"scope":        scopeSummary,
		},
		IsAlert:    d.NewStatus == "alert",
		IsWarning:  d.NewStatus == "warn",
		IsRecovery: d.IsRecovery,
	})
}

func defaultMessage(m models.MonitorRow, value, threshold float64, d expr.Decision) string {
	verb := "triggered"
	if d.IsRecovery {
		verb = "recovered"
	}
	return m.Name + " " + verb + " — value " + tmpl.FormatFloat(value) + " vs threshold " + tmpl.FormatFloat(threshold)
}

func serviceFromScope(scope models.Scope) string {
	for _, t := range scope.Tags {
		if t.Key == "service" {
			return t.Value
		}
	}
	return ""
}
