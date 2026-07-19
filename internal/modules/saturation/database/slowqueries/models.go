package slowqueries

type SlowQueryPattern struct {
	QueryHash      string   `json:"queryHash"`
	QueryText      string   `json:"queryText"`
	DBSystem       string   `json:"dbSystem"`
	CollectionName string   `json:"collectionName"`
	Namespace      string   `json:"namespace"`
	Server         string   `json:"server"`
	P50Ms          *float64 `json:"p50Ms"`
	P95Ms          *float64 `json:"p95Ms"`
	P99Ms          *float64 `json:"p99Ms"`
	CallCount      int64    `json:"callCount"`
	ErrorCount     int64    `json:"errorCount"`
}
