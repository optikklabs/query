package cpu

import (
	"context"
	"net/http"

	modulecommon "github.com/optikklabs/query/internal/shared/httputil"
)

type CPUHandler struct {
	Service *Service
}

func (h *CPUHandler) GetAvgCPU(w http.ResponseWriter, r *http.Request) {
	modulecommon.HandleRangeQuery(w, r, "Failed to query avg CPU", func(ctx context.Context, teamID, startMs, endMs int64) (any, error) {
		return h.Service.GetAvgCPU(ctx, teamID, startMs, endMs)
	})
}

func (h *CPUHandler) GetCPUByInstance(w http.ResponseWriter, r *http.Request) {
	modulecommon.HandleRangeQuery(w, r, "Failed to query CPU by instance", func(ctx context.Context, teamID, startMs, endMs int64) (any, error) {
		return h.Service.GetCPUByInstance(ctx, teamID, startMs, endMs)
	})
}
