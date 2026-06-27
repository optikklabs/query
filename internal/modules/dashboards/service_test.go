package dashboards

import (
	"encoding/json"
	"errors"
	"testing"
)

func validWidget() CreateWidgetRequest {
	return CreateWidgetRequest{
		PanelType:     "latency",
		LayoutVariant: "standard-chart",
		Spec:          json.RawMessage(`{"panelType":"latency","query":{"method":"GET","endpoint":"/spans/red/latency-percentiles-timeseries","params":{"service":"checkout"}}}`),
		Layout:        json.RawMessage(`{"x":0,"y":0,"w":6,"h":4}`),
	}
}

func asValidation(t *testing.T, err error) ErrValidation {
	t.Helper()
	var ve ErrValidation
	if !errors.As(err, &ve) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
	return ve
}

func TestValidateWidget_OK(t *testing.T) {
	if err := validateWidget(validWidget()); err != nil {
		t.Fatalf("expected valid widget, got %v", err)
	}
}

func TestValidateWidget_RejectsNonAllowlistedEndpoint(t *testing.T) {
	req := validWidget()
	req.Spec = json.RawMessage(`{"panelType":"latency","query":{"method":"GET","endpoint":"/internal/secret-dump"}}`)
	err := validateWidget(req)
	if err == nil {
		t.Fatal("expected non-allowlisted endpoint to be rejected")
	}
	asValidation(t, err)
}

func TestValidateWidget_RejectsMissingEndpoint(t *testing.T) {
	req := validWidget()
	req.Spec = json.RawMessage(`{"panelType":"latency"}`)
	if err := validateWidget(req); err == nil {
		t.Fatal("expected missing query.endpoint to be rejected")
	}
}

func TestValidateWidget_RejectsUnknownPanelType(t *testing.T) {
	req := validWidget()
	req.PanelType = "totally-made-up"
	if err := validateWidget(req); err == nil {
		t.Fatal("expected unknown panel_type to be rejected")
	}
}

func TestValidateWidget_RejectsBadLayout(t *testing.T) {
	cases := map[string]json.RawMessage{
		"missing h":     json.RawMessage(`{"x":0,"y":0,"w":6}`),
		"zero w":        json.RawMessage(`{"x":0,"y":0,"w":0,"h":4}`),
		"negative x":    json.RawMessage(`{"x":-1,"y":0,"w":6,"h":4}`),
		"not an object": json.RawMessage(`"nope"`),
	}
	for name, layout := range cases {
		t.Run(name, func(t *testing.T) {
			req := validWidget()
			req.Layout = layout
			if err := validateWidget(req); err == nil {
				t.Fatalf("expected layout %q to be rejected", name)
			}
		})
	}
}

func builderWidget() CreateWidgetRequest {
	return CreateWidgetRequest{
		PanelType: "metrics-timeseries",
		Spec:      json.RawMessage(`{"panelType":"metrics-timeseries","query":{"kind":"metrics","step":"5m","queries":[{"id":"a","aggregation":"avg","metricName":"traces.span.metrics.duration","where":[{"key":"service","operator":"eq","value":"checkout"}],"groupBy":["region"]}]}}`),
		Layout:    json.RawMessage(`{"x":0,"y":0,"w":6,"h":4}`),
	}
}

func TestValidateWidget_BuilderOK(t *testing.T) {
	if err := validateWidget(builderWidget()); err != nil {
		t.Fatalf("expected valid builder widget, got %v", err)
	}
}

func TestValidateWidget_BuilderRejectsBadAggregation(t *testing.T) {
	req := builderWidget()
	req.Spec = json.RawMessage(`{"query":{"kind":"metrics","queries":[{"aggregation":"median","metricName":"m"}]}}`)
	asValidation(t, validateWidget(req))
}

func TestValidateWidget_BuilderRejectsEmptyMetric(t *testing.T) {
	req := builderWidget()
	req.Spec = json.RawMessage(`{"query":{"kind":"metrics","queries":[{"aggregation":"avg","metricName":"  "}]}}`)
	asValidation(t, validateWidget(req))
}

func TestValidateWidget_BuilderRejectsNoQueries(t *testing.T) {
	req := builderWidget()
	req.Spec = json.RawMessage(`{"query":{"kind":"metrics","queries":[]}}`)
	asValidation(t, validateWidget(req))
}

func TestValidateWidget_BuilderRejectsBadOperator(t *testing.T) {
	req := builderWidget()
	req.Spec = json.RawMessage(`{"query":{"kind":"metrics","queries":[{"aggregation":"avg","metricName":"m","where":[{"key":"k","operator":"regex","value":"v"}]}]}}`)
	asValidation(t, validateWidget(req))
}

func TestBuildPageArgs_NameRequired(t *testing.T) {
	if _, err := buildPageArgs(1, 1, CreatePageRequest{Name: "   "}); err == nil {
		t.Fatal("expected blank name to be rejected")
	}
}

func TestBuildPageArgs_DefaultsAndTags(t *testing.T) {
	args, err := buildPageArgs(7, 3, CreatePageRequest{Name: "  My Page  ", Tags: []string{"prod"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if args.Name != "My Page" {
		t.Fatalf("name not trimmed: %q", args.Name)
	}
	if args.Icon != "layout-grid" || args.IconColor != "primary" {
		t.Fatalf("expected icon defaults, got %q/%q", args.Icon, args.IconColor)
	}
	if string(args.TagsJSON) != `["prod"]` {
		t.Fatalf("unexpected tags json: %s", args.TagsJSON)
	}
	if !args.CreatedByUserID.Valid || args.CreatedByUserID.Int64 != 3 {
		t.Fatalf("expected created_by_user_id to be set")
	}
}

func TestBuildPageArgs_EmptyTagsSerializeToArray(t *testing.T) {
	args, err := buildPageArgs(1, 0, CreatePageRequest{Name: "p"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(args.TagsJSON) != "[]" {
		t.Fatalf("expected [] tags, got %s", args.TagsJSON)
	}
	if args.CreatedByUserID.Valid {
		t.Fatal("expected anonymous create to leave created_by_user_id null")
	}
}
