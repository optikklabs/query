package template

import "testing"

func TestFormatFloat(t *testing.T) {
	cases := map[float64]string{
		1:      "1",
		1.5:    "1.5",
		1000:   "1000",
		0.0001: "0.0001",
	}
	for in, want := range cases {
		if got := FormatFloat(in); got != want {
			t.Errorf("FormatFloat(%v) = %q, want %q", in, got, want)
		}
	}
}

// renderScalars replaces {{key}} with its value; unknown keys collapse to empty
// and section markers pass through untouched.
func TestRenderScalars(t *testing.T) {
	vals := map[string]string{"name": "orders", "count": "5"}
	cases := []struct {
		body, want string
	}{
		{"svc {{name}} has {{count}}", "svc orders has 5"},
		{"{{ name }} trimmed", "orders trimmed"},
		{"missing {{nope}} key", "missing  key"},
		{"unterminated {{name", "unterminated {{name"},
		{"section {{#is_alert}} kept", "section {{#is_alert}} kept"},
	}
	for _, c := range cases {
		if got := renderScalars(c.body, vals); got != c.want {
			t.Errorf("renderScalars(%q) = %q, want %q", c.body, got, c.want)
		}
	}
}

func TestRenderSections(t *testing.T) {
	body := "a{{#is_alert}}B{{/is_alert}}c"
	if got := renderSections(body, "is_alert", true); got != "aBc" {
		t.Errorf("keep=true -> %q, want aBc", got)
	}
	if got := renderSections(body, "is_alert", false); got != "ac" {
		t.Errorf("keep=false -> %q, want ac", got)
	}
}

func TestRender_Flow(t *testing.T) {
	body := "{{#is_alert}}ALERT {{svc}}{{/is_alert}}{{#is_recovery}}ok{{/is_recovery}}"
	got := Render(body, Vars{
		Values:  map[string]string{"svc": "orders"},
		IsAlert: true,
	})
	if got != "ALERT orders" {
		t.Errorf("Render = %q, want \"ALERT orders\"", got)
	}
}
