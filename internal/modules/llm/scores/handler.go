package scores

import (
	"net/http"

	"github.com/optikklabs/query/internal/shared/errorcode"
	modulecommon "github.com/optikklabs/query/internal/shared/httputil"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateScoreRequest
	if err := modulecommon.DecodeJSON(r, &req); err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "Invalid request body", nil)
		return
	}
	if err := h.svc.Create(r.Context(), modulecommon.Tenant(r).TenantID, req); err != nil {
		if IsValidationError(err) {
			modulecommon.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, err.Error(), nil)
			return
		}
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to save score", err)
		return
	}
	modulecommon.RespondOK(w, map[string]bool{"ok": true})
}

func (h *Handler) Names(w http.ResponseWriter, r *http.Request) {
	startMs, endMs, ok := modulecommon.ParseRequiredRange(w, r)
	if !ok {
		return
	}
	resp, err := h.svc.Names(r.Context(), modulecommon.Tenant(r).TenantID, startMs, endMs)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to query score names", err)
		return
	}
	modulecommon.RespondOK(w, resp)
}

func (h *Handler) Summary(w http.ResponseWriter, r *http.Request) {
	startMs, endMs, ok := modulecommon.ParseRequiredRange(w, r)
	if !ok {
		return
	}
	resp, err := h.svc.Summary(r.Context(), modulecommon.Tenant(r).TenantID, startMs, endMs)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to query score summary", err)
		return
	}
	modulecommon.RespondOK(w, resp)
}

func (h *Handler) Timeseries(w http.ResponseWriter, r *http.Request) {
	startMs, endMs, ok := modulecommon.ParseRequiredRange(w, r)
	if !ok {
		return
	}
	name := r.URL.Query().Get("name")
	if name == "" {
		modulecommon.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "name is required", nil)
		return
	}
	resp, err := h.svc.Timeseries(r.Context(), modulecommon.Tenant(r).TenantID, startMs, endMs, name)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to query score timeseries", err)
		return
	}
	modulecommon.RespondOK(w, resp)
}

func (h *Handler) Distribution(w http.ResponseWriter, r *http.Request) {
	startMs, endMs, ok := modulecommon.ParseRequiredRange(w, r)
	if !ok {
		return
	}
	name := r.URL.Query().Get("name")
	if name == "" {
		modulecommon.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "name is required", nil)
		return
	}
	resp, err := h.svc.Distribution(r.Context(), modulecommon.Tenant(r).TenantID, startMs, endMs, name)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to query score distribution", err)
		return
	}
	modulecommon.RespondOK(w, resp)
}
