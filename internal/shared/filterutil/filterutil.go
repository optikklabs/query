package filterutil

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/optikklabs/query/internal/shared/errorcode"
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

// ValidateTimeRange checks a request window of Unix milliseconds.
func ValidateTimeRange(startMs, endMs int64) error {
	switch {
	case startMs <= 0 || endMs <= 0:
		return errorcode.ValidationError{Msg: "startTime and endTime must be positive Unix milliseconds"}
	case startMs >= endMs:
		return errorcode.ValidationError{Msg: "startTime must be before endTime"}
	case endMs-startMs > MaxTimeRangeMs:
		return errorcode.ValidationError{Msg: "time range must not exceed 30 days"}
	}
	return nil
}

func ValidateAttrs(attrs []AttrFilter) error {
	for _, af := range attrs {
		if strings.TrimSpace(af.Key) == "" {
			return errorcode.ValidationError{Msg: "attribute key is required"}
		}
		if _, ok := ValidAttrOps[af.Op]; !ok {
			return errorcode.ValidationError{Msg: fmt.Sprintf("unsupported attribute op %q", af.Op)}
		}
		switch af.Op {
		case "gt", "gte", "lt", "lte":
			if _, err := strconv.ParseFloat(af.Value, 64); err != nil {
				return errorcode.ValidationError{Msg: fmt.Sprintf("attribute %q: op %q requires a numeric value", af.Key, af.Op)}
			}
		case "regex":
			if _, err := regexp.Compile(af.Value); err != nil {
				return errorcode.ValidationError{Msg: fmt.Sprintf("attribute %q: invalid regex: %v", af.Key, err)}
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

// Limit resolves a requested page size: 0 (unset) means def, and anything
// outside [1, max] is a validation error.
func Limit(v, def, max int) (int, error) {
	switch {
	case v == 0:
		return def, nil
	case v < 0 || v > max:
		return 0, errorcode.ValidationError{Msg: fmt.Sprintf("limit must be between 1 and %d", max)}
	default:
		return v, nil
	}
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

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// LikeSubstringPattern is a LIKE pattern matching term anywhere in a
// lower-cased value.
func LikeSubstringPattern(term string) string {
	return "%" + likeEscaper.Replace(strings.ToLower(term)) + "%"
}

// WildcardPattern turns a glob where * matches any run of characters into
// a LIKE pattern; every other character matches literally.
func WildcardPattern(glob string) string {
	return strings.ReplaceAll(likeEscaper.Replace(glob), "*", "%")
}
