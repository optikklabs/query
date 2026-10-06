package datasets

import (
	"net/http"

	"github.com/optikklabs/query/internal/shared/errorcode"
	httputil "github.com/optikklabs/query/internal/shared/httputil"
)

type Handler struct {
	svc        *Service
	experiment *ExperimentService
}

func NewHandler(svc *Service, experiment *ExperimentService) *Handler {
	return &Handler{svc: svc, experiment: experiment}
}

func (h *Handler) RunExperiment(w http.ResponseWriter, r *http.Request) {
	if h.experiment == nil {
		httputil.RespondErrorWithCause(w, r, http.StatusServiceUnavailable, errorcode.Unavailable, "experiments require provider keys to be configured", nil)
		return
	}
	id, ok := httputil.ParseIDParam(w, r, "id")
	if !ok {
		return
	}
	var req RunExperimentRequest
	if !httputil.BindJSON(w, r, &req) {
		return
	}
	res, err := h.experiment.Run(r.Context(), httputil.Tenant(r).TenantID, id, req)
	if err != nil {
		httputil.RespondServiceError(w, r, err, "dataset request failed")
		return
	}
	httputil.RespondAccepted(w, res)
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	res, err := h.svc.List(r.Context(), httputil.Tenant(r).TenantID)
	if err != nil {
		httputil.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.QueryFailed, "failed to list datasets", err)
		return
	}
	httputil.RespondOK(w, map[string]any{"items": res})
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := httputil.ParseIDParam(w, r, "id")
	if !ok {
		return
	}
	res, err := h.svc.Get(r.Context(), httputil.Tenant(r).TenantID, id)
	if err != nil {
		httputil.RespondServiceError(w, r, err, "dataset request failed")
		return
	}
	httputil.RespondOK(w, res)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateDatasetRequest
	if !httputil.BindJSON(w, r, &req) {
		return
	}
	tenant := httputil.Tenant(r)
	res, err := h.svc.Create(r.Context(), tenant.TenantID, tenant.UserID, req)
	if err != nil {
		httputil.RespondServiceError(w, r, err, "dataset request failed")
		return
	}
	httputil.RespondOK(w, res)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := httputil.ParseIDParam(w, r, "id")
	if !ok {
		return
	}
	if err := h.svc.Delete(r.Context(), httputil.Tenant(r).TenantID, id); err != nil {
		httputil.RespondServiceError(w, r, err, "dataset request failed")
		return
	}
	httputil.RespondOK(w, map[string]any{"deleted": id})
}

func (h *Handler) AddItems(w http.ResponseWriter, r *http.Request) {
	id, ok := httputil.ParseIDParam(w, r, "id")
	if !ok {
		return
	}
	var req AddItemsRequest
	if !httputil.BindJSON(w, r, &req) {
		return
	}
	added, err := h.svc.AddItems(r.Context(), httputil.Tenant(r).TenantID, id, req)
	if err != nil {
		httputil.RespondServiceError(w, r, err, "dataset request failed")
		return
	}
	httputil.RespondOK(w, map[string]any{"added": added})
}
