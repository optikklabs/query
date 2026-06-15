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
	modulecommon.HandleRangeQuery(w, r, "Failed to query avg memory", func(ctx context.Context, teamID, startMs, endMs int64) (any, error) {
		return h.Service.GetAvgMemory(ctx, teamID, startMs, endMs)
	})
}

func (h *MemoryHandler) GetMemoryByInstance(w http.ResponseWriter, r *http.Request) {
	teamID := modulecommon.Tenant(r).TeamID
	startMs, endMs, ok := modulecommon.ParseRequiredRange(w, r)
	if !ok {
		return
	}
	host := r.URL.Query().Get("host")
	pod := r.URL.Query().Get("pod")
	container := r.URL.Query().Get("container")
	serviceName := r.URL.Query().Get("serviceName")
	if serviceName == "" {
		modulecommon.RespondError(w, r, http.StatusBadRequest, errorcode.BadRequest, "serviceName is required")
		return
	}
	resp, err := h.Service.GetMemoryByInstance(r.Context(), teamID, host, pod, container, serviceName, startMs, endMs)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to query memory by instance", err)
		return
	}
	modulecommon.RespondOK(w, resp)
}
