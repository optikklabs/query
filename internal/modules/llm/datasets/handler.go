package datasets

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

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
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var req RunExperimentRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "invalid request body", nil)
		return
	}
	res, err := h.experiment.Run(r.Context(), httputil.Tenant(r).TenantID, id, req)
	if err != nil {
		respondErr(w, r, err)
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
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	res, err := h.svc.Get(r.Context(), httputil.Tenant(r).TenantID, id)
	if err != nil {
		respondErr(w, r, err)
		return
	}
	httputil.RespondOK(w, res)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateDatasetRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "invalid request body", nil)
		return
	}
	tenant := httputil.Tenant(r)
	res, err := h.svc.Create(r.Context(), tenant.TenantID, tenant.UserID, req)
	if err != nil {
		respondErr(w, r, err)
		return
	}
	httputil.RespondOK(w, res)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	if err := h.svc.Delete(r.Context(), httputil.Tenant(r).TenantID, id); err != nil {
		respondErr(w, r, err)
		return
	}
	httputil.RespondOK(w, map[string]any{"deleted": id})
}

func (h *Handler) AddItems(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var req AddItemsRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "invalid request body", nil)
		return
	}
	added, err := h.svc.AddItems(r.Context(), httputil.Tenant(r).TenantID, id, req)
	if err != nil {
		respondErr(w, r, err)
		return
	}
	httputil.RespondOK(w, map[string]any{"added": added})
}

func (h *Handler) GetRun(w http.ResponseWriter, r *http.Request) {
	runID, ok := parseID(w, r, "runId")
	if !ok {
		return
	}
	res, err := h.svc.GetRun(r.Context(), httputil.Tenant(r).TenantID, runID)
	if err != nil {
		respondErr(w, r, err)
		return
	}
	httputil.RespondOK(w, res)
}

func parseID(w http.ResponseWriter, r *http.Request, key string) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, key), 10, 64)
	if err != nil || id <= 0 {
		httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.BadRequest, "invalid "+key, nil)
		return 0, false
	}
	return id, true
}

func respondErr(w http.ResponseWriter, r *http.Request, err error) {
	if IsProviderUnavailable(err) {
		httputil.RespondErrorWithCause(w, r, http.StatusServiceUnavailable, errorcode.Unavailable, "no provider key configured for this provider", nil)
		return
	}
	httputil.RespondServiceError(w, r, err, "dataset request failed")
}
