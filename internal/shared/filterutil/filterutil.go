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

type AttrFilter struct {
	Key   string `json:"key"`
	Op    string `json:"op,omitempty"`
	Value string `json:"value"`
}

var ValidAttrOps = map[string]struct{}{
	"": {}, "eq": {}, "neq": {}, "contains": {}, "regex": {},
	"gt": {}, "gte": {}, "lt": {}, "lte": {}, "exists": {}, "not_exists": {},
}

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
		return errors.New("filters: time range must not exceed 30 days")
	}
	return nil
}

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

func PickLimit(v, def, max int) int {
	if v <= 0 {
		return def
	}
	if v > max {
		return max
	}
	return v
}

type SuggestRequest struct {
	StartTime int64  `json:"startTime"`
	EndTime   int64  `json:"endTime"`
	Field     string `json:"field"`
	Prefix    string `json:"prefix"`
	Limit     int    `json:"limit"`
}

type SuggestResponse struct {
	Suggestions []Suggestion `json:"suggestions"`
}

type Suggestion struct {
	Value string `json:"value"`
	Count uint64 `json:"count"`
}

type SuggestionRow struct {
	Value string `ch:"value"`
	Count uint64 `ch:"count"`
}

func MapSuggestionRows(rows []SuggestionRow) []Suggestion {
	out := make([]Suggestion, len(rows))
	for i, row := range rows {
		out[i] = Suggestion(row)
	}
	return out
}

func MapOperator(op string) string {
	switch op {
	case "eq":
		return "="
	case "neq":
		return "!="
	case "in":
		return "IN"
	case "not_in":
		return "NOT IN"
	default:
		return op
	}
}

func ExtractValues(v any) []string {
	switch val := v.(type) {
	case string:
		return []string{val}
	case []any:
		out := make([]string, 0, len(val))
		for _, item := range val {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return val
	default:
		if s := fmt.Sprint(v); s != "" {
			return []string{s}
		}
		return nil
	}
}

func LikeSubstringPattern(term string) string {
	esc := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(strings.ToLower(term))
	return "%" + esc + "%"
}
