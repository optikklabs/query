package errors

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/optikklabs/query/internal/infra/cursor"
	"github.com/optikklabs/query/internal/modules/services/errors/models"
	"github.com/optikklabs/query/internal/modules/services/errors/service"
	"github.com/optikklabs/query/internal/shared/errorcode"

	modulecommon "github.com/optikklabs/query/internal/shared/httputil"
)

type ErrorHandler struct {
	Service *service.Service
}

func (h *ErrorHandler) GetServiceErrorRate(w http.ResponseWriter, r *http.Request) {
	serviceName := r.URL.Query().Get("serviceName")
	modulecommon.HandleRangeQuery(w, r, "Failed to query service error rate", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetServiceErrorRate(ctx, tenantID, startMs, endMs, serviceName)
	})
}

func (h *ErrorHandler) QueryErrorGroups(w http.ResponseWriter, r *http.Request) {
	var req models.GroupsRequest
	if !modulecommon.BindFiltered(w, r, &req) {
		return
	}
	resp, err := h.Service.QueryErrorGroups(r.Context(), req)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to query error groups", err)
		return
	}
	modulecommon.RespondOK(w, resp)
}

func (h *ErrorHandler) QueryErrorFacets(w http.ResponseWriter, r *http.Request) {
	var req models.FacetsRequest
	if !modulecommon.BindFiltered(w, r, &req) {
		return
	}
	resp, err := h.Service.QueryErrorFacets(r.Context(), req)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to query error facets", err)
		return
	}
	modulecommon.RespondOK(w, resp)
}

func (h *ErrorHandler) QueryErrorOverview(w http.ResponseWriter, r *http.Request) {
	var req models.OverviewRequest
	if !modulecommon.BindFiltered(w, r, &req) {
		return
	}
	resp, err := h.Service.QueryErrorOverview(r.Context(), req)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to query error overview", err)
		return
	}
	modulecommon.RespondOK(w, resp)
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
	var cur models.ErrorTracesCursor
	if cursorStr := r.URL.Query().Get("cursor"); cursorStr != "" {
		if decoded, ok := cursor.Decode[models.ErrorTracesCursor](cursorStr); ok {
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
