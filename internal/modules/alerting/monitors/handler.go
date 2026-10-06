package monitors

import (
	"net/http"

	models "github.com/optikklabs/query/internal/modules/alerting/shared/models"
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

const (
	defaultListLimit  = 50
	defaultEventLimit = 20
	maxListLimit      = 200
)

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	q, ok := parseListQuery(w, r)
	if !ok {
		return
	}
	res, err := h.Service.List(r.Context(), httputil.Tenant(r).TenantID, q)
	if err != nil {
		httputil.RespondServiceError(w, r, err, "failed to list monitors")
		return
	}
	httputil.RespondOK(w, res)
}

func parseListQuery(w http.ResponseWriter, r *http.Request) (q ListQuery, ok bool) {
	q.Search = r.URL.Query().Get("q")
	if q.Statuses, ok = httputil.QueryEnums(w, r, "status", models.Statuses); !ok {
		return q, false
	}
	if q.Type, ok = httputil.QueryEnum(w, r, "type", models.SupportedMonitorTypes); !ok {
		return q, false
	}
	if q.Priority, ok = httputil.QueryEnum(w, r, "priority", models.SupportedPriorities); !ok {
		return q, false
	}
	if q.Muted, ok = httputil.QueryBool(w, r, "muted"); !ok {
		return q, false
	}
	if q.Limit, ok = httputil.QueryLimit(w, r, defaultListLimit, maxListLimit); !ok {
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

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	tenant := httputil.Tenant(r)
	id, ok := httputil.ParseIDParam(w, r, "id")
	if !ok {
		return
	}
	res, err := h.Service.GetByID(r.Context(), tenant.TenantID, id)
	if err != nil {
		httputil.RespondServiceError(w, r, err, "monitor request failed")
		return
	}
	httputil.RespondOK(w, res)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	tenant := httputil.Tenant(r)
	var req CreateMonitorRequest
	if !httputil.BindJSON(w, r, &req) {
		return
	}
	res, err := h.Service.Create(r.Context(), tenant.TenantID, tenant.UserID, req)
	if err != nil {
		httputil.RespondServiceError(w, r, err, "monitor request failed")
		return
	}
	httputil.RespondOK(w, res)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	tenant := httputil.Tenant(r)
	id, ok := httputil.ParseIDParam(w, r, "id")
	if !ok {
		return
	}
	var req UpdateMonitorRequest
	if !httputil.BindJSON(w, r, &req) {
		return
	}
	res, err := h.Service.Update(r.Context(), tenant.TenantID, tenant.UserID, id, req)
	if err != nil {
		httputil.RespondServiceError(w, r, err, "monitor request failed")
		return
	}
	httputil.RespondOK(w, res)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	tenant := httputil.Tenant(r)
	id, ok := httputil.ParseIDParam(w, r, "id")
	if !ok {
		return
	}
	if err := h.Service.Delete(r.Context(), tenant.TenantID, id); err != nil {
		httputil.RespondServiceError(w, r, err, "monitor request failed")
		return
	}
	httputil.RespondOK(w, map[string]any{"deleted": id})
}
