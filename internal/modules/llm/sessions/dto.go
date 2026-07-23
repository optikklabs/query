package sessions

import "time"

// SessionsOverviewResponse is the KPI row of the Sessions tab.
type SessionsOverviewResponse struct {
	Sessions      uint64  `json:"sessions"`
	AvgTurns      float64 `json:"avgTurns"`
	AvgDurationMs float64 `json:"avgDurationMs"`
	AvgCost       float64 `json:"avgCost"`
}

// Session is one row of the Sessions table.
type Session struct {
	SessionID  string  `json:"sessionId"`
	Service    string  `json:"service"`
	UserID     string  `json:"userId"`
	Preview    string  `json:"preview"`
	Turns      uint64  `json:"turns"`
	DurationMs int64   `json:"durationMs"`
	Cost       float64 `json:"cost"`
	AvgScore   float64 `json:"avgScore"`
	LastMs     int64   `json:"lastMs"`
}

type SessionsQueryRequest struct {
	StartTime int64 `json:"startTime"`
	EndTime   int64 `json:"endTime"`
	Limit     int   `json:"limit"`
}

type SessionsQueryResponse struct {
	Sessions []Session `json:"sessions"`
}

// Turn is one request/response exchange within a session, linked to its trace.
type Turn struct {
	TraceID    string  `json:"traceId"`
	StartMs    int64   `json:"startMs"`
	DurationMs int64   `json:"durationMs"`
	Model      string  `json:"model"`
	UserText   string  `json:"userText"`
	OutputText string  `json:"outputText"`
	Cost       float64 `json:"cost"`
}

type SessionDetailResponse struct {
	SessionID string `json:"sessionId"`
	Service   string `json:"service"`
	UserID    string `json:"userId"`
	Turns     []Turn `json:"turns"`
}

// --- ClickHouse row structs ---

type sessionRow struct {
	SessionID  string    `ch:"session_id"`
	Service    string    `ch:"service"`
	UserID     string    `ch:"user_id"`
	Preview    string    `ch:"preview"`
	Turns      uint64    `ch:"turns"`
	DurationMs int64     `ch:"duration_ms"`
	Cost       float64   `ch:"cost"`
	LastTs     time.Time `ch:"last_ts"`
}

type overviewRow struct {
	Sessions   uint64  `ch:"sessions"`
	Turns      uint64  `ch:"turns"`
	DurationMs float64 `ch:"duration_ms"`
	Cost       float64 `ch:"cost"`
}

type sessionScoreRow struct {
	SessionID string  `ch:"session_id"`
	Mean      float64 `ch:"mean"`
}

type turnRow struct {
	TraceID    string    `ch:"trace_id"`
	Start      time.Time `ch:"start_ts"`
	DurationMs int64     `ch:"duration_ms"`
	Model      string    `ch:"model"`
	UserText   string    `ch:"user_text"`
	OutputText string    `ch:"output_text"`
	Cost       float64   `ch:"cost"`
}
