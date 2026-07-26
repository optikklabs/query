package repogolden

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	logsfilter "github.com/optikklabs/query/internal/modules/logs/filter"
	logsmodels "github.com/optikklabs/query/internal/modules/logs/models"
	logsrepo "github.com/optikklabs/query/internal/modules/logs/repository"
	logsservice "github.com/optikklabs/query/internal/modules/logs/service"
	"github.com/optikklabs/query/internal/shared/chtest"
)

// encodedCursor is a real encoded keyset cursor, so the pagination predicate
// and its nanosecond-scale bind are exercised rather than skipped.
func encodedCursor() string {
	return logsmodels.Cursor{
		Timestamp: time.Date(2026, 1, 2, 3, 4, 5, 123456789, time.UTC),
		LogID:     "deadbeefdeadbeef",
	}.Encode()
}

// logFilters exercises resource, severity, correlation, and attribute
// predicates together, so a dropped filter shows as a missing bind.
func logFilters() logsfilter.Filters {
	return logsfilter.Filters{
		TenantID:   tenantID,
		StartMs:    startMs,
		EndMs:      endMs,
		Services:   []string{"api"},
		Severities: []string{"error"},
		TraceID:    "abc123",
		Search:     "timeout",
		Attributes: []logsfilter.AttrFilter{{Key: "k", Op: "eq", Value: "v"}},
	}
}

// TestLogsRepoSQL pins the SQL of every read in the logs domain. Three of the
// modules it was merged from — explorer, logdetail and trace_logs — had no
// test at all before this.
func TestLogsRepoSQL(t *testing.T) {
	ctx := context.Background()
	rec := &chtest.Recorder{}
	var b strings.Builder

	record := func(name string, call func()) {
		rec.Reset()
		call()
		fmt.Fprintf(&b, "=== %s\n%s\n", name, rec.Render())
	}

	repo := logsrepo.NewRepository(rec)

	// The list query is reached through the service, which is also what pins
	// the cursor predicate.
	svc := logsservice.NewService(repo)
	record("explorer.Query", func() {
		req := logsmodels.QueryRequest{StartTime: startMs, EndTime: endMs, Limit: 100}
		req.Filters = logFilters()
		_, _ = svc.Query(ctx, req)
	})
	record("explorer.Query/cursor", func() {
		req := logsmodels.QueryRequest{
			StartTime: startMs, EndTime: endMs, Limit: 100,
			// A cursor the encoder produced, so the keyset predicate is exercised.
			Cursor: encodedCursor(),
		}
		req.Filters = logFilters()
		_, _ = svc.Query(ctx, req)
	})

	record("explorer.SuggestScalar/service", func() {
		_, _ = repo.SuggestScalar(ctx, tenantID, startMs, endMs, "service_name", "ap", 10)
	})
	record("explorer.SuggestScalar/severity", func() {
		_, _ = repo.SuggestScalar(ctx, tenantID, startMs, endMs, "severity_text", "er", 10)
	})
	record("explorer.SuggestAttribute", func() {
		_, _ = repo.SuggestAttribute(ctx, tenantID, startMs, endMs, "@k", "v", 10)
	})

	record("facets.Compute", func() { _, _ = repo.Facets(ctx, logFilters()) })

	record("trends.Summary", func() { _, _ = repo.Summary(ctx, logFilters()) })
	record("trends.Trend", func() { _, _ = repo.Trend(ctx, logFilters()) })

	record("logdetail.GetByID", func() {
		_, _ = repo.GetByID(ctx, tenantID, "deadbeefdeadbeef", startMs, endMs)
	})

	record("trace_logs.FetchLogsByTrace", func() {
		_, _ = repo.FetchLogsByTrace(ctx, tenantID, "abc123", 500)
	})

	compareGolden(t, "logs.golden.txt", b.String())
}
