package redfleet

import (
	"strings"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

func TestBuildREDClausesNoServiceFilter(t *testing.T) {
	start := time.Date(2026, time.July, 16, 7, 25, 0, 0, time.UTC)
	end := start.Add(30 * time.Minute)
	where, args := BuildREDClauses(REDFilters{
		TenantID: 1,
		StartMs:  start.UnixMilli(),
		EndMs:    end.UnixMilli(),
	})

	if where != "" {
		t.Fatalf("fleet-wide clause should be empty, got %q", where)
	}

	named := namedArgs(args)
	if got := named["tenantID"]; got != uint32(1) {
		t.Fatalf("tenantID = %v, want 1", got)
	}
	if got, ok := named["start"].(time.Time); !ok || !got.Equal(start) {
		t.Fatalf("start = %v, want %v", named["start"], start)
	}
}

func TestBuildREDClausesKeepsServiceFilter(t *testing.T) {
	where, args := BuildREDClauses(REDFilters{
		TenantID: 2,
		StartMs:  time.Date(2026, time.July, 16, 5, 50, 0, 0, time.UTC).UnixMilli(),
		EndMs:    time.Date(2026, time.July, 16, 6, 20, 0, 0, time.UTC).UnixMilli(),
		Services: []string{"checkout"},
	})

	if !strings.Contains(where, "service = @serviceName") {
		t.Fatalf("service filter missing from clause: %q", where)
	}

	named := namedArgs(args)
	if got := named["serviceName"]; got != "checkout" {
		t.Fatalf("serviceName = %v, want checkout", got)
	}
}

func TestBuildREDClausesMultiServiceFilter(t *testing.T) {
	where, args := BuildREDClauses(REDFilters{
		TenantID: 2,
		StartMs:  time.Date(2026, time.July, 16, 5, 50, 0, 0, time.UTC).UnixMilli(),
		EndMs:    time.Date(2026, time.July, 16, 6, 20, 0, 0, time.UTC).UnixMilli(),
		Services: []string{"checkout", "cart"},
	})

	if !strings.Contains(where, "service IN @services") {
		t.Fatalf("multi-service filter missing from clause: %q", where)
	}
	if _, ok := namedArgs(args)["services"]; !ok {
		t.Fatalf("services arg missing")
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
