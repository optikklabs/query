package log_facets //nolint:revive,stylecheck

import (
	"log/slog"
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

// Facets powers POST /api/v1/logs/facets.
func (h *Handler) Facets(w http.ResponseWriter, r *http.Request) {
	var req Request
	if err := modulecommon.DecodeJSON(r, &req); err != nil {
		modulecommon.RespondError(w, r, http.StatusBadRequest, errorcode.Validation, "Invalid request body")
		return
	}
	req.Filters.TeamID = modulecommon.Tenant(r).TeamID
	req.Filters.StartMs = req.StartTime
	req.Filters.EndMs = req.EndTime
	if err := req.Filters.Validate(); err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "Invalid filters", err)
		return
	}
	resp, err := h.svc.ComputeResponse(r.Context(), req.Filters)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to query logs facets", err)
		return
	}
	slog.Debug("Logs facets queried successfully", slog.Any("resp", resp))
	modulecommon.RespondOK(w, resp)
}
