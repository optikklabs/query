package database

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"testing"
)

func TestIsNoRows(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"sql.ErrNoRows", sql.ErrNoRows, true},
		{"wrapped sql.ErrNoRows", fmt.Errorf("query: %w", sql.ErrNoRows), true},
		{"io.EOF is a transport failure, not empty result", io.EOF, false},
		{"wrapped io.EOF", fmt.Errorf("read: %w", io.EOF), false},
		{"message containing 'no rows' is not empty result", errors.New("scan: no rows affected"), false},
		{"message containing 'EOF' is not empty result", errors.New("unexpected EOF"), false},
		{"other error", errors.New("connection refused"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isNoRows(tt.err); got != tt.want {
				t.Errorf("isNoRows(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
