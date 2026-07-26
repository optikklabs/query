package repository

import (
	"strings"
	"testing"
)

func TestQueriesDoNotSortByTraffic(t *testing.T) {
	if strings.Contains(clientsQuery, "count()") {
		t.Fatal("clients query must not rank services by series count")
	}
	if !strings.Contains(clientsQuery, "SELECT DISTINCT service") || !strings.Contains(clientsQuery, "ORDER BY service") {
		t.Fatal("clients query must return distinct services in deterministic name order")
	}
	if strings.Contains(edgesQuery("rollup"), "ORDER BY call_count") {
		t.Fatal("edges query must not sort aggregated rows by call count")
	}
}
