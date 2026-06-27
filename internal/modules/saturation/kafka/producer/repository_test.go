package producer

import (
	"strings"
	"testing"

	"github.com/optikklabs/query/internal/infra/timebucket"
)

const (
	window20m = 20 * 60 * 1000
	window6h  = 6 * 60 * 60 * 1000
	window30d = 30 * 24 * 60 * 60 * 1000
)

// The SQL bucket grain must track DisplayGrainSQL(window): the fold divides by
// DisplayGrain seconds, so a hardcoded grain over/under-counts on other windows
// (the historical 5x bug). Asserting per window also proves it is not hardcoded
// (the three windows resolve to three distinct fragments).
func TestPublishRateByTopicQuery_GrainAlignment(t *testing.T) {
	cases := []struct {
		name   string
		window int64
		want   string
	}{
		{"20m->1m", window20m, "toStartOfMinute(timestamp)"},
		{"6h->5m", window6h, "toStartOfFiveMinutes(timestamp)"},
		{"30d->1d", window30d, "toStartOfDay(timestamp)"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.want != timebucket.DisplayGrainSQL(c.window) {
				t.Fatalf("test setup: window %d resolves to %q not %q", c.window, timebucket.DisplayGrainSQL(c.window), c.want)
			}
			q := publishRateByTopicQuery(c.window)
			if !strings.Contains(q, c.want) {
				t.Errorf("query for window %d missing grain %q:\n%s", c.window, c.want, q)
			}
		})
	}
}

func TestPublishRateByTopicQuery_DeltaSum(t *testing.T) {
	q := publishRateByTopicQuery(window20m)
	if !strings.Contains(q, "sum(m.value)") {
		t.Errorf("rate query must sum the delta counter, got:\n%s", q)
	}
	for _, bad := range []string{"max(m.value)", "min(m.value)"} {
		if strings.Contains(q, bad) {
			t.Errorf("rate query must not use cumulative %q:\n%s", bad, q)
		}
	}
}

func TestPublishRateByTopicQuery_DimsAndTables(t *testing.T) {
	q := publishRateByTopicQuery(window20m)
	for _, want := range []string{
		"FROM optikk.metrics_series",
		"FROM optikk.metrics AS m",
		"`messaging.destination.name`",
		"`messaging.system`",
		"= 'kafka'",
		"GROUP BY timestamp, topic",
	} {
		if !strings.Contains(q, want) {
			t.Errorf("query missing %q:\n%s", want, q)
		}
	}
}

func TestProduceCounterMetrics(t *testing.T) {
	want := []string{"kafka.producer.record_send_total"}
	if len(produceCounterMetrics) != len(want) || produceCounterMetrics[0] != want[0] {
		t.Errorf("produceCounterMetrics = %v, want %v", produceCounterMetrics, want)
	}
}
