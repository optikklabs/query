package monitors

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/optikklabs/query/internal/modules/alerting/shared/query"
	"github.com/optikklabs/query/internal/shared/errorcode"
	httputil "github.com/optikklabs/query/internal/shared/httputil"
)

type Handler struct {
	Service *Service
	Queries query.Registry
}

func NewHandler(service *Service, queries query.Registry) *Handler {
	return &Handler{
		Service: service,
		Queries: queries,
	}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	tenant := httputil.Tenant(r)
	q := ListQuery{
		Status:   r.URL.Query().Get("status"),
		Type:     r.URL.Query().Get("type"),
		Priority: r.URL.Query().Get("priority"),
		Search:   r.URL.Query().Get("q"),
		Limit:    httputil.ParseIntParam(r, "limit", 50),
		Offset:   httputil.ParseIntParam(r, "offset", 0),
	}
	if mv := r.URL.Query().Get("muted"); mv != "" {
		b := mv == "true" || mv == "1"
		q.Muted = &b
	}
	res, err := h.Service.List(r.Context(), tenant.TenantID, q)
	if err != nil {
		httputil.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.QueryFailed, "failed to list monitors", err)
		return
	}
	httputil.RespondOK(w, res)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	tenant := httputil.Tenant(r)
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	res, err := h.Service.GetByID(r.Context(), tenant.TenantID, id)
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	httputil.RespondOK(w, res)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	tenant := httputil.Tenant(r)
	var req CreateMonitorRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "validation error", err)
		return
	}
	res, err := h.Service.Create(r.Context(), tenant.TenantID, tenant.UserID, req)
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	httputil.RespondOK(w, res)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	tenant := httputil.Tenant(r)
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	var req UpdateMonitorRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "validation error", err)
		return
	}
	res, err := h.Service.Update(r.Context(), tenant.TenantID, tenant.UserID, id, req)
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	httputil.RespondOK(w, res)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	tenant := httputil.Tenant(r)
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	if err := h.Service.Delete(r.Context(), tenant.TenantID, id); err != nil {
		respondServiceError(w, r, err)
		return
	}
	httputil.RespondOK(w, map[string]any{"deleted": id})
}

func parseIDParam(w http.ResponseWriter, r *http.Request) (int64, bool) {
	raw := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.BadRequest, "invalid monitor id", nil)
		return 0, false
	}
	return id, true
}

func respondServiceError(w http.ResponseWriter, r *http.Request, err error) {
	var ve ErrValidation
	switch {
	case errors.Is(err, ErrNotFound):
		httputil.RespondErrorWithCause(w, r, http.StatusNotFound, errorcode.NotFound, "monitor not found", nil)
	case errors.As(err, &ve):
		httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "validation error", ve)
	default:
		httputil.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "monitor request failed", err)
	}
}
