package spanstats_test

import (
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Columns of the span_stats rollup, mirroring the DDL in the ingest repo at
// ingest/db/14_span_stats.sql. Add a column here when the cascade gains one.
var spanStatsColumns = map[string]bool{
	"tenant_id": true, "timestamp": true, "service": true, "environment": true,
	"host": true, "pod": true, "span_name": true, "kind_string": true,
	"status_code_string": true, "http_status_bucket": true, "http_route": true,
	"http_method": true, "rpc_system": true, "db_system": true,
	"messaging_system": true, "messaging_destination": true,
	"messaging_consumer_group": true, "cloud_provider": true,
	"cloud_platform": true, "cloud_region": true, "k8s_node": true,
	"peer_name": true, "peer_type": true, "request_count": true,
	"duration_ms_sum": true, "latency_state": true,
}

// Matches `<agg>(...) AS <alias>`, allowing one level of nested parens for
// parametric aggregates such as quantilesTDigestMerge(0.95)(latency_state).
var aggAlias = regexp.MustCompile(
	`(?i)\b(sum|sumIf|count|countIf|any|anyIf|anyLast|anyLastIf|min|max|minIf|maxIf|` +
		`avg|avgIf|uniq|uniqIf|uniqExact|uniqCombined64|argMin|argMax|topK|median|` +
		`groupArray|groupUniqArray|groupUniqArrayIf|quantile\w*)` +
		`(?:\([^()]*\))?\s*\((?:[^()]|\([^()]*\))*\)\s+AS\s+(\w+)`)

// shadowedAliases returns the offending `AGG(x) AS x` fragments in one query.
func shadowedAliases(query string) []string {
	var out []string
	for _, m := range aggAlias.FindAllStringSubmatch(query, -1) {
		if spanStatsColumns[m[2]] {
			out = append(out, strings.Join(strings.Fields(m[0]), " "))
		}
	}
	return out
}

// readsSpanStats reports whether a query targets the span_stats cascade. Only
// that cascade shares this column vocabulary; queries over spans, logs, or
// metrics_series are unaffected by the rule.
func readsSpanStats(query string) bool {
	return strings.Contains(query, "span_stats") || strings.Contains(query, "SpanStatsRollup")
}

// A ClickHouse alias outranks the physical column of the same name, so
// `sum(request_count) AS request_count` makes every later reference to
// request_count — a sibling aggregate, or a PREWHERE predicate appended by a
// filter builder — resolve to the aggregate and fail with ILLEGAL_AGGREGATION
// (code 184). That outage is invisible to Go's type system and to every test
// that does not hand the SQL to ClickHouse, so it is asserted statically here.
//
// Use the spanstats projection constants instead of retyping the measures;
// their aliases are chosen not to collide.
func TestNoAggregateAliasShadowsASpanStatsColumn(t *testing.T) {
	fset := token.NewFileSet()
	byFile := map[string]int{}

	err := filepath.WalkDir(filepath.Join("..", "..", "..", "internal"),
		func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") ||
				strings.HasSuffix(path, "_test.go") {
				return err
			}
			f, perr := parser.ParseFile(fset, path, nil, 0)
			if perr != nil {
				t.Errorf("parse %s: %v", path, perr)
				return nil
			}
			for _, q := range sqlStrings(fset, f) {
				if !readsSpanStats(q) || !strings.Contains(strings.ToUpper(q), "SELECT") {
					continue
				}
				byFile[path]++
				for _, bad := range shadowedAliases(q) {
					t.Errorf("%s: aggregate aliased to its own column:\n    %s\n"+
						"    use a spanstats projection constant, or pick a "+
						"non-colliding alias", path, bad)
				}
			}
			return nil
		})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}

	// Every package known to read the cascade must be reached, so a change to
	// how queries are built cannot silently drop files from the scan and let
	// this test pass vacuously — which it did while topology's table name came
	// from a local variable the extractor had not resolved.
	for _, want := range []string{
		"services/redfleet", "services/errors", "services/topology",
		"cloud", "infrastructure/nodes", "infrastructure/hosts",
		"infrastructure/fleet", "infrastructure/containerdetail",
		"alerting/shared/query", "saturation/database", "saturation/kafka",
	} {
		found := false
		for path := range byFile {
			if strings.Contains(filepath.ToSlash(path), want) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("no span_stats query found under %q: the extractor is "+
				"missing queries it used to see", want)
		}
	}
}

