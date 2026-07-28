package seriesgroup

type AggKind int

const (
	Gauge AggKind = iota

	Rate
)

type Def struct {
	ID          string
	MetricNames []string

	LabelSQL string
	Agg      AggKind

	Scale float64
}

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

func (c Catalog) Def(id string) (Def, bool) {
	d, ok := c.byID[id]
	return d, ok
}

func (c Catalog) MetricNames() []string { return c.metricNames }

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

func EmptyIfNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
