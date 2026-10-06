package dashboards

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/optikklabs/query/internal/shared/errorcode"
)

const maxWidgetsPerPage = 30

// Widget specs come from the metrics widget builder; these are the only
// values it produces.
var (
	panelTypes        = []string{"metrics-timeseries", "metrics-value", "metrics-toplist", "metrics-table"}
	layoutVariants    = []string{"standard-chart", "kpi", "ranking", "detail-table"}
	steps             = []string{"1m", "5m", "15m", "1h", "1d"}
	aggregations      = []string{"avg", "sum", "min", "max", "count", "p50", "p95", "p99", "rate"}
	spaceAggregations = []string{"avg", "sum", "min", "max"}
	filterOperators   = []string{"eq", "neq", "in", "not_in", "wildcard"}
)

// panelSpecProbe holds the spec fields the server checks. The spec is stored
// verbatim and returned as-is; the web reads it back with the same contract.
type panelSpecProbe struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	PanelType     string `json:"panelType"`
	LayoutVariant string `json:"layoutVariant"`
	Legend        *bool  `json:"legend"`
	Smooth        *bool  `json:"smooth"`
	Layout        *struct {
		X *float64 `json:"x"`
		Y *float64 `json:"y"`
		W *float64 `json:"w"`
		H *float64 `json:"h"`
	} `json:"layout"`
	Query *struct {
		Kind             string              `json:"kind"`
		Step             string              `json:"step"`
		SpaceAggregation string              `json:"spaceAggregation"`
		Queries          []builderQueryProbe `json:"queries"`
	} `json:"query"`
}

type builderQueryProbe struct {
	MetricName       string `json:"metricName"`
	Aggregation      string `json:"aggregation"`
	SpaceAggregation string `json:"spaceAggregation"`
	Where            []struct {
		Operator string `json:"operator"`
	} `json:"where"`
}

func invalid(format string, args ...any) error {
	return errorcode.ValidationError{Msg: fmt.Sprintf(format, args...)}
}

func validateWidget(spec json.RawMessage) error {
	var p panelSpecProbe
	if err := json.Unmarshal(spec, &p); err != nil {
		return invalid("spec must be a valid panel spec object")
	}
	switch {
	case strings.TrimSpace(p.ID) == "":
		return invalid("spec.id is required")
	case strings.TrimSpace(p.Title) == "":
		return invalid("spec.title is required")
	case p.Legend == nil || p.Smooth == nil:
		return invalid("spec.legend and spec.smooth are required")
	case !slices.Contains(panelTypes, p.PanelType):
		return invalid("spec.panelType %q is not supported", p.PanelType)
	case !slices.Contains(layoutVariants, p.LayoutVariant):
		return invalid("spec.layoutVariant %q is not supported", p.LayoutVariant)
	}
	if err := validateLayout(p); err != nil {
		return err
	}
	q := p.Query
	switch {
	case q == nil:
		return invalid("spec.query is required")
	case q.Kind != "metrics":
		return invalid("spec.query.kind %q is not supported; expected \"metrics\"", q.Kind)
	case !slices.Contains(steps, q.Step):
		return invalid("spec.query.step %q is not supported", q.Step)
	case !slices.Contains(spaceAggregations, q.SpaceAggregation):
		return invalid("spec.query.spaceAggregation %q is not supported", q.SpaceAggregation)
	}
	return validateBuilderQueries(q.Queries)
}

func validateLayout(p panelSpecProbe) error {
	l := p.Layout
	switch {
	case l == nil || l.X == nil || l.Y == nil || l.W == nil || l.H == nil:
		return invalid("spec.layout requires x, y, w and h")
	case *l.W <= 0 || *l.H <= 0:
		return invalid("spec.layout w and h must be positive")
	case *l.X < 0 || *l.Y < 0:
		return invalid("spec.layout x and y must not be negative")
	}
	return nil
}

func validateBuilderQueries(queries []builderQueryProbe) error {
	if len(queries) == 0 {
		return invalid("spec.query.queries must have at least one query")
	}
	for _, q := range queries {
		switch {
		case strings.TrimSpace(q.MetricName) == "":
			return invalid("spec.query.queries[].metricName is required")
		case !slices.Contains(aggregations, q.Aggregation):
			return invalid("aggregation %q is not supported", q.Aggregation)
		case !slices.Contains(spaceAggregations, q.SpaceAggregation):
			return invalid("spaceAggregation %q is not supported", q.SpaceAggregation)
		}
		for _, f := range q.Where {
			if !slices.Contains(filterOperators, f.Operator) {
				return invalid("filter operator %q is not supported", f.Operator)
			}
		}
	}
	return nil
}