// Guards the detector itself: if the regex or the scope check stops working,
// the walk above would pass by finding nothing wrong.
func TestShadowDetection(t *testing.T) {
	for _, tc := range []struct {
		name string
		sql  string
		want bool
	}{
		{"self-aliased sum", "SELECT sum(request_count) AS request_count FROM optikk.span_stats_1m", true},
		{"self-aliased any dimension", "SELECT any(service) AS service FROM optikk.span_stats_1m", true},
		{"self-aliased sumIf", "SELECT sumIf(request_count, x) AS request_count FROM optikk.span_stats_1m", true},
		{"parametric aggregate", "SELECT quantilesTDigestMerge(0.95)(latency_state) AS latency_state FROM x", true},
		{"renamed alias", "SELECT sum(request_count) AS request_total FROM optikk.span_stats_1m", false},
		{"plain column alias", "SELECT service AS service FROM optikk.span_stats_1m", false},
		{"alias is not a column", "SELECT sum(request_count) AS error_count FROM optikk.span_stats_1m", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := len(shadowedAliases(tc.sql)) > 0; got != tc.want {
				t.Errorf("shadowedAliases(%q) flagged=%v, want %v", tc.sql, got, tc.want)
			}
		})
	}
}

func TestReadsSpanStats(t *testing.T) {
	for _, tc := range []struct {
		sql  string
		want bool
	}{
		{"SELECT x FROM optikk.span_stats_1m", true},
		{"SELECT x FROM  timebucket.SpanStatsRollup(d) ", true},
		{"SELECT x FROM optikk.spans", false},
		{"SELECT x FROM optikk.metrics_series", false},
	} {
		if got := readsSpanStats(tc.sql); got != tc.want {
			t.Errorf("readsSpanStats(%q) = %v, want %v", tc.sql, got, tc.want)
		}
	}
}

// sqlStrings flattens `+` concatenation chains into whole queries, resolving
// string locals declared in the same function. Without that resolution, a query
// that names its table via a local (FROM ... + rollup + ...) hides that table
// from readsSpanStats and silently escapes the check.
func sqlStrings(fset *token.FileSet, f *ast.File) []string {
	var out []string
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			out = append(out, rawStrings(fset, decl, nil)...)
			continue
		}
		out = append(out, rawStrings(fset, fn, bindings(fset, fn))...)
	}
	return out
}

// bindings maps each `name := <expr>` in a function to that expr's source text.
func bindings(fset *token.FileSet, fn *ast.FuncDecl) map[string]string {
	out := map[string]string{}
	ast.Inspect(fn, func(n ast.Node) bool {
		as, ok := n.(*ast.AssignStmt)
		if !ok || len(as.Lhs) != len(as.Rhs) {
			return true
		}
		for i, lhs := range as.Lhs {
			if id, ok := lhs.(*ast.Ident); ok {
				out[id.Name] = flatten(fset, as.Rhs[i], nil)
			}
		}
		return true
	})
	return out
}

func rawStrings(fset *token.FileSet, root ast.Node, binds map[string]string) []string {
	var out []string
	ast.Inspect(root, func(n ast.Node) bool {
		switch e := n.(type) {
		case *ast.BinaryExpr:
			if e.Op != token.ADD {
				return true
			}
			out = append(out, flatten(fset, e, binds))
			return false // children belong to this chain
		case *ast.BasicLit:
			if e.Kind == token.STRING {
				out = append(out, litValue(e))
			}
		}
		return true
	})
	return out
}

func flatten(fset *token.FileSet, n ast.Expr, binds map[string]string) string {
	switch e := n.(type) {
	case *ast.BinaryExpr:
		if e.Op == token.ADD {
			return flatten(fset, e.X, binds) + flatten(fset, e.Y, binds)
		}
	case *ast.BasicLit:
		if e.Kind == token.STRING {
			return litValue(e)
		}
	case *ast.Ident:
		if text, ok := binds[e.Name]; ok {
			return " " + text + " "
		}
	}
	var sb strings.Builder
	if err := printer.Fprint(&sb, fset, n); err != nil {
		return " "
	}
	return " " + sb.String() + " "
}

func litValue(l *ast.BasicLit) string { return strings.Trim(l.Value, "`\"") }
