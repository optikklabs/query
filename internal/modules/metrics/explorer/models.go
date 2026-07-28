package explorer

type MetricNameResult struct {
	MetricName  string `json:"metricName"`
	MetricType  string `json:"metricType"`
	Unit        string `json:"unit"`
	Description string `json:"description"`
}

type TagKeyResult struct {
	TagKey string `json:"tagKey"`
}

type TagValueResult struct {
	TagValue string `json:"tagValue"`
	Count    uint64 `json:"count"`
}

type TimeseriesPoint struct {
	Timestamp string  `json:"timestamp"`
	Value     float64 `json:"value"`
}

type FEMetricNameEntry struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Unit        string `json:"unit,omitempty"`
	Description string `json:"description,omitempty"`
}

type FEMetricNamesResponse struct {
	Metrics []FEMetricNameEntry `json:"metrics"`
}

type FETagEntry struct {
	Key    string   `json:"key"`
	Values []string `json:"values"`
}

type FETagsResponse struct {
	Tags []FETagEntry `json:"tags"`
}

type FEFilter struct {
	Key      string `json:"key"`
	Operator string `json:"operator"`
	Value    any    `json:"value"`
}

type FEMetricQuery struct {
	ID          string     `json:"id"`
	Aggregation string     `json:"aggregation"`
	MetricName  string     `json:"metricName"`
	Where       []FEFilter `json:"where"`
	GroupBy     []string   `json:"groupBy,omitempty"`
}

type FEQueryRequest struct {
	StartTime int64           `json:"startTime"`
	EndTime   int64           `json:"endTime"`
	Step      string          `json:"step"`
	Queries   []FEMetricQuery `json:"queries"`
}

type FESeries struct {
	Tags   map[string]string `json:"tags"`
	Values []*float64        `json:"values"`
}

type FEQueryResult struct {
	Timestamps []int64    `json:"timestamps"`
	Series     []FESeries `json:"series"`
}

type FEQueryResponse struct {
	Results map[string]FEQueryResult `json:"results"`
}
