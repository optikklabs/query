package users

import (
	"net/http"

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
		httputil.RespondServiceError(w, r, err, "Failed to query LLM users overview")
		return
	}
	httputil.RespondOK(w, resp)
}

func (h *Handler) Query(w http.ResponseWriter, r *http.Request) {
	var req UsersQueryRequest
	if !httputil.BindJSON(w, r, &req) {
		return
	}
	resp, err := h.svc.Query(r.Context(), httputil.Tenant(r).TenantID, req)
	if err != nil {
		httputil.RespondServiceError(w, r, err, "Failed to query LLM users")
		return
	}
	httputil.RespondOK(w, resp)
}
