package querydetail

import "testing"

func TestClampExecutionsLimit(t *testing.T) {
	tests := []struct {
		input int
		want  int
	}{
		{input: 0, want: defaultExecutionsLimit},
		{input: -1, want: defaultExecutionsLimit},
		{input: 100, want: 100},
		{input: 1000, want: maxExecutionsLimit},
	}
	for _, tt := range tests {
		if got := clampExecutionsLimit(tt.input); got != tt.want {
			t.Errorf("clampExecutionsLimit(%d) = %d, want %d", tt.input, got, tt.want)
		}
	}
}
