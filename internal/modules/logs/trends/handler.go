package logtrends //nolint:revive,stylecheck

import (
	"net/http"

	"github.com/optikklabs/query/internal/shared/errorcode"
	modulecommon "github.com/optikklabs/query/internal/shared/httputil"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{
		svc: svc,
	}
}

// Summary powers POST /api/v1/logs/summary — total / errors / warns counts.
func (h *Handler) Summary(w http.ResponseWriter, r *http.Request) {
	var req Request
	if !modulecommon.BindFiltered(w, r, &req) {
		return
	}
	sum, err := h.svc.Summary(r.Context(), req.Filters)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to query logs summary", err)
		return
	}
	modulecommon.RespondOK(w, SummaryResponse{Summary: sum})
}

func (h *Handler) Trend(w http.ResponseWriter, r *http.Request) {
	var req Request
	if !modulecommon.BindFiltered(w, r, &req) {
		return
	}
	tr, err := h.svc.Trend(r.Context(), req.Filters)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to query logs trend", err)
		return
	}
	modulecommon.RespondOK(w, TrendResponse{Trend: tr})
}


