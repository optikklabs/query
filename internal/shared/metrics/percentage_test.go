package metrics

import "testing"

func TestPercentage(t *testing.T) {
	tests := []struct {
		name               string
		numerator, divisor uint64
		want               float64
	}{
		{name: "zero divisor", numerator: 3, divisor: 0, want: 0},
		{name: "zero numerator", numerator: 0, divisor: 4, want: 0},
		{name: "quarter", numerator: 1, divisor: 4, want: 25},
		{name: "all", numerator: 4, divisor: 4, want: 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Percentage(tt.numerator, tt.divisor); got != tt.want {
				t.Fatalf("Percentage(%d, %d) = %v, want %v", tt.numerator, tt.divisor, got, tt.want)
			}
		})
	}
}

func TestPercentageInt(t *testing.T) {
	if got := PercentageInt(1, 4); got != 25 {
		t.Fatalf("PercentageInt(1, 4) = %v, want 25", got)
	}
	if got := PercentageInt(3, 0); got != 0 {
		t.Fatalf("PercentageInt(3, 0) = %v, want 0", got)
	}
}
