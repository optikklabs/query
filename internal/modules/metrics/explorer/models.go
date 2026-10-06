package explorer

import (
	"encoding/json"
	"errors"
)

type TagValueResult struct {
	TagValue string `json:"tagValue"`
	Count    uint64 `json:"count"`
}

type TimeseriesPoint struct {
	TimestampMs int64
	Value       float64
}

type MetricNameEntry struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Unit        string `json:"unit,omitempty"`
	Description string `json:"description,omitempty"`
	Temporality string `json:"temporality"`
	IsMonotonic bool   `json:"isMonotonic"`
}

type MetricNamesResponse struct {
	Metrics []MetricNameEntry `json:"metrics"`
}

type TagEntry struct {
	Key    string   `json:"key"`
	Values []string `json:"values"`
}

type TagsResponse struct {
	Tags []TagEntry `json:"tags"`
}

type Filter struct {
	Key      string       `json:"key"`
	Operator string       `json:"operator"`
	Value    FilterValues `json:"value"`
}

var errFilterValue = errors.New("filter value must be a string or a list of strings")

// FilterValues is a filter's value: a single string or a list of strings.
type FilterValues []string

func (v *FilterValues) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return errFilterValue
	}
	var one string
	if err := json.Unmarshal(data, &one); err == nil {
		*v = FilterValues{one}
		return nil
	}
	var many []string
	if err := json.Unmarshal(data, &many); err != nil {
		return errFilterValue
	}
	*v = many
	return nil
}

type MetricQuery struct {
	ID          string   `json:"id"`
	Aggregation string   `json:"aggregation"`
	MetricName  string   `json:"metricName"`
	Where       []Filter `json:"where"`
	GroupBy     []string `json:"groupBy,omitempty"`
}

type QueryRequest struct {
	StartTime int64         `json:"startTime"`
	EndTime   int64         `json:"endTime"`
	Step      string        `json:"step"`
	Queries   []MetricQuery `json:"queries"`
}

type Series struct {
	Tags   map[string]string `json:"tags"`
	Values []*float64        `json:"values"`
}

type QueryResult struct {
	Timestamps []int64  `json:"timestamps"`
	Series     []Series `json:"series"`
}

type QueryResponse struct {
	Results map[string]QueryResult `json:"results"`
}
