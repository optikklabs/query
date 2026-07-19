package trace_logs

import (
	"net/http"

	"github.com/optikklabs/query/internal/shared/errorcode"
	modulecommon "github.com/optikklabs/query/internal/shared/httputil"
)

const (
	defaultLimit = 1000
	maxLimit     = 5000
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{
		svc: svc,
	}
}

// GetByTrace powers GET /api/v1/logs/trace/{traceID} — all logs for a trace.
func (h *Handler) GetByTrace(w http.ResponseWriter, r *http.Request) {
	traceID := modulecommon.URLParamLower(r, "traceID")
	if traceID == "" {
		modulecommon.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "trace id required", nil)
		return
	}
	limit := modulecommon.ParseIntParam(r, "limit", defaultLimit)
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	logs, err := h.svc.GetByTraceID(r.Context(), modulecommon.Tenant(r).TenantID, traceID, limit)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to fetch logs by trace", err)
		return
	}
	modulecommon.RespondOK(w, logs)
}
