// Package sqljson maps Go values to MySQL JSON columns.
package sqljson

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// Scan decodes a JSON column value into dst; SQL NULL leaves dst untouched.
func Scan(src, dst any) error {
	switch v := src.(type) {
	case nil:
		return nil
	case []byte:
		return json.Unmarshal(v, dst)
	case string:
		return json.Unmarshal([]byte(v), dst)
	default:
		return fmt.Errorf("sqljson: cannot scan %T into %T", src, dst)
	}
}

// StringList is a JSON array-of-strings column; a nil list is stored as [].
type StringList []string

func (l *StringList) Scan(src any) error { return Scan(src, (*[]string)(l)) }

func (l StringList) Value() (driver.Value, error) {
	if l == nil {
		return []byte("[]"), nil
	}
	return json.Marshal([]string(l))
}
