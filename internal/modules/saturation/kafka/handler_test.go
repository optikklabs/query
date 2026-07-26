package kafka

import "testing"

// An empty or comma-padded param must not widen the query to every service.
func TestParseServices(t *testing.T) {
	cases := map[string][]string{
		"":          {},
		",":         {},
		"  ":        {},
		"a":         {"a"},
		"a,b":       {"a", "b"},
		" a , ,b, ": {"a", "b"},
	}
	for raw, want := range cases {
		got := parseServices(raw)
		if len(got) != len(want) {
			t.Errorf("parseServices(%q) = %v, want %v", raw, got, want)
			continue
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("parseServices(%q) = %v, want %v", raw, got, want)
				break
			}
		}
	}
}
