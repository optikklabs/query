package database

import (
	"errors"
	"testing"

	"github.com/go-sql-driver/mysql"
)

func TestIsDuplicateEntry(t *testing.T) {
	dup := &mysql.MySQLError{Number: 1062}
	if !IsDuplicateEntry(dup) || !IsDuplicateEntry(errors.Join(errors.New("wrap"), dup)) {
		t.Fatal("duplicate entry not detected")
	}
	if IsDuplicateEntry(&mysql.MySQLError{Number: 1452}) || IsDuplicateEntry(nil) {
		t.Fatal("false positive")
	}
}
