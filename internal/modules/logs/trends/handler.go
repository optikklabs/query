package log_trends //nolint:revive,stylecheck

import (
	"net/http"

	"github.com/optikklabs/query/internal/modules/logs/filter"
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
	f, ok := h.bindFilters(w, r)
	if !ok {
		return
	}
	sum, err := h.svc.Summary(r.Context(), f)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to query logs summary", err)
		return
	}
	modulecommon.RespondOK(w, SummaryResponse{Summary: sum})
}

func (h *Handler) Trend(w http.ResponseWriter, r *http.Request) {
	f, ok := h.bindFilters(w, r)
	if !ok {
		return
	}
	tr, err := h.svc.Trend(r.Context(), f)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to query logs trend", err)
		return
	}
	modulecommon.RespondOK(w, TrendResponse{Trend: tr})
}

func (h *Handler) bindFilters(w http.ResponseWriter, r *http.Request) (filter.Filters, bool) {
	var req Request
	if err := modulecommon.DecodeJSON(r, &req); err != nil {
		modulecommon.RespondError(w, r, http.StatusBadRequest, errorcode.Validation, "Invalid request body")
		return filter.Filters{}, false
	}
	req.Filters.TeamID = modulecommon.Tenant(r).TeamID
	req.Filters.StartMs = req.StartTime
	req.Filters.EndMs = req.EndTime
	if err := req.Filters.Validate(); err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "Invalid filters", err)
		return filter.Filters{}, false
	}
	return req.Filters, true
}
