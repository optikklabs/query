package dashboards

import (
	"net/http"

	"github.com/optikklabs/query/internal/shared/errorcode"
	httputil "github.com/optikklabs/query/internal/shared/httputil"
)

type Handler struct {
	Service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{Service: service}
}

const (
	defaultPageLimit = 50
	maxPageLimit     = 200
)

func (h *Handler) ListPages(w http.ResponseWriter, r *http.Request) {
	q, ok := parseListPagesQuery(w, r)
	if !ok {
		return
	}
	res, err := h.Service.ListPages(r.Context(), httputil.Tenant(r).TenantID, q)
	if err != nil {
		httputil.RespondServiceError(w, r, err, "failed to list dashboard pages")
		return
	}
	httputil.RespondOK(w, res)
}

func parseListPagesQuery(w http.ResponseWriter, r *http.Request) (q ListPagesQuery, ok bool) {
	q.Search = r.URL.Query().Get("q")
	q.Tag = r.URL.Query().Get("tag")
	favorite, ok := httputil.QueryBool(w, r, "favorite")
	if !ok {
		return q, false
	}
	q.Favorite = favorite != nil && *favorite
	if q.Limit, ok = httputil.QueryLimit(w, r, defaultPageLimit, maxPageLimit); !ok {
		return q, false
	}
	if q.Offset, ok = httputil.QueryInt(w, r, "offset", 0); !ok {
		return q, false
	}
	if q.Offset < 0 {
		httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "offset must not be negative", nil)
		return q, false
	}
	return q, true
}

func (h *Handler) GetPage(w http.ResponseWriter, r *http.Request) {
	tenant := httputil.Tenant(r)
	id, ok := httputil.ParseIDParam(w, r, "id")
	if !ok {
		return
	}
	res, err := h.Service.GetPageDetail(r.Context(), tenant.TenantID, id)
	if err != nil {
		httputil.RespondServiceError(w, r, err, "dashboard request failed")
		return
	}
	httputil.RespondOK(w, res)
}

func (h *Handler) CreatePage(w http.ResponseWriter, r *http.Request) {
	tenant := httputil.Tenant(r)
	var req CreatePageRequest
	if !httputil.BindJSON(w, r, &req) {
		return
	}
	res, err := h.Service.CreatePage(r.Context(), tenant.TenantID, tenant.UserID, req)
	if err != nil {
		httputil.RespondServiceError(w, r, err, "dashboard request failed")
		return
	}
	httputil.RespondOK(w, res)
}

func (h *Handler) UpdatePage(w http.ResponseWriter, r *http.Request) {
	tenant := httputil.Tenant(r)
	id, ok := httputil.ParseIDParam(w, r, "id")
	if !ok {
		return
	}
	var req UpdatePageRequest
	if !httputil.BindJSON(w, r, &req) {
		return
	}
	res, err := h.Service.UpdatePage(r.Context(), tenant.TenantID, tenant.UserID, id, req)
	if err != nil {
		httputil.RespondServiceError(w, r, err, "dashboard request failed")
		return
	}
	httputil.RespondOK(w, res)
}

func (h *Handler) DeletePage(w http.ResponseWriter, r *http.Request) {
	tenant := httputil.Tenant(r)
	id, ok := httputil.ParseIDParam(w, r, "id")
	if !ok {
		return
	}
	if err := h.Service.DeletePage(r.Context(), tenant.TenantID, id); err != nil {
		httputil.RespondServiceError(w, r, err, "dashboard request failed")
		return
	}
	httputil.RespondOK(w, map[string]any{"deleted": id})
}

func (h *Handler) CreateWidget(w http.ResponseWriter, r *http.Request) {
	tenant := httputil.Tenant(r)
	pageID, ok := httputil.ParseIDParam(w, r, "id")
	if !ok {
		return
	}
	var req CreateWidgetRequest
	if !httputil.BindJSON(w, r, &req) {
		return
	}
	res, err := h.Service.CreateWidget(r.Context(), tenant.TenantID, pageID, req)
	if err != nil {
		httputil.RespondServiceError(w, r, err, "dashboard request failed")
		return
	}
	httputil.RespondOK(w, res)
}

func (h *Handler) UpdateWidget(w http.ResponseWriter, r *http.Request) {
	tenant := httputil.Tenant(r)
	pageID, ok := httputil.ParseIDParam(w, r, "id")
	if !ok {
		return
	}
	widgetID, ok := httputil.ParseIDParam(w, r, "widgetId")
	if !ok {
		return
	}
	var req UpdateWidgetRequest
	if !httputil.BindJSON(w, r, &req) {
		return
	}
	res, err := h.Service.UpdateWidget(r.Context(), tenant.TenantID, pageID, widgetID, req)
	if err != nil {
		httputil.RespondServiceError(w, r, err, "dashboard request failed")
		return
	}
	httputil.RespondOK(w, res)
}

func (h *Handler) DeleteWidget(w http.ResponseWriter, r *http.Request) {
	tenant := httputil.Tenant(r)
	pageID, ok := httputil.ParseIDParam(w, r, "id")
	if !ok {
		return
	}
	widgetID, ok := httputil.ParseIDParam(w, r, "widgetId")
	if !ok {
		return
	}
	if err := h.Service.DeleteWidget(r.Context(), tenant.TenantID, pageID, widgetID); err != nil {
		httputil.RespondServiceError(w, r, err, "dashboard request failed")
		return
	}
	httputil.RespondOK(w, map[string]any{"deleted": widgetID})
}
