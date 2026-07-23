package sessions

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
	return &Handler{svc: svc}
}

func (h *Handler) Overview(w http.ResponseWriter, r *http.Request) {
	startMs, endMs, ok := modulecommon.ParseRequiredRange(w, r)
	if !ok {
		return
	}
	resp, err := h.svc.Overview(r.Context(), modulecommon.Tenant(r).TenantID, startMs, endMs)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to query LLM sessions overview", err)
		return
	}
	modulecommon.RespondOK(w, resp)
}

func (h *Handler) Query(w http.ResponseWriter, r *http.Request) {
	var req SessionsQueryRequest
	if err := modulecommon.DecodeJSON(r, &req); err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "Invalid request body", nil)
		return
	}
	if req.StartTime <= 0 || req.EndTime <= 0 || req.StartTime >= req.EndTime {
		modulecommon.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "Valid startTime and endTime are required", nil)
		return
	}
	resp, err := h.svc.Query(r.Context(), modulecommon.Tenant(r).TenantID, req)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to query LLM sessions", err)
		return
	}
	modulecommon.RespondOK(w, resp)
}

func (h *Handler) Detail(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionId")
	if sessionID == "" {
		modulecommon.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "sessionId is required", nil)
		return
	}
	startMs, endMs, ok := modulecommon.ParseRequiredExplicitRange(w, r)
	if !ok {
		return
	}
	resp, err := h.svc.Detail(r.Context(), modulecommon.Tenant(r).TenantID, sessionID, startMs, endMs)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to load LLM session", err)
		return
	}
	if len(resp.Turns) == 0 {
		modulecommon.RespondErrorWithCause(w, r, http.StatusNotFound, errorcode.NotFound, "Session not found", nil)
		return
	}
	modulecommon.RespondOK(w, resp)
}
