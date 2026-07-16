package redfleet

import (
	"strings"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

func TestBuildREDClausesPrunesMetricsSeriesIndexBuckets(t *testing.T) {
	start := time.Date(2026, time.July, 16, 7, 25, 0, 0, time.UTC)
	end := start.Add(30 * time.Minute)
	where, args := BuildREDClauses(REDFilters{
		TenantID: 1,
		StartMs:  start.UnixMilli(),
		EndMs:    end.UnixMilli(),
	})

	if !strings.Contains(where, "toStartOfInterval(s.timestamp, INTERVAL 6 HOUR)") {
		t.Fatalf("series clause does not constrain the indexed time bucket: %q", where)
	}

	named := namedArgs(args)
	wantBucket := time.Date(2026, time.July, 16, 6, 0, 0, 0, time.UTC)
	if got := named["seriesBucketStart"]; got != wantBucket {
		t.Fatalf("seriesBucketStart = %v, want %v", got, wantBucket)
	}
	if got := named["seriesBucketEnd"]; got != wantBucket {
		t.Fatalf("seriesBucketEnd = %v, want %v", got, wantBucket)
	}
}

func TestBuildREDClausesKeepsServiceFilter(t *testing.T) {
	where, args := BuildREDClauses(REDFilters{
		TenantID: 2,
		StartMs:  time.Date(2026, time.July, 16, 5, 50, 0, 0, time.UTC).UnixMilli(),
		EndMs:    time.Date(2026, time.July, 16, 6, 20, 0, 0, time.UTC).UnixMilli(),
		Services: []string{"checkout"},
	})

	if !strings.Contains(where, "s.service = @serviceName") {
		t.Fatalf("service filter missing from clause: %q", where)
	}

	named := namedArgs(args)
	if got := named["serviceName"]; got != "checkout" {
		t.Fatalf("serviceName = %v, want checkout", got)
	}
	if got, want := named["seriesBucketStart"], time.Date(2026, time.July, 16, 0, 0, 0, 0, time.UTC); got != want {
		t.Fatalf("seriesBucketStart = %v, want %v", got, want)
	}
	if got, want := named["seriesBucketEnd"], time.Date(2026, time.July, 16, 6, 0, 0, 0, time.UTC); got != want {
		t.Fatalf("seriesBucketEnd = %v, want %v", got, want)
	}
}

func namedArgs(args []any) map[string]any {
	named := make(map[string]any)
	for _, arg := range args {
		if value, ok := arg.(driver.NamedValue); ok {
			named[value.Name] = value.Value
		}
	}
	return named
}
