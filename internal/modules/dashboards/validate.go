package dashboards

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/optikklabs/query/internal/shared/errorcode"
)

func validateWidget(spec json.RawMessage) (querySpecProbe, error) {
	var probe querySpecProbe
	if err := json.Unmarshal(spec, &probe); err != nil {
		return probe, errorcode.ValidationError{Msg: "spec must be a valid panel spec object"}
	}
	if !isValidPanelType(probe.PanelType) {
		return probe, errorcode.ValidationError{Msg: fmt.Sprintf("panel_type %q is not a supported dashboard panel", probe.PanelType)}
	}
	if !isValidLayoutVariant(probe.LayoutVariant) {
		return probe, errorcode.ValidationError{Msg: fmt.Sprintf("layout_variant %q is not supported", probe.LayoutVariant)}
	}
	if err := validateLayout(probe.Layout); err != nil {
		return probe, err
	}
	if probe.Query == nil {
		return probe, errorcode.ValidationError{Msg: "spec.query is required"}
	}
	if probe.Query.Kind != "metrics" {
		return probe, errorcode.ValidationError{Msg: fmt.Sprintf("spec.query.kind %q is not supported; expected \"metrics\"", probe.Query.Kind)}
	}
	return probe, validateBuilderQuery(probe.Query.Queries)
}

type layoutProbe struct {
	X *float64 `json:"x"`
	Y *float64 `json:"y"`
	W *float64 `json:"w"`
	H *float64 `json:"h"`
}

func validateLayout(raw json.RawMessage) error {
	var l layoutProbe
	if err := json.Unmarshal(raw, &l); err != nil {
		return errorcode.ValidationError{Msg: "layout must be a {x,y,w,h} object"}
	}
	if l.X == nil || l.Y == nil || l.W == nil || l.H == nil {
		return errorcode.ValidationError{Msg: "layout requires x, y, w and h"}
	}
	if *l.W <= 0 || *l.H <= 0 {
		return errorcode.ValidationError{Msg: "layout w and h must be positive"}
	}
	if *l.X < 0 || *l.Y < 0 {
		return errorcode.ValidationError{Msg: "layout x and y must not be negative"}
	}
	return nil
}

type builderFilterProbe struct {
	Operator string `json:"operator"`
}

type builderQueryProbe struct {
	MetricName  string               `json:"metricName"`
	Aggregation string               `json:"aggregation"`
	Where       []builderFilterProbe `json:"where"`
}

type querySpecProbe struct {
	Title         string          `json:"title"`
	PanelType     string          `json:"panelType"`
	LayoutVariant string          `json:"layoutVariant"`
	Layout        json.RawMessage `json:"layout"`
	Query         *struct {
		Kind    string              `json:"kind"`
		Queries []builderQueryProbe `json:"queries"`
	} `json:"query"`
}

func validateBuilderQuery(queries []builderQueryProbe) error {
	if len(queries) == 0 {
		return errorcode.ValidationError{Msg: "spec.query.queries must have at least one query"}
	}
	for _, q := range queries {
		if strings.TrimSpace(q.MetricName) == "" {
			return errorcode.ValidationError{Msg: "spec.query.queries[].metricName is required"}
		}
		if !isValidBuilderAggregation(q.Aggregation) {
			return errorcode.ValidationError{Msg: fmt.Sprintf("aggregation %q is not supported", q.Aggregation)}
		}
		for _, f := range q.Where {
			if !isValidBuilderOperator(f.Operator) {
				return errorcode.ValidationError{Msg: fmt.Sprintf("filter operator %q is not supported", f.Operator)}
			}
		}
	}
	return nil
}
