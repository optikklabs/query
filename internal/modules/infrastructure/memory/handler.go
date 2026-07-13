package memory

import (
	"context"
	"net/http"

	"github.com/optikklabs/query/internal/shared/errorcode"

	modulecommon "github.com/optikklabs/query/internal/shared/httputil"
)

type MemoryHandler struct {
	Service *Service
}

func (h *MemoryHandler) GetAvgMemory(w http.ResponseWriter, r *http.Request) {
	modulecommon.HandleRangeQuery(w, r, "Failed to query avg memory", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetAvgMemory(ctx, tenantID, startMs, endMs)
	})
}

func (h *MemoryHandler) GetMemoryByInstance(w http.ResponseWriter, r *http.Request) {
	tenantID := modulecommon.Tenant(r).TenantID
	startMs, endMs, ok := modulecommon.ParseRequiredRange(w, r)
	if !ok {
		return
	}
	host := r.URL.Query().Get("host")
	pod := r.URL.Query().Get("pod")
	container := r.URL.Query().Get("container")
	serviceName := r.URL.Query().Get("serviceName")
	if serviceName == "" {
		modulecommon.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.BadRequest, "serviceName is required", nil)
		return
	}
	resp, err := h.Service.GetMemoryByInstance(r.Context(), tenantID, host, pod, container, serviceName, startMs, endMs)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to query memory by instance", err)
		return
	}
	modulecommon.RespondOK(w, resp)
}
