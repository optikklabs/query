package stream

import (
	"database/sql"
	"sort"
	"sync"
	"time"

	"github.com/optikklabs/query/internal/modules/alerting/shared/expr"
	models "github.com/optikklabs/query/internal/modules/alerting/shared/models"
)

type sample struct {
	at    time.Time
	value float64
}

type Transition struct {
	Monitor  models.MonitorRow
	State    models.MonitorStateRow
	Decision expr.Decision
	Value    float64
	At       time.Time
}

type metricKey struct {
	tenantID   int64
	metricName string
}

type Engine struct {
	mu          sync.Mutex
	monitors    map[int64]models.MonitorRow
	states      map[int64]models.MonitorStateRow
	windows     map[int64][]sample
	metricIndex map[metricKey][]int64
}

func NewEngine(monitors []models.MonitorRow, states []models.MonitorStateRow) *Engine {
	e := &Engine{
		monitors:    make(map[int64]models.MonitorRow, len(monitors)),
		states:      make(map[int64]models.MonitorStateRow, len(states)),
		windows:     make(map[int64][]sample),
		metricIndex: make(map[metricKey][]int64),
	}
	for _, m := range monitors {
		e.monitors[m.ID] = m
		if m.Active && m.Type == "metric" && m.Query.Metric != nil && m.Query.Metric.Metric != "" {
			key := metricKey{tenantID: m.TenantID, metricName: m.Query.Metric.Metric}
			e.metricIndex[key] = append(e.metricIndex[key], m.ID)
		}
	}
	for _, s := range states {
		e.states[s.MonitorID] = s
	}
	return e
}

func (e *Engine) OnMetric(event MetricEvent) []Transition {
	e.mu.Lock()
	defer e.mu.Unlock()
	key := metricKey{tenantID: event.TenantID, metricName: event.MetricName}
	matchingIDs := e.metricIndex[key]
	if len(matchingIDs) == 0 {
		return nil
	}

	at := time.Unix(0, event.TimestampNs).UTC()
	var out []Transition
	for _, id := range matchingIDs {
		monitor, exists := e.monitors[id]
		if !exists || !monitor.Active || monitor.Type != "metric" || monitor.Query.Metric == nil {
			continue
		}
		windowSeconds := monitor.Query.Metric.WindowSec
		if windowSeconds <= 0 {
			windowSeconds = 300
		}
		cutoff := at.Add(-time.Duration(windowSeconds) * time.Second)
		samples := append(e.windows[id], sample{at: at, value: event.Value})

		if len(samples) > 1 && samples[len(samples)-1].at.Before(samples[len(samples)-2].at) {
			sort.Slice(samples, func(i, j int) bool { return samples[i].at.Before(samples[j].at) })
		}
		first := sort.Search(len(samples), func(i int) bool { return !samples[i].at.Before(cutoff) })
		samples = samples[first:]
		e.windows[id] = samples
		if monitor.Conditions.MinSample != nil && len(samples) < *monitor.Conditions.MinSample {
			continue
		}
		value := aggregate(monitor.Query.Metric.Aggregation, samples)
		state := e.states[id]
		if state.Status == "" {
			state.Status = "no_data"
		}
		renotify := int64(0)
		if monitor.RenotifyEverySec.Valid {
			renotify = monitor.RenotifyEverySec.Int64
		}
		decision := expr.Decide(state, monitor, monitor.Conditions, value, true, renotify, at)
		if !decision.Transition && !decision.ShouldNotify {
			continue
		}
		state.Status = decision.NewStatus
		state.CurrentValue = sql.NullFloat64{Valid: true, Float64: value}
		state.LastEvaluatedAt = sql.NullTime{Valid: true, Time: at}
		if decision.NewStatus == "alert" || decision.NewStatus == "warn" {
			if !state.TriggeredAt.Valid {
				state.TriggeredAt = sql.NullTime{Valid: true, Time: at}
			}
		} else {
			state.TriggeredAt = sql.NullTime{}
		}
		if decision.ShouldNotify {
			state.LastNotifiedAt = sql.NullTime{Valid: true, Time: at}
		}
		out = append(out, Transition{Monitor: monitor, State: state, Decision: decision, Value: value, At: at})
	}
	return out
}

func (e *Engine) Commit(t Transition) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.states[t.Monitor.ID] = t.State
}

func aggregate(kind string, samples []sample) float64 {
	if len(samples) == 0 {
		return 0
	}
	v := samples[0].value
	sum := 0.0
	for _, s := range samples {
		sum += s.value
		if kind == "min" && s.value < v {
			v = s.value
		}
		if kind == "max" && s.value > v {
			v = s.value
		}
	}
	switch kind {
	case "sum":
		return sum
	case "min", "max":
		return v
	default:
		return sum / float64(len(samples))
	}
}
