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
	modulecommon.HandleRangeQuery(w, r, "Failed to query avg CPU", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetAvgCPU(ctx, tenantID, startMs, endMs)
	})
}

func (h *CPUHandler) GetCPUByInstance(w http.ResponseWriter, r *http.Request) {
	modulecommon.HandleRangeQuery(w, r, "Failed to query CPU by instance", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetCPUByInstance(ctx, tenantID, startMs, endMs)
	})
}
