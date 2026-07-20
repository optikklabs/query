// Package filterutil contains shared filter types and validation functions for telemetry search queries.
package filterutil

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const MaxTimeRangeMs int64 = 30 * 24 * 60 * 60 * 1000

// AttrFilter represents an attribute key-value comparison predicate.
type AttrFilter struct {
	Key   string `json:"key"`
	Op    string `json:"op,omitempty"`
	Value string `json:"value"`
}

// ValidAttrOps defines the supported operators for attribute filtering.
var ValidAttrOps = map[string]struct{}{
	"": {}, "eq": {}, "neq": {}, "contains": {}, "regex": {},
	"gt": {}, "gte": {}, "lt": {}, "lte": {}, "exists": {}, "not_exists": {},
}

// ValidateTimeRange validates and normalizes start and end timestamps.
func ValidateTimeRange(startMs, endMs *int64) error {
	if *endMs <= 0 {
		*endMs = time.Now().UnixMilli()
	}
	if *startMs <= 0 {
		return errors.New("filters: startTime is required")
	}
	if *endMs <= *startMs {
		return errors.New("filters: endTime must be after startTime")
	}
	if (*endMs - *startMs) > MaxTimeRangeMs {
		*startMs = *endMs - MaxTimeRangeMs
	}
	return nil
}

// ValidateAttrs validates attribute filter keys, operators, numeric values, and regexes.
func ValidateAttrs(attrs []AttrFilter) error {
	for _, af := range attrs {
		if strings.TrimSpace(af.Key) == "" {
			return errors.New("filters: attribute key is required")
		}
		if _, ok := ValidAttrOps[af.Op]; !ok {
			return fmt.Errorf("filters: unsupported attribute op %q", af.Op)
		}
		switch af.Op {
		case "gt", "gte", "lt", "lte":
			if _, err := strconv.ParseFloat(af.Value, 64); err != nil {
				return fmt.Errorf("filters: attribute %q: op %q requires a numeric value", af.Key, af.Op)
			}
		case "regex":
			if _, err := regexp.Compile(af.Value); err != nil {
				return fmt.Errorf("filters: attribute %q: invalid regex: %v", af.Key, err)
			}
		}
	}
	return nil
}

// CmpSQL converts a comparison operator string to the corresponding SQL symbol.
// Shared by both logs and traces filter clause builders.
func CmpSQL(op string) string {
	switch op {
	case "gt":
		return ">"
	case "gte":
		return ">="
	case "lt":
		return "<"
	default:
		return "<="
	}
}

// PickLimit clamps a user-supplied limit to [1, max] with a default.
// Unifies the duplicated pickSuggestLimit and pickExplorerLimit helpers.
func PickLimit(v, def, max int) int {
	if v <= 0 {
		return def
	}
	if v > max {
		return max
	}
	return v
}

// SuggestRequest is the shared wire payload for suggest endpoints.
type SuggestRequest struct {
	StartTime int64  `json:"startTime"`
	EndTime   int64  `json:"endTime"`
	Field     string `json:"field"`
	Prefix    string `json:"prefix"`
	Limit     int    `json:"limit"`
}

// SuggestResponse is the shared wire response for suggest endpoints.
type SuggestResponse struct {
	Suggestions []Suggestion `json:"suggestions"`
}

// Suggestion is a single value+count pair from suggest endpoints.
type Suggestion struct {
	Value string `json:"value"`
	Count uint64 `json:"count"`
}

// SuggestionRow is the ClickHouse scan target for suggest queries.
type SuggestionRow struct {
	Value string `ch:"value"`
	Count uint64 `ch:"count"`
}

// MapSuggestionRows converts ClickHouse scan rows to API response items.
func MapSuggestionRows(rows []SuggestionRow) []Suggestion {
	out := make([]Suggestion, len(rows))
	for i, row := range rows {
		out[i] = Suggestion{Value: row.Value, Count: row.Count}
	}
	return out
}
