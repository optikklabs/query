package sessions

import (
	"net/http"

	"github.com/go-chi/chi/v5"
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
		httputil.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to query LLM sessions overview", err)
		return
	}
	httputil.RespondOK(w, resp)
}

func (h *Handler) Query(w http.ResponseWriter, r *http.Request) {
	var req SessionsQueryRequest
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
		httputil.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to query LLM sessions", err)
		return
	}
	httputil.RespondOK(w, resp)
}

func (h *Handler) Detail(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionId")
	if sessionID == "" {
		httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "sessionId is required", nil)
		return
	}
	startMs, endMs, ok := httputil.ParseRequiredExplicitRange(w, r)
	if !ok {
		return
	}
	resp, err := h.svc.Detail(r.Context(), httputil.Tenant(r).TenantID, sessionID, startMs, endMs)
	if err != nil {
		httputil.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to load LLM session", err)
		return
	}
	if len(resp.Turns) == 0 {
		httputil.RespondErrorWithCause(w, r, http.StatusNotFound, errorcode.NotFound, "Session not found", nil)
		return
	}
	httputil.RespondOK(w, resp)
}
