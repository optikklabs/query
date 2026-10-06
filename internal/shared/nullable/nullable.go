// Package nullable converts database NULL-aware values into optional fields.
package nullable

// Ptr returns &v when valid and nil otherwise, so a NULL column becomes an
// omitted JSON field.
func Ptr[T any](v T, valid bool) *T {
	if !valid {
		return nil
	}
	return &v
}

// OrEmpty returns s, or an empty slice when s is nil, so it encodes as []
// rather than null.
func OrEmpty[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}
