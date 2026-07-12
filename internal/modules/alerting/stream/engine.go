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

// Transition is emitted only when a monitor's externally visible state or
// notification deadline changes. Raw metric ingestion never writes MySQL.
type Transition struct {
	Monitor  models.MonitorRow
	State    models.MonitorStateRow
	Decision expr.Decision
	Value    float64
	At       time.Time
}

// Engine owns bounded, in-process window state. Kafka partitions serialize
// events today; the mutex also makes the domain object safe in tests and
// future batch consumers.
type Engine struct {
	mu       sync.Mutex
	monitors map[int64]models.MonitorRow
	states   map[int64]models.MonitorStateRow
	windows  map[int64][]sample
}

func NewEngine(monitors []models.MonitorRow, states []models.MonitorStateRow) *Engine {
	e := &Engine{monitors: make(map[int64]models.MonitorRow, len(monitors)), states: make(map[int64]models.MonitorStateRow, len(states)), windows: make(map[int64][]sample)}
	for _, m := range monitors {
		e.monitors[m.ID] = m
	}
	for _, s := range states {
		e.states[s.MonitorID] = s
	}
	return e
}

func (e *Engine) OnMetric(event MetricEvent) []Transition {
	e.mu.Lock()
	defer e.mu.Unlock()
	at := time.Unix(0, event.TimestampNs).UTC()
	var out []Transition
	for id, monitor := range e.monitors {
		if !monitor.Active || monitor.Type != "metric" || monitor.TenantID != event.TenantID || monitor.Query.Metric == nil || monitor.Query.Metric.Metric != event.MetricName {
			continue
		}
		windowSeconds := monitor.Query.Metric.WindowSec
		if windowSeconds <= 0 {
			windowSeconds = 300
		}
		cutoff := at.Add(-time.Duration(windowSeconds) * time.Second)
		samples := append(e.windows[id], sample{at: at, value: event.Value})
		// Keep time order even if a producer sends a late sample.
		sort.Slice(samples, func(i, j int) bool { return samples[i].at.Before(samples[j].at) })
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

// Commit records a transition only after its durable projection succeeds. This
// ordering is essential: advancing in-memory state before MySQL succeeds
// would make a redelivered Kafka record look non-transitioning and lose an
// alert after a transient database failure.
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
