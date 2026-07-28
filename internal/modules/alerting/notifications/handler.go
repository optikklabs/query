package notifications

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/optikklabs/query/internal/shared/errorcode"
	httputil "github.com/optikklabs/query/internal/shared/httputil"
)

type Handler struct {
	Service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{
		Service: service,
	}
}

func (h *Handler) ListChannels(w http.ResponseWriter, r *http.Request) {
	t := httputil.Tenant(r)
	res, err := h.Service.ListChannels(r.Context(), t.TenantID)
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	httputil.RespondOK(w, res)
}

func (h *Handler) GetChannel(w http.ResponseWriter, r *http.Request) {
	t := httputil.Tenant(r)
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	res, err := h.Service.GetChannel(r.Context(), t.TenantID, id)
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	httputil.RespondOK(w, res)
}

func (h *Handler) CreateChannel(w http.ResponseWriter, r *http.Request) {
	t := httputil.Tenant(r)
	var req CreateChannelRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "validation error", err)
		return
	}
	res, err := h.Service.CreateChannel(r.Context(), t.TenantID, req)
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	httputil.RespondOK(w, res)
}

func (h *Handler) UpdateChannel(w http.ResponseWriter, r *http.Request) {
	t := httputil.Tenant(r)
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	var req UpdateChannelRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "validation error", err)
		return
	}
	res, err := h.Service.UpdateChannel(r.Context(), t.TenantID, id, req)
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	httputil.RespondOK(w, res)
}

func (h *Handler) DeleteChannel(w http.ResponseWriter, r *http.Request) {
	t := httputil.Tenant(r)
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	if err := h.Service.DeleteChannel(r.Context(), t.TenantID, id); err != nil {
		respondServiceError(w, r, err)
		return
	}
	httputil.RespondOK(w, map[string]any{"deleted": id})
}

func (h *Handler) TestChannel(w http.ResponseWriter, r *http.Request) {
	t := httputil.Tenant(r)
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	res, err := h.Service.TestChannel(r.Context(), t.TenantID, id)
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	httputil.RespondOK(w, res)
}

func (h *Handler) ListPolicies(w http.ResponseWriter, r *http.Request) {
	t := httputil.Tenant(r)
	res, err := h.Service.ListPolicies(r.Context(), t.TenantID)
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	httputil.RespondOK(w, res)
}

func (h *Handler) CreatePolicy(w http.ResponseWriter, r *http.Request) {
	t := httputil.Tenant(r)
	var req CreatePolicyRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "validation error", err)
		return
	}
	res, err := h.Service.CreatePolicy(r.Context(), t.TenantID, req)
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	httputil.RespondOK(w, res)
}

func (h *Handler) UpdatePolicy(w http.ResponseWriter, r *http.Request) {
	t := httputil.Tenant(r)
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	var req UpdatePolicyRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "validation error", err)
		return
	}
	res, err := h.Service.UpdatePolicy(r.Context(), t.TenantID, id, req)
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	httputil.RespondOK(w, res)
}

func (h *Handler) DeletePolicy(w http.ResponseWriter, r *http.Request) {
	t := httputil.Tenant(r)
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	if err := h.Service.DeletePolicy(r.Context(), t.TenantID, id); err != nil {
		respondServiceError(w, r, err)
		return
	}
	httputil.RespondOK(w, map[string]any{"deleted": id})
}

func (h *Handler) ListTemplates(w http.ResponseWriter, r *http.Request) {
	t := httputil.Tenant(r)
	res, err := h.Service.ListTemplates(r.Context(), t.TenantID)
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	httputil.RespondOK(w, res)
}

func (h *Handler) CreateTemplate(w http.ResponseWriter, r *http.Request) {
	t := httputil.Tenant(r)
	var req CreateTemplateRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "validation error", err)
		return
	}
	res, err := h.Service.CreateTemplate(r.Context(), t.TenantID, req)
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	httputil.RespondOK(w, res)
}

func (h *Handler) UpdateTemplate(w http.ResponseWriter, r *http.Request) {
	t := httputil.Tenant(r)
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	var req UpdateTemplateRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "validation error", err)
		return
	}
	res, err := h.Service.UpdateTemplate(r.Context(), t.TenantID, id, req)
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	httputil.RespondOK(w, res)
}

func (h *Handler) DeleteTemplate(w http.ResponseWriter, r *http.Request) {
	t := httputil.Tenant(r)
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	if err := h.Service.DeleteTemplate(r.Context(), t.TenantID, id); err != nil {
		respondServiceError(w, r, err)
		return
	}
	httputil.RespondOK(w, map[string]any{"deleted": id})
}

func (h *Handler) ListIntegrations(w http.ResponseWriter, r *http.Request) {
	t := httputil.Tenant(r)
	res, err := h.Service.ListIntegrations(r.Context(), t.TenantID)
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	httputil.RespondOK(w, res)
}

func parseIDParam(w http.ResponseWriter, r *http.Request) (int64, bool) {
	raw := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.BadRequest, "invalid id", nil)
		return 0, false
	}
	return id, true
}

func respondServiceError(w http.ResponseWriter, r *http.Request, err error) {
	httputil.RespondServiceError(w, r, err, "request failed")
}
