package logfacets

import (
	"strings"
	"testing"

	"github.com/optikklabs/query/internal/modules/logs/filter"
)

func TestBuildFacetQuery(t *testing.T) {
	prewhere, where, _ := filter.BuildClauses(filter.Filters{
		TenantID: 1,
		StartMs:  100,
		EndMs:    200,
		Services: []string{"checkout"},
	})

	q := buildFacetQuery(prewhere, where)

	if !strings.Contains(q, "GROUP BY GROUPING SETS") {
		t.Errorf("query missing GROUP BY GROUPING SETS:\n%s", q)
	}
	if !strings.Contains(q, "LIMIT @facetLimit BY dim") {
		t.Errorf("query missing LIMIT BY dim:\n%s", q)
	}
	if strings.Contains(q, "UNION ALL") {
		t.Errorf("query contains obsolete UNION ALL:\n%s", q)
	}
}
