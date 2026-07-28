package database

import (
	"context"
	"net/http"
	"strings"

	"github.com/optikklabs/query/internal/modules/saturation/database/filter"
	"github.com/optikklabs/query/internal/modules/saturation/database/service"
	"github.com/optikklabs/query/internal/shared/errorcode"
	modulecommon "github.com/optikklabs/query/internal/shared/httputil"
)

type Handler struct {
	Service *service.Service
}

func (h *Handler) GetDatastoreSystems(w http.ResponseWriter, r *http.Request) {
	modulecommon.HandleRangeQuery(w, r, "Failed to query datastore systems", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetDatastoreSystems(ctx, tenantID, startMs, endMs)
	})
}

func (h *Handler) GetLatencyBySystem(w http.ResponseWriter, r *http.Request) {
	modulecommon.HandleRangeQuery(w, r, "Failed to query latency by system", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetLatencyBySystem(ctx, tenantID, startMs, endMs, filter.ParseFilters(r))
	})
}

func (h *Handler) GetOpsBySystem(w http.ResponseWriter, r *http.Request) {
	modulecommon.HandleRangeQuery(w, r, "Failed to query ops by system", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetOpsBySystem(ctx, tenantID, startMs, endMs, filter.ParseFilters(r))
	})
}

func (h *Handler) GetSlowQueryPatterns(w http.ResponseWriter, r *http.Request) {
	modulecommon.HandleRangeQuery(w, r, "Failed to query slow query patterns", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetSlowQueryPatterns(ctx, tenantID, startMs, endMs, filter.ParseFilters(r), modulecommon.ParsePageSize(r, "limit", service.DefaultPatternLimit))
	})
}

func parseHash(w http.ResponseWriter, r *http.Request) (string, bool) {
	hash := strings.TrimSpace(r.URL.Query().Get("hash"))
	if hash == "" {
		modulecommon.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "hash is required", nil)
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
		return h.Service.GetExecutions(ctx, tenantID, startMs, endMs, hash, filter.ParseFilters(r), modulecommon.ParsePageSize(r, "limit", service.DefaultExecutionsLimit))
	})
}
