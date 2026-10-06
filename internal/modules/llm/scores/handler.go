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
	if !httputil.BindJSON(w, r, &req) {
		return
	}
	if err := h.svc.Create(r.Context(), httputil.Tenant(r).TenantID, req); err != nil {
		httputil.RespondServiceError(w, r, err, "Failed to save score")
		return
	}
	httputil.RespondOK(w, map[string]bool{"ok": true})
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
