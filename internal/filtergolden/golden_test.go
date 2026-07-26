// Package filtergolden snapshots the exact SQL fragments and bound arguments
// every filter package emits.
//
// It exists to make filter refactors verifiable. The hand-written tests in each
// filter package assert intent ("a service filter reaches the PREWHERE"); this
// asserts the literal output, so any change to a clause, a bind name, or a
// bound value's type shows up as a golden diff.
//
// It lives outside the filter packages because it must import all of them, and
// they all import shared/filterutil.
//
// Regenerate with: go test ./internal/filtergolden -update
package filtergolden

import (
	"flag"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"

	logsfilter "github.com/optikklabs/query/internal/modules/logs/filter"
	metricsfilter "github.com/optikklabs/query/internal/modules/metrics/filter"
	dbfilter "github.com/optikklabs/query/internal/modules/saturation/database/filter"
	tracesfilter "github.com/optikklabs/query/internal/modules/traces/filter"
)

var update = flag.Bool("update", false, "rewrite the golden file")

// Fixed so time-derived arguments (bucket boundaries, time.Time binds) are
// stable across runs: a 2-hour window starting 2026-01-02T02:06:40Z.
const (
	startMs int64 = 1767319600000
	endMs   int64 = startMs + 2*3_600_000
)

func TestFilterSQLGolden(t *testing.T) {
	var b strings.Builder
	for _, c := range cases() {
		fmt.Fprintf(&b, "=== %s\n%s\n\n", c.name, c.body)
	}
	got := b.String()

	path := filepath.Join("testdata", "filters.golden.txt")
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Log("golden updated")
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden (run with -update to create): %v", err)
	}
	if got != string(want) {
		t.Errorf("filter SQL changed.\n--- want\n%s\n--- got\n%s", want, got)
	}
}

type snapshot struct {
	name string
	body string
}

// fragment renders one named SQL fragment, normalising whitespace so that
// indentation changes alone do not fail the comparison.
func fragment(label, sql string) string {
	return "  " + label + ": " + strings.Join(strings.Fields(sql), " ") + "\n"
}

// renderArgs prints bound arguments sorted by bind name. ClickHouse resolves
// named binds by name, so emission order carries no meaning; sorting keeps the
// golden sensitive to what matters — a missing, extra, retyped, or revalued
// bind — while letting a builder regroup its clauses freely.
func renderArgs(args []any) string {
	lines := make([]string, 0, len(args))
	for _, a := range args {
		nv, ok := a.(driver.NamedValue)
		if !ok {
			lines = append(lines, fmt.Sprintf("    (positional) %s", renderValue(a)))
			continue
		}
		lines = append(lines, fmt.Sprintf("    @%s = %s", nv.Name, renderValue(nv.Value)))
	}
	sort.Strings(lines)

	var b strings.Builder
	b.WriteString("  args:\n")
	for _, l := range lines {
		b.WriteString(l + "\n")
	}
	return b.String()
}

// renderValue prints a bound value with its Go type. time.Time is normalised
// to UTC: time.UnixMilli yields a local-zone value, so printing it directly
// would make the golden depend on the machine's timezone.
func renderValue(v any) string {
	if t, ok := v.(time.Time); ok {
		return fmt.Sprintf("time.Time %s", t.UTC().Format(time.RFC3339Nano))
	}
	return fmt.Sprintf("%T %#v", v, v)
}

func allAttrOps() []string {
	return []string{"", "eq", "neq", "contains", "regex", "gt", "gte", "lt", "lte", "exists", "not_exists"}
}

func cases() []snapshot {
	var out []snapshot
	out = append(out, logsCases()...)
	out = append(out, tracesCases()...)
	out = append(out, dbCases()...)
	out = append(out, metricsCases()...)
	return out
}

