package users

import (
	"net/http"

	"github.com/optikklabs/query/internal/shared/errorcode"
	"github.com/optikklabs/query/internal/shared/httputil"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Overview(w http.ResponseWriter, r *http.Request) {
	startMs, endMs, ok := httputil.ParseRequiredRange(w, r)
	if !ok {
		return
	}
	resp, err := h.svc.Overview(r.Context(), httputil.Tenant(r).TenantID, startMs, endMs)
	if err != nil {
		httputil.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to query LLM users overview", err)
		return
	}
	httputil.RespondOK(w, resp)
}

func (h *Handler) Query(w http.ResponseWriter, r *http.Request) {
	var req UsersQueryRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "Invalid request body", nil)
		return
	}
	if req.StartTime <= 0 || req.EndTime <= 0 || req.StartTime >= req.EndTime {
		httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "Valid startTime and endTime are required", nil)
		return
	}
	resp, err := h.svc.Query(r.Context(), httputil.Tenant(r).TenantID, req)
	if err != nil {
		httputil.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to query LLM users", err)
		return
	}
	httputil.RespondOK(w, resp)
}
