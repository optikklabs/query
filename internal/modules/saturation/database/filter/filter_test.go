package filter

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/optikklabs/query/internal/shared/chtest"
)

func TestParseFilters(t *testing.T) {
	req := httptest.NewRequest("GET", "/?dbSystem=postgresql&dbSystem=mysql&collection=orders&namespace=app&server=db.internal", nil)
	got := ParseFilters(req)
	if len(got.DBSystem) != 2 || got.DBSystem[0] != "postgresql" || got.DBSystem[1] != "mysql" {
		t.Fatalf("DBSystem = %v", got.DBSystem)
	}
	if got.Collection[0] != "orders" || got.Namespace[0] != "app" || got.Server[0] != "db.internal" {
		t.Fatalf("filters = %#v", got)
	}
}

func TestBuildSpanClauses(t *testing.T) {
	where, args := BuildSpanClauses(Filters{
		DBSystem:   []string{"postgresql"},
		Collection: []string{"orders"},
		Namespace:  []string{"app"},
		Server:     []string{"db.internal"},
	})
	for _, bind := range []string{"@dbSystem", "@dbCollection", "@dbNamespace", "@dbServer"} {
		if !strings.Contains(where, bind) {
			t.Errorf("WHERE %q missing %s", where, bind)
		}
	}
	if !strings.Contains(where, "db_name IN @dbCollection") {
		t.Errorf("WHERE %q does not use promoted db_name for collection filtering", where)
	}
	if len(chtest.NamedArgs(args)) != 4 {
		t.Fatalf("args = %#v", chtest.NamedArgs(args))
	}
}

func TestBuildMetricsClausesOnlyUsesProducedDimensions(t *testing.T) {
	where, args := BuildMetricsClauses(Filters{
		DBSystem:   []string{"postgresql"},
		Collection: []string{"orders"},
		Namespace:  []string{"app"},
		Server:     []string{"db.internal"},
	})
	if !strings.Contains(where, "@dbSystem") {
		t.Fatalf("WHERE %q missing db_system", where)
	}
	for _, unsupported := range []string{"dbCollection", "dbNamespace", "dbServer"} {
		if strings.Contains(where, unsupported) {
			t.Errorf("WHERE %q contains unsupported %s filter", where, unsupported)
		}
	}
	gotArgs := chtest.NamedArgs(args)
	if len(gotArgs) != 1 || gotArgs["dbSystem"] == nil {
		t.Fatalf("args = %#v", gotArgs)
	}
}
