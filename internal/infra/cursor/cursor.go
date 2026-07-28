package cursor

import (
	"encoding/base64"
	"encoding/json"

	"github.com/optikklabs/query/internal/shared/contracts"
)

func Encode[T any](cur T) string {
	b, err := json.Marshal(cur)
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// Paginate trims a limit+1 result set and builds cursor pagination info.
// encode maps the last visible row to the next-page cursor.
func Paginate[T any](rows []T, limit int, encode func(T) string) ([]T, contracts.PageInfo) {
	info := contracts.PageInfo{Limit: limit}
	if len(rows) > limit {
		rows = rows[:limit]
		info.HasMore = true
		if len(rows) > 0 {
			info.NextCursor = encode(rows[len(rows)-1])
		}
	}
	return rows, info
}

func Decode[T any](raw string) (T, bool) {
	var zero T
	if raw == "" {
		return zero, false
	}
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return zero, false
	}
	var cur T
	if err := json.Unmarshal(b, &cur); err != nil {
		return zero, false
	}
	return cur, true
}
