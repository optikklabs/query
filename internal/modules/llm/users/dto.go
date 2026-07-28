package users

import "time"

type UsersOverviewResponse struct {
	ActiveUsers      uint64  `json:"activeUsers"`
	AvgCostPerUser   float64 `json:"avgCostPerUser"`
	AvgTracesPerUser float64 `json:"avgTracesPerUser"`
	LowScoreUsers    uint64  `json:"lowScoreUsers"`
}

type User struct {
	UserID     string  `json:"userId"`
	TopService string  `json:"topService"`
	Traces     uint64  `json:"traces"`
	Tokens     uint64  `json:"tokens"`
	Cost       float64 `json:"cost"`
	AvgScore   float64 `json:"avgScore"`
	LastSeenMs int64   `json:"lastSeenMs"`
}

type UsersQueryRequest struct {
	StartTime int64 `json:"startTime"`
	EndTime   int64 `json:"endTime"`
	Limit     int   `json:"limit"`
}

type UsersQueryResponse struct {
	Users []User `json:"users"`
}

type userRow struct {
	UserID     string    `ch:"user_id"`
	TopService string    `ch:"top_service"`
	Traces     uint64    `ch:"traces"`
	Tokens     uint64    `ch:"tokens"`
	Cost       float64   `ch:"cost"`
	LastSeen   time.Time `ch:"last_seen"`
}

type overviewRow struct {
	ActiveUsers uint64  `ch:"active_users"`
	Traces      uint64  `ch:"traces"`
	Cost        float64 `ch:"cost"`
}

type userScoreRow struct {
	UserID string  `ch:"user_id"`
	Mean   float64 `ch:"mean"`
}
