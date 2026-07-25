// Package seriesgroup owns the chartable metric-group model shared by the
// resource detail pages (host, container). A group is a set of metric names
// plus the SQL that names one series per datapoint label; the query that
// serves it differs only by which resource column scopes the series.
package seriesgroup

type AggKind int

const (
	// Gauge averages datapoint values per display bucket.
	Gauge AggKind = iota
	// Rate converts counters to per-second rates per display bucket.
	Rate
)

// Def describes one chartable metric group on a resource detail page.
type Def struct {
	ID          string
	MetricNames []string
	// LabelSQL evaluates against metrics_series rows to name each series.
	LabelSQL string
	Agg      AggKind
	// Scale multiplies values after aggregation (100 for 0..1 fractions).
	Scale float64
}

// Catalog is one page's ordered set of groups, indexed by API metric id.
type Catalog struct {
	defs        []Def
	byID        map[string]Def
	metricNames []string
}

func NewCatalog(defs []Def) Catalog {
	byID := make(map[string]Def, len(defs))
	var names []string
	for _, d := range defs {
		byID[d.ID] = d
		names = append(names, d.MetricNames...)
	}
	return Catalog{defs: defs, byID: byID, metricNames: names}
}

// Def resolves an API metric id to its definition.
func (c Catalog) Def(id string) (Def, bool) {
	d, ok := c.byID[id]
	return d, ok
}

// MetricNames is the union of metric names across all groups, used to detect
// which groups have data for a resource.
func (c Catalog) MetricNames() []string { return c.metricNames }

// GroupsFor maps present metric names to available group ids, preserving
// catalog order.
func (c Catalog) GroupsFor(present []string) []string {
	set := make(map[string]struct{}, len(present))
	for _, n := range present {
		set[n] = struct{}{}
	}
	var groups []string
	for _, d := range c.defs {
		for _, n := range d.MetricNames {
			if _, ok := set[n]; ok {
				groups = append(groups, d.ID)
				break
			}
		}
	}
	return groups
}

// EmptyIfNil keeps JSON array fields as [] instead of null.
func EmptyIfNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
