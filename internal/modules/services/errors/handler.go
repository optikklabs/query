package errors

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/optikklabs/query/internal/infra/cursor"
	"github.com/optikklabs/query/internal/shared/errorcode"

	modulecommon "github.com/optikklabs/query/internal/shared/httputil"
)

type ErrorHandler struct {
	Service *Service
}

func (h *ErrorHandler) GetServiceErrorRate(w http.ResponseWriter, r *http.Request) {
	serviceName := r.URL.Query().Get("serviceName")
	modulecommon.HandleRangeQuery(w, r, "Failed to query service error rate", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetServiceErrorRate(ctx, tenantID, startMs, endMs, serviceName)
	})
}

func (h *ErrorHandler) GetErrorVolume(w http.ResponseWriter, r *http.Request) {
	serviceName := r.URL.Query().Get("serviceName")
	modulecommon.HandleRangeQuery(w, r, "Failed to query error volume", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetErrorVolume(ctx, tenantID, startMs, endMs, serviceName)
	})
}

func (h *ErrorHandler) GetErrorGroups(w http.ResponseWriter, r *http.Request) {
	tenantID := modulecommon.Tenant(r).TenantID
	serviceName := r.URL.Query().Get("serviceName")

	limit := modulecommon.ParseIntParam(r, "limit", 100)
	startMs, endMs, ok := modulecommon.ParseRequiredRange(w, r)
	if !ok {
		return
	}
	cursorStr := r.URL.Query().Get("cursor")
	var cur ErrorGroupsCursor
	if cursorStr != "" {
		if decoded, ok := cursor.Decode[ErrorGroupsCursor](cursorStr); ok {
			cur = decoded
		}
	}

	groups, err := h.Service.GetErrorGroups(r.Context(), tenantID, startMs, endMs, serviceName, limit, cur)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to query overview errors", err)
		return
	}

	modulecommon.RespondOK(w, groups)
}

func (h *ErrorHandler) GetErrorGroupDetail(w http.ResponseWriter, r *http.Request) {
	groupID := chi.URLParam(r, "groupId")
	modulecommon.HandleRangeQuery(w, r, "Failed to query error group detail", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetErrorGroupDetail(ctx, tenantID, startMs, endMs, groupID)
	})
}

const maxTracesLimit = 20

func (h *ErrorHandler) GetErrorGroupTraces(w http.ResponseWriter, r *http.Request) {
	tenantID := modulecommon.Tenant(r).TenantID
	groupID := chi.URLParam(r, "groupId")
	limit := modulecommon.ParseIntParam(r, "limit", maxTracesLimit)
	if limit < 1 || limit > maxTracesLimit {
		limit = maxTracesLimit
	}
	startMs, endMs, ok := modulecommon.ParseRequiredRange(w, r)
	if !ok {
		return
	}
	var cur ErrorTracesCursor
	if cursorStr := r.URL.Query().Get("cursor"); cursorStr != "" {
		if decoded, ok := cursor.Decode[ErrorTracesCursor](cursorStr); ok {
			cur = decoded
		}
	}

	traces, err := h.Service.GetErrorGroupTraces(r.Context(), tenantID, startMs, endMs, groupID, limit, cur)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to query error group traces", err)
		return
	}
	modulecommon.RespondOK(w, traces)
}

func (h *ErrorHandler) GetErrorGroupTimeseries(w http.ResponseWriter, r *http.Request) {
	groupID := chi.URLParam(r, "groupId")
	modulecommon.HandleRangeQuery(w, r, "Failed to query error group timeseries", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetErrorGroupTimeseries(ctx, tenantID, startMs, endMs, groupID)
	})
}

func (h *ErrorHandler) GetErrorGroupLatestOccurrence(w http.ResponseWriter, r *http.Request) {
	groupID := chi.URLParam(r, "groupId")
	modulecommon.HandleRangeQuery(w, r, "Failed to query error group latest occurrence", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetErrorGroupLatestOccurrence(ctx, tenantID, startMs, endMs, groupID)
	})
}

func (h *ErrorHandler) GetErrorGroupFacets(w http.ResponseWriter, r *http.Request) {
	groupID := chi.URLParam(r, "groupId")
	modulecommon.HandleRangeQuery(w, r, "Failed to query error group facets", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetErrorGroupFacets(ctx, tenantID, startMs, endMs, groupID)
	})
}

func (h *ErrorHandler) GetErrorHotspot(w http.ResponseWriter, r *http.Request) {
	modulecommon.HandleRangeQuery(w, r, "Failed to query error hotspot", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetErrorHotspot(ctx, tenantID, startMs, endMs)
	})
}