func logsCases() []snapshot {
	base := func() logsfilter.Filters {
		return logsfilter.Filters{TenantID: 7, StartMs: startMs, EndMs: endMs}
	}
	render := func(f logsfilter.Filters) string {
		prewhere, where, args := logsfilter.BuildClauses(f)
		return fragment("prewhere", prewhere) + fragment("where", where) + renderArgs(args)
	}

	var out []snapshot
	out = append(out, snapshot{"logs/minimal", render(base())})

	// Every scalar and slice field, one at a time, so a diff names the field.
	fields := []struct {
		name string
		set  func(*logsfilter.Filters)
	}{
		{"services", func(f *logsfilter.Filters) { f.Services = []string{"api", "web"} }},
		{"excludeServices", func(f *logsfilter.Filters) { f.ExcludeServices = []string{"noisy"} }},
		{"hosts", func(f *logsfilter.Filters) { f.Hosts = []string{"h1"} }},
		{"excludeHosts", func(f *logsfilter.Filters) { f.ExcludeHosts = []string{"h2"} }},
		{"pods", func(f *logsfilter.Filters) { f.Pods = []string{"p1"} }},
		{"containers", func(f *logsfilter.Filters) { f.Containers = []string{"c1"} }},
		{"environments", func(f *logsfilter.Filters) { f.Environments = []string{"prod"} }},
		{"severities", func(f *logsfilter.Filters) { f.Severities = []string{"error", "Warn"} }},
		{"excludeSeverities", func(f *logsfilter.Filters) { f.ExcludeSeverities = []string{"debug"} }},
		{"traceID", func(f *logsfilter.Filters) { f.TraceID = "abc123" }},
		{"spanID", func(f *logsfilter.Filters) { f.SpanID = "def456" }},
		{"search", func(f *logsfilter.Filters) { f.Search = "Timeout" }},
		{"searchEscaping", func(f *logsfilter.Filters) { f.Search = `50% off_now\x` }},
	}
	for _, fl := range fields {
		f := base()
		fl.set(&f)
		out = append(out, snapshot{"logs/" + fl.name, render(f)})
	}

	// Attribute ops. Values chosen to hit the numeric and bool widening paths
	// in eq/neq, which is where logs differs most from traces.
	for _, op := range allAttrOps() {
		for _, v := range []struct{ label, val string }{
			{"string", "gateway"},
			{"numeric", "42"},
			{"bool", "true"},
		} {
			f := base()
			f.Attributes = []logsfilter.AttrFilter{{Key: "k", Op: op, Value: v.val}}
			out = append(out, snapshot{fmt.Sprintf("logs/attr/%s/%s", opLabel(op), v.label), render(f)})
		}
	}

	// Multiple attributes must get distinct bind indices.
	f := base()
	f.Attributes = []logsfilter.AttrFilter{
		{Key: "a", Op: "eq", Value: "1"},
		{Key: "b", Op: "contains", Value: "x"},
		{Key: "c", Op: "not_exists"},
	}
	out = append(out, snapshot{"logs/attr/multiple", render(f)})

	// Everything at once: pins the order fields are appended in.
	full := base()
	for _, fl := range fields {
		fl.set(&full)
	}
	full.Attributes = []logsfilter.AttrFilter{{Key: "a", Op: "gte", Value: "3"}}
	out = append(out, snapshot{"logs/all", render(full)})

	return out
}

func tracesCases() []snapshot {
	base := func() tracesfilter.Filters {
		return tracesfilter.Filters{TenantID: 7, StartMs: startMs, EndMs: endMs}
	}
	render := func(f tracesfilter.Filters) string {
		c := tracesfilter.BuildClauses(f)
		return fragment("resource", c.Resource) +
			fragment("span", c.Span) +
			fragment("root", c.Root) +
			fmt.Sprintf("  hasSpanMatch: %v\n", c.HasSpanMatch()) +
			renderArgs(c.Args)
	}

	tru, fls := true, false
	var out []snapshot
	out = append(out, snapshot{"traces/minimal", render(base())})

	fields := []struct {
		name string
		set  func(*tracesfilter.Filters)
	}{
		{"services", func(f *tracesfilter.Filters) { f.Services = []string{"api"} }},
		{"excludeServices", func(f *tracesfilter.Filters) { f.ExcludeServices = []string{"noisy"} }},
		{"operations", func(f *tracesfilter.Filters) { f.Operations = []string{"GET /x"} }},
		{"spanKinds", func(f *tracesfilter.Filters) { f.SpanKinds = []string{"SPAN_KIND_SERVER"} }},
		{"httpMethods", func(f *tracesfilter.Filters) { f.HTTPMethods = []string{"GET"} }},
		{"httpStatuses", func(f *tracesfilter.Filters) { f.HTTPStatuses = []string{"500"} }},
		{"statuses", func(f *tracesfilter.Filters) { f.Statuses = []string{"STATUS_CODE_ERROR"} }},
		{"excludeStatuses", func(f *tracesfilter.Filters) { f.ExcludeStatuses = []string{"STATUS_CODE_OK"} }},
		{"environments", func(f *tracesfilter.Filters) { f.Environments = []string{"prod"} }},
		{"peerServices", func(f *tracesfilter.Filters) { f.PeerServices = []string{"redis"} }},
		{"traceID", func(f *tracesfilter.Filters) { f.TraceID = "abc123" }},
		{"minDuration", func(f *tracesfilter.Filters) { f.MinDurationNs = 1_000_000 }},
		{"maxDuration", func(f *tracesfilter.Filters) { f.MaxDurationNs = 9_000_000 }},
		{"search", func(f *tracesfilter.Filters) { f.Search = "checkout" }},
		{"hasErrorTrue", func(f *tracesfilter.Filters) { f.HasError = &tru }},
		{"hasErrorFalse", func(f *tracesfilter.Filters) { f.HasError = &fls }},
	}
	for _, fl := range fields {
		f := base()
		fl.set(&f)
		out = append(out, snapshot{"traces/" + fl.name, render(f)})
	}

	for _, op := range allAttrOps() {
		f := base()
		f.Attributes = []tracesfilter.AttrFilter{{Key: "k", Op: op, Value: "42"}}
		out = append(out, snapshot{"traces/attr/" + opLabel(op), render(f)})
	}

	f := base()
	f.Attributes = []tracesfilter.AttrFilter{
		{Key: "a", Op: "eq", Value: "1"},
		{Key: "b", Op: "regex", Value: "^x"},
	}
	out = append(out, snapshot{"traces/attr/multiple", render(f)})

	full := base()
	for _, fl := range fields {
		fl.set(&full)
	}
	full.Attributes = []tracesfilter.AttrFilter{{Key: "a", Op: "lt", Value: "3"}}
	out = append(out, snapshot{"traces/all", render(full)})

	return out
}

