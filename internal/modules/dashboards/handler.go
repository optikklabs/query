package dashboards

import (
	"errors"
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
	return &Handler{Service: service}
}

func (h *Handler) ListPages(w http.ResponseWriter, r *http.Request) {
	tenant := httputil.Tenant(r)
	q := ListPagesQuery{
		Search: r.URL.Query().Get("q"),
		Tag:    r.URL.Query().Get("tag"),
		Limit:  httputil.ParseIntParam(r, "limit", 50),
		Offset: httputil.ParseIntParam(r, "offset", 0),
	}
	if fv := r.URL.Query().Get("favorite"); fv == "true" || fv == "1" {
		q.Favorite = true
	}
	res, err := h.Service.ListPages(r.Context(), tenant.TenantID, q)
	if err != nil {
		httputil.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.QueryFailed, "failed to list dashboard pages", err)
		return
	}
	httputil.RespondOK(w, res)
}

func (h *Handler) GetPage(w http.ResponseWriter, r *http.Request) {
	tenant := httputil.Tenant(r)
	id, ok := parseIDParam(w, r, "id")
	if !ok {
		return
	}
	res, err := h.Service.GetPageDetail(r.Context(), tenant.TenantID, id)
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	httputil.RespondOK(w, res)
}

func (h *Handler) CreatePage(w http.ResponseWriter, r *http.Request) {
	tenant := httputil.Tenant(r)
	var req CreatePageRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "validation error", err)
		return
	}
	res, err := h.Service.CreatePage(r.Context(), tenant.TenantID, tenant.UserID, req)
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	httputil.RespondOK(w, res)
}

func (h *Handler) UpdatePage(w http.ResponseWriter, r *http.Request) {
	tenant := httputil.Tenant(r)
	id, ok := parseIDParam(w, r, "id")
	if !ok {
		return
	}
	var req UpdatePageRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "validation error", err)
		return
	}
	res, err := h.Service.UpdatePage(r.Context(), tenant.TenantID, tenant.UserID, id, req)
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	httputil.RespondOK(w, res)
}

func (h *Handler) DeletePage(w http.ResponseWriter, r *http.Request) {
	tenant := httputil.Tenant(r)
	id, ok := parseIDParam(w, r, "id")
	if !ok {
		return
	}
	if err := h.Service.DeletePage(r.Context(), tenant.TenantID, id); err != nil {
		respondServiceError(w, r, err)
		return
	}
	httputil.RespondOK(w, map[string]any{"deleted": id})
}

func (h *Handler) ListWidgets(w http.ResponseWriter, r *http.Request) {
	tenant := httputil.Tenant(r)
	pageID, ok := parseIDParam(w, r, "id")
	if !ok {
		return
	}
	res, err := h.Service.ListWidgets(r.Context(), tenant.TenantID, pageID)
	if err != nil {
		httputil.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.QueryFailed, "failed to list widgets", err)
		return
	}
	httputil.RespondOK(w, map[string]any{"items": res})
}

func (h *Handler) CreateWidget(w http.ResponseWriter, r *http.Request) {
	tenant := httputil.Tenant(r)
	pageID, ok := parseIDParam(w, r, "id")
	if !ok {
		return
	}
	var req CreateWidgetRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "validation error", err)
		return
	}
	res, err := h.Service.CreateWidget(r.Context(), tenant.TenantID, pageID, req)
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	httputil.RespondOK(w, res)
}

func (h *Handler) UpdateWidget(w http.ResponseWriter, r *http.Request) {
	tenant := httputil.Tenant(r)
	pageID, ok := parseIDParam(w, r, "id")
	if !ok {
		return
	}
	widgetID, ok := parseIDParam(w, r, "widgetId")
	if !ok {
		return
	}
	var req UpdateWidgetRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "validation error", err)
		return
	}
	res, err := h.Service.UpdateWidget(r.Context(), tenant.TenantID, pageID, widgetID, req)
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	httputil.RespondOK(w, res)
}

func (h *Handler) DeleteWidget(w http.ResponseWriter, r *http.Request) {
	tenant := httputil.Tenant(r)
	pageID, ok := parseIDParam(w, r, "id")
	if !ok {
		return
	}
	widgetID, ok := parseIDParam(w, r, "widgetId")
	if !ok {
		return
	}
	if err := h.Service.DeleteWidget(r.Context(), tenant.TenantID, pageID, widgetID); err != nil {
		respondServiceError(w, r, err)
		return
	}
	httputil.RespondOK(w, map[string]any{"deleted": widgetID})
}

func parseIDParam(w http.ResponseWriter, r *http.Request, key string) (int64, bool) {
	raw := chi.URLParam(r, key)
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.BadRequest, "invalid "+key, nil)
		return 0, false
	}
	return id, true
}

func respondServiceError(w http.ResponseWriter, r *http.Request, err error) {
	var ve ErrValidation
	switch {
	case errors.Is(err, ErrNotFound):
		httputil.RespondErrorWithCause(w, r, http.StatusNotFound, errorcode.NotFound, "dashboard not found", nil)
	case errors.As(err, &ve):
		httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "validation error", ve)
	default:
		httputil.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "dashboard request failed", err)
	}
}
