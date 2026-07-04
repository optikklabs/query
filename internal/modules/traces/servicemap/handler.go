package servicemap

import (
	"net/http"

	"github.com/go-chi/chi/v5"
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
	traceID := chi.URLParam(r, "traceId")
	if traceID == "" {
		modulecommon.RespondError(w, r, http.StatusBadRequest, errorcode.Validation, "trace id required")
		return
	}
	resp, err := h.svc.GetServiceMap(r.Context(), tenantID, traceID)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to compute service map", err)
		return
	}
	modulecommon.RespondOK(w, resp)
}

func (h *Handler) GetTraceErrors(w http.ResponseWriter, r *http.Request) {
	tenantID := modulecommon.Tenant(r).TenantID
	traceID := chi.URLParam(r, "traceId")
	if traceID == "" {
		modulecommon.RespondError(w, r, http.StatusBadRequest, errorcode.Validation, "trace id required")
		return
	}
	groups, err := h.svc.GetTraceErrors(r.Context(), tenantID, traceID)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to fetch trace errors", err)
		return
	}
	modulecommon.RespondOK(w, groups)
}
