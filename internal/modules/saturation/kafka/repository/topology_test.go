package repository

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/optikklabs/query/internal/shared/chtest"
)

func TestQueriesDoNotSortByTraffic(t *testing.T) {
	query := clientsQuery(0)
	if strings.Contains(query, "count()") {
		t.Fatal("clients query must not rank services by series count")
	}
	if !strings.Contains(query, "SELECT DISTINCT service") || !strings.Contains(query, "ORDER BY service") {
		t.Fatal("clients query must return distinct services in deterministic name order")
	}
	if strings.Contains(edgesQuery("rollup"), "ORDER BY call_count") {
		t.Fatal("edges query must not sort aggregated rows by call count")
	}
}

func TestQueryClientsUsesWindowRollup(t *testing.T) {
	const tenantID int64 = 7
	start := time.Date(2026, time.January, 2, 2, 6, 40, 0, time.UTC)

	tests := []struct {
		name      string
		window    time.Duration
		wantTable string
		wantStart time.Time
	}{
		{
			name:      "thirty minutes",
			window:    30 * time.Minute,
			wantTable: "optikk.span_stats_1m",
			wantStart: start,
		},
		{
			name:      "six hours",
			window:    6 * time.Hour,
			wantTable: "optikk.span_stats_5m",
			wantStart: start,
		},
		{
			name:      "thirty days",
			window:    30 * 24 * time.Hour,
			wantTable: "optikk.span_stats_1h",
			wantStart: start.Truncate(time.Hour),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := &chtest.Recorder{}
			repo := NewRepository(rec)
			end := start.Add(tt.window)

			if _, err := repo.QueryClients(
				context.Background(),
				tenantID,
				start.UnixMilli(),
				end.UnixMilli(),
			); err != nil {
				t.Fatalf("QueryClients returned error: %v", err)
			}

			calls := rec.Calls()
			if len(calls) != 1 {
				t.Fatalf("recorded %d calls, want 1", len(calls))
			}
			if !strings.Contains(calls[0].Query, "FROM "+tt.wantTable) {
				t.Errorf("query does not use %s:\n%s", tt.wantTable, calls[0].Query)
			}

			args := chtest.NamedArgs(calls[0].Args)
			gotStart, ok := args["start"].(time.Time)
			if !ok {
				t.Fatalf("@start has type %T, want time.Time", args["start"])
			}
			if !gotStart.Equal(tt.wantStart) {
				t.Errorf("@start = %s, want %s", gotStart, tt.wantStart)
			}
		})
	}
}
