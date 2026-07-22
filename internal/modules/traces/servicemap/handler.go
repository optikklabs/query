package servicemap

import (
	"net/http"

	"github.com/optikklabs/query/internal/shared/errorcode"
	modulecommon "github.com/optikklabs/query/internal/shared/httputil"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{
		svc: svc,
	}
}

func (h *Handler) GetServiceMap(w http.ResponseWriter, r *http.Request) {
	tenantID := modulecommon.Tenant(r).TenantID
	traceID := modulecommon.URLParamLower(r, "traceId")
	if traceID == "" {
		modulecommon.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "trace id required", nil)
		return
	}
	startMs, endMs, ok := modulecommon.ParseRequiredRange(w, r)
	if !ok {
		return
	}
	resp, err := h.svc.GetServiceMap(r.Context(), tenantID, traceID, startMs, endMs)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to compute service map", err)
		return
	}
	modulecommon.RespondOK(w, resp)
}

func (h *Handler) GetTraceErrors(w http.ResponseWriter, r *http.Request) {
	tenantID := modulecommon.Tenant(r).TenantID
	traceID := modulecommon.URLParamLower(r, "traceId")
	if traceID == "" {
		modulecommon.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "trace id required", nil)
		return
	}
	startMs, endMs, ok := modulecommon.ParseRequiredRange(w, r)
	if !ok {
		return
	}
	groups, err := h.svc.GetTraceErrors(r.Context(), tenantID, traceID, startMs, endMs)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to fetch trace errors", err)
		return
	}
	modulecommon.RespondOK(w, groups)
}
