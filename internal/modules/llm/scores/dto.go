package scores

type CreateScoreRequest struct {
	TraceID     string   `json:"traceId"`
	SpanID      string   `json:"spanId"`
	Name        string   `json:"name"`
	DataType    string   `json:"dataType"`
	Value       *float64 `json:"value"`
	StringValue string   `json:"stringValue"`
	Comment     string   `json:"comment"`
}

type ScoreSummary struct {
	Name     string  `json:"name"`
	DataType string  `json:"dataType"`
	Count    uint64  `json:"count"`
	Mean     float64 `json:"mean"`
}

type ScoreSummaryResponse struct {
	Summaries []ScoreSummary `json:"summaries"`
}

type summaryRow struct {
	Name     string  `ch:"name"`
	DataType string  `ch:"data_type"`
	Count    uint64  `ch:"cnt"`
	Mean     float64 `ch:"mean"`
}
