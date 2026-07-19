package slowqueries

type SlowQueryPattern struct {
	QueryHash      string   `json:"query_hash"`
	QueryText      string   `json:"query_text"`
	DBSystem       string   `json:"db_system"`
	CollectionName string   `json:"collection_name"`
	Namespace      string   `json:"namespace"`
	Server         string   `json:"server"`
	P50Ms          *float64 `json:"p50_ms"`
	P95Ms          *float64 `json:"p95_ms"`
	P99Ms          *float64 `json:"p99_ms"`
	CallCount      int64    `json:"call_count"`
	ErrorCount     int64    `json:"error_count"`
}
