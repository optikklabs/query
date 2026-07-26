package repository

import (
	"strings"
	"testing"
)

func TestActiveConnectionsQueryUsesLatestValuePerSeries(t *testing.T) {
	query := activeConnectionsQuery("optikk.metrics_5m")
	for _, want := range []string{
		"FROM optikk.metrics_5m",
		"argMaxMerge(m.val_last)",
		"GROUP BY db_system, fingerprint",
		"sum(latest_value)",
	} {
		if !strings.Contains(query, want) {
			t.Errorf("query missing %q:\n%s", want, query)
		}
	}
	if strings.Contains(query, "sum(val_sum) / sum(val_count)") {
		t.Errorf("query still averages connection gauges:\n%s", query)
	}
}

func TestClampPatternLimit(t *testing.T) {
	tests := []struct {
		name  string
		input int
		want  int
	}{
		{name: "default", input: 0, want: DefaultPatternLimit},
		{name: "negative", input: -1, want: DefaultPatternLimit},
		{name: "accepted", input: 50, want: 50},
		{name: "clamped", input: 1000, want: maxPatternLimit},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := clampPatternLimit(tt.input); got != tt.want {
				t.Errorf("clampPatternLimit(%d) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestSlowQueryPatternsQueryPreservesScope(t *testing.T) {
	query := slowQueryPatternsQuery(" AND db_system IN @dbSystem")
	for _, want := range []string{
		"db_system",
		"db_name",
		"timestamp BETWEEN @start AND @end",
		"query_hash != ''",
		"GROUP BY query_hash, db_system, collection_name",
		"AND db_system IN @dbSystem",
	} {
		if !strings.Contains(query, want) {
			t.Errorf("query missing %q:\n%s", want, query)
		}
	}
	if strings.Contains(query, "attributes.") {
		t.Errorf("hot query must not read dynamic JSON attributes:\n%s", query)
	}
	if strings.Contains(query, "db_statement != ''") {
		t.Errorf("hot query must not read db_statement just to test for a fingerprint:\n%s", query)
	}
}

func TestClampExecutionsLimit(t *testing.T) {
	tests := []struct {
		input int
		want  int
	}{
		{input: 0, want: DefaultExecutionsLimit},
		{input: -1, want: DefaultExecutionsLimit},
		{input: 100, want: 100},
		{input: 1000, want: maxExecutionsLimit},
	}
	for _, tt := range tests {
		if got := clampExecutionsLimit(tt.input); got != tt.want {
			t.Errorf("clampExecutionsLimit(%d) = %d, want %d", tt.input, got, tt.want)
		}
	}
}
