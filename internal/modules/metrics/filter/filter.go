package filter

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/ClickHouse/clickhouse-go/v2"

	"github.com/optikklabs/query/internal/shared/filterutil"
)

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

// TagFilter matches a resource or attribute key with one of the Op*
// operators.
type TagFilter struct {
	Key      string
	Operator string
	Values   []string
}

// Tag filter operators, as the query builder sends them. Wildcard matches
// its single value with * standing for any run of characters.
const (
	OpEq       = "eq"
	OpNeq      = "neq"
	OpIn       = "in"
	OpNotIn    = "not_in"
	OpWildcard = "wildcard"
)

// singleValueOps take exactly one value; the others take one or more.
var singleValueOps = map[string]bool{OpEq: true, OpNeq: true, OpWildcard: true}

var validOperators = map[string]bool{OpEq: true, OpNeq: true, OpIn: true, OpNotIn: true, OpWildcard: true}

func negated(op string) bool { return op == OpNeq || op == OpNotIn }

var validAggregations = map[string]bool{
	"avg": true, "sum": true, "min": true, "max": true, "count": true,
	"p50": true, "p95": true, "p99": true,
	"rate": true,
}

// Validate checks one metric query. The time window is validated once per
// request by the explorer.
func (f *Filters) Validate() error {
	if f.MetricName == "" {
		return errors.New("metricName is required")
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
		switch {
		case !ValidKey(tag.Key):
			return errors.New("invalid metric filter key: " + tag.Key)
		case !validOperators[tag.Operator]:
			return errors.New("unsupported metric filter operator: " + tag.Operator)
		case len(tag.Values) == 0, singleValueOps[tag.Operator] && len(tag.Values) != 1:
			return fmt.Errorf("metric filter %s %s needs %s", tag.Key, tag.Operator, valueArity(tag.Operator))
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

// ValidKey reports whether key is a resource key or an attribute key made of
// [A-Za-z0-9._-]; keys are interpolated into SQL, so nothing else passes.
func ValidKey(key string) bool {
	return key != "" && (Canonical(key) != "" || !strings.ContainsFunc(key, invalidKeyRune))
}

func invalidKeyRune(r rune) bool {
	switch {
	case 'a' <= r && r <= 'z', 'A' <= r && r <= 'Z', '0' <= r && r <= '9', r == '.', r == '_', r == '-':
		return false
	default:
		return true
	}
}

func valueArity(op string) string {
	if singleValueOps[op] {
		return "exactly one value"
	}
	return "at least one value"
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
		canonical := Canonical(t.Key)
		if canonical != "" && t.Operator != OpWildcard {
			acc := resAccum[canonical]
			if acc == nil {
				acc = &resourceAccum{}
				resAccum[canonical] = acc
				resourceOrder = append(resourceOrder, canonical)
			}
			if negated(t.Operator) {
				acc.negative = append(acc.negative, t.Values...)
			} else {
				acc.positive = append(acc.positive, t.Values...)
			}
			continue
		}

		bind := "mf" + strconv.Itoa(rowIdx)
		rowIdx++
		if canonical != "" {
			resourceWhere += " AND " + canonical + " LIKE @" + bind
			args = append(args, clickhouse.Named(bind, filterutil.WildcardPattern(t.Values[0])))
			continue
		}
		column := AttrColumn(t.Key)
		var cond string
		switch t.Operator {
		case OpEq:
			cond, args = column+" = @"+bind, append(args, clickhouse.Named(bind, t.Values[0]))
		case OpNeq:
			cond, args = column+" != @"+bind, append(args, clickhouse.Named(bind, t.Values[0]))
		case OpIn:
			cond, args = column+" IN @"+bind, append(args, clickhouse.Named(bind, t.Values))
		case OpNotIn:
			cond, args = column+" NOT IN @"+bind, append(args, clickhouse.Named(bind, t.Values))
		default: // OpWildcard
			cond, args = column+" LIKE @"+bind, append(args, clickhouse.Named(bind, filterutil.WildcardPattern(t.Values[0])))
		}
		attrWhere += " AND mapContains(attributes, '" + t.Key + "') AND " + cond
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
