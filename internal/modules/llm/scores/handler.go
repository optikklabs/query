package scores

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

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateScoreRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "Invalid request body", nil)
		return
	}
	if err := h.svc.Create(r.Context(), httputil.Tenant(r).TenantID, req); err != nil {
		if IsValidationError(err) {
			httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, err.Error(), nil)
			return
		}
		httputil.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to save score", err)
		return
	}
	httputil.RespondOK(w, map[string]bool{"ok": true})
}

func (h *Handler) Names(w http.ResponseWriter, r *http.Request) {
	startMs, endMs, ok := httputil.ParseRequiredRange(w, r)
	if !ok {
		return
	}
	resp, err := h.svc.Names(r.Context(), httputil.Tenant(r).TenantID, startMs, endMs)
	if err != nil {
		httputil.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to query score names", err)
		return
	}
	httputil.RespondOK(w, resp)
}

func (h *Handler) Summary(w http.ResponseWriter, r *http.Request) {
	startMs, endMs, ok := httputil.ParseRequiredRange(w, r)
	if !ok {
		return
	}
	resp, err := h.svc.Summary(r.Context(), httputil.Tenant(r).TenantID, startMs, endMs)
	if err != nil {
		httputil.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to query score summary", err)
		return
	}
	httputil.RespondOK(w, resp)
}

func (h *Handler) Timeseries(w http.ResponseWriter, r *http.Request) {
	startMs, endMs, ok := httputil.ParseRequiredRange(w, r)
	if !ok {
		return
	}
	name := r.URL.Query().Get("name")
	if name == "" {
		httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "name is required", nil)
		return
	}
	resp, err := h.svc.Timeseries(r.Context(), httputil.Tenant(r).TenantID, startMs, endMs, name)
	if err != nil {
		httputil.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to query score timeseries", err)
		return
	}
	httputil.RespondOK(w, resp)
}

func (h *Handler) Distribution(w http.ResponseWriter, r *http.Request) {
	startMs, endMs, ok := httputil.ParseRequiredRange(w, r)
	if !ok {
		return
	}
	name := r.URL.Query().Get("name")
	if name == "" {
		httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "name is required", nil)
		return
	}
	resp, err := h.svc.Distribution(r.Context(), httputil.Tenant(r).TenantID, startMs, endMs, name)
	if err != nil {
		httputil.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to query score distribution", err)
		return
	}
	httputil.RespondOK(w, resp)
}
