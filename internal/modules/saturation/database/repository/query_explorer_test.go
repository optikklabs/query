package repository

import (
	"strings"
	"testing"

	"github.com/optikklabs/query/internal/modules/saturation/database/filter"
)

func TestBuildExplorerClausesSeparatesRowAndAggregateFilters(t *testing.T) {
	minCalls := uint64(10)
	maxP99 := 500.0
	where, having, args := buildExplorerClauses(filter.ExplorerFilters{
		DBSystems:    []string{"postgresql"},
		Collections:  []string{"payments"},
		Services:     []string{"checkout"},
		QueryText:    "select",
		MinCallCount: &minCalls,
		MaxP99Ms:     &maxP99,
	}, QueryPatternsCursor{})

	for _, fragment := range []string{"db_system IN @dbSystems", "db_name IN @collections", "service IN @services", "positionCaseInsensitive"} {
		if !strings.Contains(where, fragment) {
			t.Fatalf("row filter %q missing from WHERE: %s", fragment, where)
		}
	}
	for _, fragment := range []string{"call_count >= @minCallCount", "qs[3] <= @maxP99Ms"} {
		if !strings.Contains(having, fragment) {
			t.Fatalf("aggregate filter %q missing from HAVING: %s", fragment, having)
		}
	}
	if got, want := len(args), 6; got != want {
		t.Fatalf("argument count = %d, want %d", got, want)
	}
}

func TestBuildExplorerClausesAddsDeterministicCursor(t *testing.T) {
	_, having, args := buildExplorerClauses(filter.ExplorerFilters{}, QueryPatternsCursor{
		CallCount:      42,
		QueryHash:      "hash",
		DBSystem:       "postgresql",
		CollectionName: "payments",
	})

	for _, fragment := range []string{
		"call_count < @cursorCallCount",
		"query_hash > @cursorQueryHash",
		"db_system > @cursorDBSystem",
		"collection_name > @cursorCollection",
	} {
		if !strings.Contains(having, fragment) {
			t.Fatalf("cursor clause %q missing from HAVING: %s", fragment, having)
		}
	}
	if got, want := len(args), 4; got != want {
		t.Fatalf("argument count = %d, want %d", got, want)
	}
}
