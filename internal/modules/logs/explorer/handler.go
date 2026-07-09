package explorer

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

// Query powers POST /api/v1/logs/query (list + optional include blocks).
func (h *Handler) Query(w http.ResponseWriter, r *http.Request) {
	var req QueryRequest
	if err := modulecommon.DecodeJSON(r, &req); err != nil {
		modulecommon.RespondError(w, r, http.StatusBadRequest, errorcode.Validation, "Invalid request body")
		return
	}
	req.Filters.TenantID = modulecommon.Tenant(r).TenantID
	req.Filters.StartMs = req.StartTime
	req.Filters.EndMs = req.EndTime
	if err := req.Filters.Validate(); err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "Invalid filters", err)
		return
	}
	if req.Limit <= 0 || req.Limit > 5000 {
		req.Limit = 5000
	}
	resp, err := h.svc.Query(r.Context(), req)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to query logs", err)
		return
	}
	slog.Debug("Logs queried successfully", slog.Any("resp", resp))
	modulecommon.RespondOK(w, resp)
}
