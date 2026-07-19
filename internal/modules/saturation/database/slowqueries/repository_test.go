package slowqueries

import (
	"strings"
	"testing"
)

func TestClampLimit(t *testing.T) {
	tests := []struct {
		name  string
		input int
		want  int
	}{
		{name: "default", input: 0, want: defaultLimit},
		{name: "negative", input: -1, want: defaultLimit},
		{name: "accepted", input: 50, want: 50},
		{name: "clamped", input: 1000, want: maxLimit},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := clampLimit(tt.input); got != tt.want {
				t.Errorf("clampLimit(%d) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestSlowQueryPatternsQueryPreservesScope(t *testing.T) {
	query := slowQueryPatternsQuery(" AND db_system IN @dbSystem")
	for _, want := range []string{
		"db_system",
		"db_name",
		"timestamp BETWEEN @start AND @end",
		"query_hash != ''",
		"GROUP BY query_hash, db_system, collection_name",
		"AND db_system IN @dbSystem",
	} {
		if !strings.Contains(query, want) {
			t.Errorf("query missing %q:\n%s", want, query)
		}
	}
	if strings.Contains(query, "attributes.") {
		t.Errorf("hot query must not read dynamic JSON attributes:\n%s", query)
	}
	if strings.Contains(query, "db_statement != ''") {
		t.Errorf("hot query must not read db_statement just to test for a fingerprint:\n%s", query)
	}
}
