package repository

import "testing"

func edge(service, topic string) EdgeRow {
	return EdgeRow{Service: service, Topic: topic}
}

// Replaces the scoped_topics CTE: every edge on a topic the selected services
// touch is kept, including edges from services outside the selection.
func TestScopeEdgesToTopics(t *testing.T) {
	rows := []EdgeRow{
		edge("orders", "payments"),
		edge("ledger", "payments"),
		edge("audit", "unrelated"),
	}
	got := scopeEdgesToTopics(rows, []string{"orders"})
	if len(got) != 2 {
		t.Fatalf("want 2 edges on the payments topic, got %d: %+v", len(got), got)
	}
	for _, row := range got {
		if row.Topic != "payments" {
			t.Errorf("unscoped topic leaked: %+v", row)
		}
	}
}

func TestScopeEdgesToTopicsNoMatch(t *testing.T) {
	rows := []EdgeRow{edge("audit", "unrelated")}
	if got := scopeEdgesToTopics(rows, []string{"orders"}); len(got) != 0 {
		t.Errorf("want no edges, got %+v", got)
	}
}
