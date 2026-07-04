package paths

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

func (h *Handler) GetCriticalPath(w http.ResponseWriter, r *http.Request) {
	tenantID := modulecommon.Tenant(r).TenantID
	traceID := chi.URLParam(r, "traceId")
	if traceID == "" {
		modulecommon.RespondError(w, r, http.StatusBadRequest, errorcode.Validation, "trace id required")
		return
	}
	path, err := h.svc.GetCriticalPath(r.Context(), tenantID, traceID)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to compute critical path", err)
		return
	}
	modulecommon.RespondOK(w, path)
}

func (h *Handler) GetErrorPath(w http.ResponseWriter, r *http.Request) {
	tenantID := modulecommon.Tenant(r).TenantID
	traceID := chi.URLParam(r, "traceId")
	if traceID == "" {
		modulecommon.RespondError(w, r, http.StatusBadRequest, errorcode.Validation, "trace id required")
		return
	}
	path, err := h.svc.GetErrorPath(r.Context(), tenantID, traceID)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to compute error path", err)
		return
	}
	modulecommon.RespondOK(w, path)
}
