package evaluators

import (
	"net/http"

	"github.com/optikklabs/query/internal/shared/errorcode"
	httputil "github.com/optikklabs/query/internal/shared/httputil"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	startMs, endMs, ok := httputil.ParseRequiredRange(w, r)
	if !ok {
		return
	}
	res, err := h.svc.List(r.Context(), httputil.Tenant(r).TenantID, startMs, endMs)
	if err != nil {
		httputil.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.QueryFailed, "failed to list evaluators", err)
		return
	}
	httputil.RespondOK(w, map[string]any{"items": res})
}


func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req UpsertRequest
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

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := httputil.ParseIDParam(w, r, "id")
	if !ok {
		return
	}
	var req UpsertRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "invalid request body", nil)
		return
	}
	res, err := h.svc.Update(r.Context(), httputil.Tenant(r).TenantID, id, req)
	if err != nil {
		respondErr(w, r, err)
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
		respondErr(w, r, err)
		return
	}
	httputil.RespondOK(w, map[string]any{"deleted": id})
}

func respondErr(w http.ResponseWriter, r *http.Request, err error) {
	httputil.RespondServiceError(w, r, err, "evaluator request failed")
}
