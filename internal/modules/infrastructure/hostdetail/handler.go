package hostdetail

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/optikklabs/query/internal/shared/errorcode"
	modulecommon "github.com/optikklabs/query/internal/shared/httputil"
)

type Handler struct {
	Service *Service
}

func (h *Handler) GetOverview(w http.ResponseWriter, r *http.Request) {
	host := chi.URLParam(r, "host")
	modulecommon.HandleRangeQuery(w, r, "Failed to query host overview", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetOverview(ctx, tenantID, host, startMs, endMs)
	})
}

func (h *Handler) GetSeries(w http.ResponseWriter, r *http.Request) {
	host := chi.URLParam(r, "host")
	metricID := r.URL.Query().Get("metric")
	tenantID := modulecommon.Tenant(r).TenantID
	startMs, endMs, ok := modulecommon.ParseRequiredRange(w, r)
	if !ok {
		return
	}
	rows, known, err := h.Service.GetSeries(r.Context(), tenantID, host, metricID, startMs, endMs)
	if !known {
		modulecommon.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.BadRequest, "unknown metric group", nil)
		return
	}
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to query host series", err)
		return
	}
	modulecommon.RespondOK(w, rows)
}
