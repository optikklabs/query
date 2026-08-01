package filter

import (
	"errors"
	"strconv"
	"strings"

	"github.com/ClickHouse/clickhouse-go/v2"
)

const MaxTimeRangeMs = 30 * 24 * 60 * 60 * 1000

type Filters struct {
	TenantID int64
	StartMs  int64
	EndMs    int64

	MetricName  string
	Aggregation string
	Step        string
	GroupBy     []string

	Cumulative bool

	Histogram bool

	Tags []TagFilter
}

type TagFilter struct {
	Key      string
	Operator string
	Values   []string
}

var validAggregations = map[string]bool{
	"avg": true, "sum": true, "min": true, "max": true, "count": true,
	"p50": true, "p95": true, "p99": true,
	"rate": true,
}

func (f *Filters) Validate() error {
	if f.MetricName == "" {
		return errors.New("metricName is required")
	}
	if f.StartMs <= 0 || f.EndMs <= 0 {
		return errors.New("startTime and endTime are required")
	}
	if f.EndMs <= f.StartMs {
		return errors.New("endTime must be greater than startTime")
	}
	if f.EndMs-f.StartMs > MaxTimeRangeMs {
		return errors.New("time range must not exceed 30 days")
	}
	if f.Aggregation == "" {
		f.Aggregation = "avg"
	}
	if !validAggregations[f.Aggregation] {
		return errors.New("unsupported aggregation: " + f.Aggregation)
	}
	for _, key := range f.GroupBy {
		if !ValidKey(key) {
			return errors.New("invalid group-by key: " + key)
		}
	}
	for _, tag := range f.Tags {
		if !ValidKey(tag.Key) || !validOperators[tag.Operator] || len(tag.Values) == 0 {
			return errors.New("invalid metric filter: " + tag.Key)
		}
	}
	return nil
}

var resourceColumns = map[string]string{
	"service":                "service",
	"service.name":           "service",
	"host":                   "host",
	"host.name":              "host",
	"pod":                    "pod",
	"k8s.pod.name":           "pod",
	"container":              "container",
	"container.name":         "container",
	"environment":            "environment",
	"deployment.environment": "environment",
	"k8s_namespace":          "k8s_namespace",
	"k8s.namespace.name":     "k8s_namespace",
	"k8s.node.name":          "k8s_node",
	"cloud.provider":         "cloud_provider",
	"cloud.account.id":       "cloud_account",
	"cloud.region":           "cloud_region",
	"cloud.platform":         "cloud_platform",
}

func Canonical(key string) string {
	return resourceColumns[key]
}

func AttrColumn(key string) string {
	return "attributes['" + key + "']"
}

func ValidKey(key string) bool {
	return key != "" && (Canonical(key) != "" || strings.IndexFunc(key, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' ||
			r == '.' || r == '_' || r == '-')
	}) < 0)
}

var validOperators = map[string]bool{
	"=":      true,
	"!=":     true,
	"IN":     true,
	"NOT IN": true,
}

func BuildClauses(f Filters) (resourceWhere, attrWhere string, args []any) {
	type resourceAccum struct {
		positive []string
		negative []string
	}
	resAccum := make(map[string]*resourceAccum)
	var resourceOrder []string

	rowIdx := 0
	for _, t := range f.Tags {
		if canonical := Canonical(t.Key); canonical != "" {
			acc := resAccum[canonical]
			if acc == nil {
				acc = &resourceAccum{}
				resAccum[canonical] = acc
				resourceOrder = append(resourceOrder, canonical)
			}
			negated := t.Operator == "!=" || t.Operator == "NOT IN"
			if negated {
				acc.negative = append(acc.negative, t.Values...)
			} else {
				acc.positive = append(acc.positive, t.Values...)
			}
			continue
		}

		col := AttrColumn(t.Key)
		exists := "mapContains(attributes, '" + t.Key + "')"
		bind := "mf" + strconv.Itoa(rowIdx)
		rowIdx++
		switch t.Operator {
		case "=":
			attrWhere += " AND " + exists + " AND " + col + " = @" + bind
			args = append(args, clickhouse.Named(bind, t.Values[0]))
		case "!=":
			attrWhere += " AND " + exists + " AND " + col + " != @" + bind
			args = append(args, clickhouse.Named(bind, t.Values[0]))
		case "IN":
			attrWhere += " AND " + exists + " AND " + col + " IN @" + bind
			args = append(args, clickhouse.Named(bind, t.Values))
		case "NOT IN":
			attrWhere += " AND " + exists + " AND " + col + " NOT IN @" + bind
			args = append(args, clickhouse.Named(bind, t.Values))
		}
	}

	for i, col := range resourceOrder {
		acc, bind := resAccum[col], "mr"+strconv.Itoa(i)
		if len(acc.positive) > 0 {
			resourceWhere += " AND " + col + " IN @" + bind
			args = append(args, clickhouse.Named(bind, acc.positive))
		}
		if len(acc.negative) > 0 {
			resourceWhere += " AND " + col + " NOT IN @x" + bind
			args = append(args, clickhouse.Named("x"+bind, acc.negative))
		}
	}

	return resourceWhere, attrWhere, args
}
