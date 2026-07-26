package repogolden

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	logsexplorer "github.com/optikklabs/query/internal/modules/logs/explorer"
	logsfacets "github.com/optikklabs/query/internal/modules/logs/facets"
	logsfilter "github.com/optikklabs/query/internal/modules/logs/filter"
	logsdetail "github.com/optikklabs/query/internal/modules/logs/logdetail"
	logsmodels "github.com/optikklabs/query/internal/modules/logs/shared/models"
	logstracelogs "github.com/optikklabs/query/internal/modules/logs/trace_logs"
	logstrends "github.com/optikklabs/query/internal/modules/logs/trends"
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

// TestLogsRepoSQL pins the SQL of every read in the logs domain, which is
// about to be merged into one package. Three of these modules — explorer,
// logdetail and trace_logs — had no test at all before this.
func TestLogsRepoSQL(t *testing.T) {
	ctx := context.Background()
	rec := &chtest.Recorder{}
	var b strings.Builder

	record := func(name string, call func()) {
		rec.Reset()
		call()
		fmt.Fprintf(&b, "=== %s\n%s\n", name, rec.Render())
	}

	// getLogs is unexported, so the list query is reached through the service
	// that owns it — which is also what pins the cursor predicate.
	explSvc := logsexplorer.NewService(logsexplorer.NewRepository(rec))
	record("explorer.Query", func() {
		req := logsexplorer.QueryRequest{StartTime: startMs, EndTime: endMs, Limit: 100}
		req.Filters = logFilters()
		_, _ = explSvc.Query(ctx, req)
	})
	record("explorer.Query/cursor", func() {
		req := logsexplorer.QueryRequest{
			StartTime: startMs, EndTime: endMs, Limit: 100,
			// A cursor the encoder produced, so the keyset predicate is exercised.
			Cursor: encodedCursor(),
		}
		req.Filters = logFilters()
		_, _ = explSvc.Query(ctx, req)
	})

	explRepo := logsexplorer.NewRepository(rec)
	record("explorer.SuggestScalar/service", func() {
		_, _ = explRepo.SuggestScalar(ctx, tenantID, startMs, endMs, "service_name", "ap", 10)
	})
	record("explorer.SuggestScalar/severity", func() {
		_, _ = explRepo.SuggestScalar(ctx, tenantID, startMs, endMs, "severity_text", "er", 10)
	})
	record("explorer.SuggestAttribute", func() {
		_, _ = explRepo.SuggestAttribute(ctx, tenantID, startMs, endMs, "@k", "v", 10)
	})

	facetsRepo := logsfacets.NewRepository(rec)
	record("facets.Compute", func() { _, _ = facetsRepo.Compute(ctx, logFilters()) })

	trendsRepo := logstrends.NewRepository(rec)
	record("trends.Summary", func() { _, _ = trendsRepo.Summary(ctx, logFilters()) })
	record("trends.Trend", func() { _, _ = trendsRepo.Trend(ctx, logFilters()) })

	detailRepo := logsdetail.NewRepository(rec)
	record("logdetail.GetByID", func() {
		_, _ = detailRepo.GetByID(ctx, tenantID, "deadbeefdeadbeef", startMs, endMs)
	})

	traceRepo := logstracelogs.NewRepository(rec)
	record("trace_logs.FetchLogsByTrace", func() {
		_, _ = traceRepo.FetchLogsByTrace(ctx, tenantID, "abc123", 500)
	})

	compareGolden(t, "logs.golden.txt", b.String())
}