func dbCases() []snapshot {
	render := func(f dbfilter.Filters) string {
		spanWhere, spanArgs := dbfilter.BuildSpanClauses(f)
		metricWhere, metricArgs := dbfilter.BuildMetricsClauses(f)
		return fragment("spanWhere", spanWhere) + renderArgs(spanArgs) +
			fragment("metricsWhere", metricWhere) + renderArgs(metricArgs)
	}

	var out []snapshot
	out = append(out, snapshot{"saturation/database/empty", render(dbfilter.Filters{})})

	fields := []struct {
		name string
		set  func(*dbfilter.Filters)
	}{
		{"dbSystem", func(f *dbfilter.Filters) { f.DBSystem = []string{"postgresql", "mysql"} }},
		{"collection", func(f *dbfilter.Filters) { f.Collection = []string{"orders"} }},
		{"namespace", func(f *dbfilter.Filters) { f.Namespace = []string{"app"} }},
		{"server", func(f *dbfilter.Filters) { f.Server = []string{"db.internal"} }},
	}
	for _, fl := range fields {
		var f dbfilter.Filters
		fl.set(&f)
		out = append(out, snapshot{"saturation/database/" + fl.name, render(f)})
	}

	var full dbfilter.Filters
	for _, fl := range fields {
		fl.set(&full)
	}
	out = append(out, snapshot{"saturation/database/all", render(full)})

	// ParseFilters is the HTTP entry point; pin the query-parameter names.
	req := httptest.NewRequest("GET",
		"/?dbSystem=postgresql&dbSystem=mysql&collection=orders&namespace=app&server=db.internal", nil)
	parsed := dbfilter.ParseFilters(req)
	out = append(out, snapshot{"saturation/database/parse", fmt.Sprintf("  parsed: %#v\n", parsed)})

	return out
}

