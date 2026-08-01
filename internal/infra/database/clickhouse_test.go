package database

import (
	"testing"

	"github.com/optikklabs/query/internal/config"
)

func TestBudgetSettingsDisableQueryResultCache(t *testing.T) {
	settings := budgetSettings(config.QueryBudget{})

	if got := settings["use_query_cache"]; got != 0 {
		t.Fatalf("use_query_cache = %v, want 0", got)
	}
	for _, key := range []string{"query_cache_ttl", "query_cache_share_between_users"} {
		if _, ok := settings[key]; ok {
			t.Fatalf("result-cache setting %q should not be configured", key)
		}
	}
}
