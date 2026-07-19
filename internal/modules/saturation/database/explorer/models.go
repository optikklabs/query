package explorer

type DatastoreSystemRow struct {
	System            string  `json:"system"`
	Category          string  `json:"category"`
	QueryCount        int64   `json:"queryCount"`
	AvgLatencyMs      float64 `json:"avgLatencyMs"`
	P95LatencyMs      float64 `json:"p95LatencyMs"`
	ErrorRate         float64 `json:"errorRate"`
	ActiveConnections int64   `json:"activeConnections"`
	ServerHint        string  `json:"serverHint"`
	LastSeen          string  `json:"lastSeen"`
}
