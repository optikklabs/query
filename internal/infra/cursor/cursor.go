package cursor

import (
	"encoding/base64"
	"encoding/json"

	"github.com/optikklabs/query/internal/shared/contracts"
	"github.com/optikklabs/query/internal/shared/errorcode"
)

// Encode serializes a cursor struct; cursors hold only plain fields, so
// marshaling cannot fail.
func Encode[T any](cur T) string {
	b, _ := json.Marshal(cur) //nolint:errchkjson // cursor types are flat structs of strings, numbers and times
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

// ErrInvalid answers a page cursor this API did not issue.
var ErrInvalid = errorcode.ValidationError{Msg: "invalid cursor"}

// Decode returns the cursor raw encodes, nil for an absent cursor (the first
// page), or ErrInvalid.
func Decode[T any](raw string) (*T, error) {
	if raw == "" {
		return nil, nil //nolint:nilnil // a nil cursor is the documented first page
	}
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil, ErrInvalid
	}
	cur := new(T)
	if err := json.Unmarshal(b, cur); err != nil {
		return nil, ErrInvalid
	}
	return cur, nil
}
