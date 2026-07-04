package querydetail

import (
	"context"
	"net/http"
	"strings"

	"github.com/optikklabs/query/internal/modules/saturation/database/filter"
	"github.com/optikklabs/query/internal/shared/errorcode"
	modulecommon "github.com/optikklabs/query/internal/shared/httputil"
)

type Handler struct {
	Service *Service
}

// parseHash returns the required query hash param, responding 400 when absent.
func parseHash(w http.ResponseWriter, r *http.Request) (string, bool) {
	hash := strings.TrimSpace(r.URL.Query().Get("hash"))
	if hash == "" {
		modulecommon.RespondError(w, r, http.StatusBadRequest, errorcode.Validation, "hash is required")
		return "", false
	}
	return hash, true
}

func (h *Handler) GetSummary(w http.ResponseWriter, r *http.Request) {
	hash, ok := parseHash(w, r)
	if !ok {
		return
	}
	modulecommon.HandleRangeQuery(w, r, "Failed to query detail summary", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetSummary(ctx, tenantID, startMs, endMs, hash, filter.ParseFilters(r))
	})
}

func (h *Handler) GetTimeseries(w http.ResponseWriter, r *http.Request) {
	hash, ok := parseHash(w, r)
	if !ok {
		return
	}
	modulecommon.HandleRangeQuery(w, r, "Failed to query detail timeseries", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetTimeseries(ctx, tenantID, startMs, endMs, hash, filter.ParseFilters(r))
	})
}

func (h *Handler) GetExecutions(w http.ResponseWriter, r *http.Request) {
	hash, ok := parseHash(w, r)
	if !ok {
		return
	}
	modulecommon.HandleRangeQuery(w, r, "Failed to query detail executions", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetExecutions(ctx, tenantID, startMs, endMs, hash, filter.ParseFilters(r), filter.ParseLimit(r, 50))
	})
}
