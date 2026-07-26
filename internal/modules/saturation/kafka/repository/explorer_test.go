package repository

import (
	"strings"
	"testing"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/optikklabs/query/internal/modules/saturation/kafka/filter"
)

// seriesCTE includes only the dimension columns requested, plus the base guard
// and optional drill-down filter.
func TestSeriesCTE_Columns(t *testing.T) {
	topicCol := filter.AttrTopic + " AS topic"
	groupCol := filter.AttrConsumerGroup + " AS consumer_group"

	both := seriesCTE(true, true, "base", "AND extra")
	if !strings.Contains(both, topicCol) || !strings.Contains(both, groupCol) {
		t.Errorf("needTopic+needGroup must select both dims:\n%s", both)
	}
	if !strings.Contains(both, "base") || !strings.Contains(both, "AND extra") {
		t.Errorf("base/extra where not threaded through:\n%s", both)
	}

	topicOnly := seriesCTE(true, false, "base", "")
	if !strings.Contains(topicOnly, topicCol) || strings.Contains(topicOnly, groupCol) {
		t.Errorf("needTopic only must omit group col:\n%s", topicOnly)
	}

	none := seriesCTE(false, false, "base", "")
	if strings.Contains(none, topicCol) || strings.Contains(none, groupCol) {
		t.Errorf("no dims requested must select only fingerprint:\n%s", none)
	}
}

func TestBuildFilterArgs(t *testing.T) {
	cases := []struct {
		name      string
		col, val  string
		wantWhere string
		wantBind  bool
	}{
		{"topic filter", "topic", "orders", "AND " + filter.AttrTopic + " = @filterVal", true},
		{"group filter", "consumer_group", "g1", "AND " + filter.AttrConsumerGroup + " = @filterVal", true},
		{"no value -> no filter", "topic", "", "", false},
		{"unknown col -> no filter but still binds", "partition", "3", "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			where, args := buildFilterArgs(1, 0, 1000, topicThroughputMetrics, c.col, c.val)
			if where != c.wantWhere {
				t.Errorf("where = %q, want %q", where, c.wantWhere)
			}
			if got := bindsFilterVal(args); got != c.wantBind {
				t.Errorf("binds @filterVal = %v, want %v", got, c.wantBind)
			}
		})
	}
}

func bindsFilterVal(args []any) bool {
	for _, a := range args {
		if nv, ok := a.(driver.NamedValue); ok && nv.Name == "filterVal" {
			return true
		}
	}
	return false
}
