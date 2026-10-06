package service

import (
	"testing"

	"github.com/optikklabs/query/internal/modules/infrastructure/models"
)

func TestClassifyHost(t *testing.T) {
	for _, tc := range []struct {
		errRate, p99 float64
		want         models.HostStatus
	}{
		{0, 0, models.HostHealthy},
		{2, 999, models.HostHealthy},
		{2.1, 0, models.HostWarn},
		{0, 1000, models.HostWarn},
		{10, 0, models.HostWarn},
		{10.1, 0, models.HostError},
		{0, 2000, models.HostError},
	} {
		if got := classifyHost(tc.errRate, tc.p99); got != tc.want {
			t.Errorf("classifyHost(%v, %v) = %v, want %v", tc.errRate, tc.p99, got, tc.want)
		}
	}
}
