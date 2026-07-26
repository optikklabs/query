package repogolden

import (
	"context"
	"fmt"
	"strings"
	"testing"

	tracesrepo "github.com/optikklabs/query/internal/modules/traces/repository"
	"github.com/optikklabs/query/internal/shared/chtest"
)

const (
	testTraceID = "0af7651916cd43dd8448eb211c80319c"
	testSpanID  = "b7ad6b7169203331"
)

// TestTracesRepoSQL pins the SQL of every read in the trace detail domain.
// Nine of its ten reads take the same four bounding binds through
// boundedTraceArgs, which existed in three copies before the merge — a
// regression there would show as a missing bind on several entries at once.
func TestTracesRepoSQL(t *testing.T) {
	ctx := context.Background()
	rec := &chtest.Recorder{}
	var b strings.Builder

	record := func(name string, call func()) {
		rec.Reset()
		call()
		fmt.Fprintf(&b, "=== %s\n%s\n", name, rec.Render())
	}

	repo := tracesrepo.NewRepository(rec)

	record("detail.GetTraceSummary", func() {
		_, _ = repo.GetTraceSummary(ctx, tenantID, testTraceID, startMs, endMs)
	})
	record("detail.ListSpansByTrace", func() {
		_, _ = repo.ListSpansByTrace(ctx, tenantID, testTraceID, startMs, endMs)
	})
	record("detail.GetSpanEvents", func() {
		_, _ = repo.GetSpanEvents(ctx, tenantID, testTraceID, startMs, endMs)
	})
	record("detail.GetSpanAttributes", func() {
		_, _ = repo.GetSpanAttributes(ctx, tenantID, testTraceID, testSpanID, startMs, endMs)
	})
	record("detail.GetRelatedTraces", func() {
		_, _ = repo.GetRelatedTraces(ctx, tenantID, "checkout", "POST /cart", startMs, endMs, testTraceID, 10)
	})

	record("paths.GetCriticalPath", func() {
		_, _ = repo.GetCriticalPath(ctx, tenantID, testTraceID, startMs, endMs)
	})
	record("paths.GetErrorPath", func() {
		_, _ = repo.GetErrorPath(ctx, tenantID, testTraceID, startMs, endMs)
	})

	record("servicemap.GetServiceMapSpans", func() {
		_, _ = repo.GetServiceMapSpans(ctx, tenantID, testTraceID, startMs, endMs)
	})
	record("servicemap.GetTraceErrors", func() {
		_, _ = repo.GetTraceErrors(ctx, tenantID, testTraceID, startMs, endMs)
	})

	compareGolden(t, "traces.golden.txt", b.String())
}