func metricsCases() []snapshot {
	base := func() metricsfilter.Filters {
		return metricsfilter.Filters{
			TenantID: 7, StartMs: startMs, EndMs: endMs,
			MetricName: "http.server.duration", Aggregation: "avg",
		}
	}
	renderClauses := func(f metricsfilter.Filters) string {
		res, attr, args := metricsfilter.BuildClauses(f)
		return fragment("resourceWhere", res) + fragment("attrWhere", attr) + renderArgs(args)
	}
	renderSelection := func(f metricsfilter.Filters) string {
		from, cte, joins, sel, grp, args := metricsfilter.BuildSelection(f)
		return fragment("fromTable", from) + fragment("cte", cte) + fragment("joins", joins) +
			fragment("selectCols", sel) + fragment("groupByCols", grp) + renderArgs(args)
	}

	var out []snapshot
	out = append(out, snapshot{"metrics/minimal", renderClauses(base())})

	// Canonical resource keys fold into resourceWhere; unknown keys become
	// attribute lookups. Both aliases of each canonical key are exercised.
	for _, key := range []string{
		"service", "service.name", "host", "host.name",
		"environment", "deployment.environment",
		"k8s_namespace", "k8s.namespace.name",
		"custom.tag",
	} {
		for _, op := range []string{"=", "!=", "IN", "NOT IN"} {
			f := base()
			f.Tags = []metricsfilter.TagFilter{{Key: key, Operator: op, Values: []string{"a", "b"}}}
			out = append(out, snapshot{
				fmt.Sprintf("metrics/tag/%s/%s", key, opLabel(op)),
				renderClauses(f),
			})
		}
	}

	// Rejected inputs must emit nothing at all.
	for _, bad := range []struct {
		name string
		tag  metricsfilter.TagFilter
	}{
		{"badOperator", metricsfilter.TagFilter{Key: "service", Operator: "~=", Values: []string{"a"}}},
		{"noValues", metricsfilter.TagFilter{Key: "service", Operator: "=", Values: nil}},
	} {
		f := base()
		f.Tags = []metricsfilter.TagFilter{bad.tag}
		out = append(out, snapshot{"metrics/tag/rejected/" + bad.name, renderClauses(f)})
	}

	// Positive and negative on the same key accumulate separately.
	f := base()
	f.Tags = []metricsfilter.TagFilter{
		{Key: "service", Operator: "IN", Values: []string{"a"}},
		{Key: "service.name", Operator: "NOT IN", Values: []string{"b"}},
		{Key: "custom.one", Operator: "=", Values: []string{"x"}},
		{Key: "custom.two", Operator: "IN", Values: []string{"y", "z"}},
	}
	out = append(out, snapshot{"metrics/tag/mixed", renderClauses(f)})

	// Key sanitisation is a SQL-injection boundary: the key is interpolated,
	// not bound, so pin exactly what survives.
	f = base()
	f.Tags = []metricsfilter.TagFilter{{Key: `ev'il"; DROP--`, Operator: "=", Values: []string{"x"}}}
	out = append(out, snapshot{"metrics/tag/sanitizeKey", renderClauses(f)})

	// Selection: rollup table choice, CTE shape, and group-by projection.
	for _, sel := range []struct {
		name string
		set  func(*metricsfilter.Filters)
	}{
		{"noFilters", func(f *metricsfilter.Filters) {}},
		{"groupByResource", func(f *metricsfilter.Filters) { f.GroupBy = []string{"service"} }},
		{"groupByAttr", func(f *metricsfilter.Filters) { f.GroupBy = []string{"custom.tag"} }},
		{"groupByMixed", func(f *metricsfilter.Filters) { f.GroupBy = []string{"service", "custom.tag"} }},
		{"filtered", func(f *metricsfilter.Filters) {
			f.Tags = []metricsfilter.TagFilter{{Key: "service", Operator: "=", Values: []string{"api"}}}
		}},
	} {
		for _, step := range []string{"", "1m", "5m", "15m", "1h", "1d"} {
			f := base()
			f.Step = step
			sel.set(&f)
			out = append(out, snapshot{
				fmt.Sprintf("metrics/selection/%s/step=%s", sel.name, stepLabel(step)),
				renderSelection(f),
			})
		}
	}

	// Auto-grain thresholds: the boundaries of the duration switch.
	for _, hours := range []int64{1, 2, 3, 24, 25, 168, 169} {
		f := base()
		f.EndMs = f.StartMs + hours*3_600_000
		out = append(out, snapshot{
			fmt.Sprintf("metrics/selection/autoGrain/%dh", hours),
			fmt.Sprintf("  bucketSeconds: %d\n",
				metricsfilter.BucketDurationSeconds(f.StartMs, f.EndMs, "")),
		})
	}

	// Tag-value arms: one arm per resolvable key, skipped for unresolvable.
	arms, args := metricsfilter.BuildTagValueArms([]string{"service", "custom.tag"})
	var b strings.Builder
	for i, arm := range arms {
		b.WriteString(fragment(fmt.Sprintf("arm[%d]", i), arm))
	}
	b.WriteString(renderArgs(args))
	out = append(out, snapshot{"metrics/tagValueArms", b.String()})

	return out
}

// opLabel makes operator strings safe and readable as snapshot names.
func opLabel(op string) string {
	switch op {
	case "":
		return "empty"
	case "=":
		return "eq"
	case "!=":
		return "neq"
	case "IN":
		return "in"
	case "NOT IN":
		return "notin"
	}
	return op
}

func stepLabel(step string) string {
	if step == "" {
		return "auto"
	}
	return step
}
