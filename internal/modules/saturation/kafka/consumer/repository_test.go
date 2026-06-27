package consumer

import (
	"strings"
	"testing"

	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/modules/saturation/kafka/filter"
)

const (
	window20m = 20 * 60 * 1000
	window6h  = 6 * 60 * 60 * 1000
	window30d = 30 * 24 * 60 * 60 * 1000
)

var grainCases = []struct {
	name   string
	window int64
	want   string
}{
	{"20m->1m", window20m, "toStartOfMinute(timestamp)"},
	{"6h->5m", window6h, "toStartOfFiveMinutes(timestamp)"},
	{"30d->1d", window30d, "toStartOfDay(timestamp)"},
}

// Consume rate SQL bucket grain must track DisplayGrainSQL(window); a hardcoded
// grain over/under-counts because the fold divides by DisplayGrain seconds.
func TestCounterSeriesByTopicQuery_GrainAlignment(t *testing.T) {
	for _, c := range grainCases {
		t.Run(c.name, func(t *testing.T) {
			if c.want != timebucket.DisplayGrainSQL(c.window) {
				t.Fatalf("test setup: window %d resolves to %q not %q", c.window, timebucket.DisplayGrainSQL(c.window), c.want)
			}
			if q := counterSeriesByTopicQuery(c.window); !strings.Contains(q, c.want) {
				t.Errorf("query for window %d missing grain %q:\n%s", c.window, c.want, q)
			}
		})
	}
}

func TestCounterSeriesByTopicQuery_DeltaSum(t *testing.T) {
	q := counterSeriesByTopicQuery(window20m)
	if !strings.Contains(q, "sum(m.value)") {
		t.Errorf("rate query must sum the delta counter, got:\n%s", q)
	}
	for _, bad := range []string{"max(m.value)", "min(m.value)"} {
		if strings.Contains(q, bad) {
			t.Errorf("rate query must not use cumulative %q:\n%s", bad, q)
		}
	}
}

func TestCounterSeriesByTopicQuery_DimsAndTables(t *testing.T) {
	q := counterSeriesByTopicQuery(window20m)
	for _, want := range []string{
		"FROM optikk.metrics_series",
		"FROM optikk.metrics AS m",
		"`messaging.destination.name`",
		"= 'kafka'",
		"GROUP BY timestamp, topic",
	} {
		if !strings.Contains(q, want) {
			t.Errorf("query missing %q:\n%s", want, q)
		}
	}
}

func TestConsumerLagByGroupTopicQuery_GrainAlignment(t *testing.T) {
	for _, c := range grainCases {
		t.Run(c.name, func(t *testing.T) {
			if q := consumerLagByGroupTopicQuery(c.window); !strings.Contains(q, c.want) {
				t.Errorf("lag query for window %d missing grain %q:\n%s", c.window, c.want, q)
			}
		})
	}
}

func TestConsumerLagByGroupTopicQuery_Avg(t *testing.T) {
	q := consumerLagByGroupTopicQuery(window20m)
	if !strings.Contains(q, "avg(m.value)") {
		t.Errorf("lag query must average the gauge, got:\n%s", q)
	}
	if strings.Contains(q, "sum(m.value)") {
		t.Errorf("lag query must not sum the gauge:\n%s", q)
	}
}

func TestConsumerLagByGroupTopicQuery_Dims(t *testing.T) {
	q := consumerLagByGroupTopicQuery(window20m)
	for _, want := range []string{
		"`messaging.consumer.group.name`",
		"`messaging.destination.name`",
		"GROUP BY timestamp, consumer_group, topic",
	} {
		if !strings.Contains(q, want) {
			t.Errorf("lag query missing %q:\n%s", want, q)
		}
	}
}

func TestConsumeCounterMetrics(t *testing.T) {
	want := []string{"kafka.consumer.records_consumed_total"}
	if len(consumeCounterMetrics) != len(want) || consumeCounterMetrics[0] != want[0] {
		t.Errorf("consumeCounterMetrics = %v, want %v", consumeCounterMetrics, want)
	}
}

func TestConsumerLagMetrics_IncludesClientLag(t *testing.T) {
	const want = "kafka.consumer.records_lag_max"
	for _, m := range filter.ConsumerLagMetrics {
		if m == want {
			return
		}
	}
	t.Errorf("filter.ConsumerLagMetrics %v missing %q", filter.ConsumerLagMetrics, want)
}
