package deployments

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/optikklabs/query/internal/modules/deployments/models"
	"github.com/optikklabs/query/internal/modules/deployments/service"
	"github.com/optikklabs/query/internal/shared/httputil"
)

const defaultDetailLimit = 50

type Handler struct {
	service *service.Service
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	httputil.HandleRangeQuery(w, r, "Failed to query deployments", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.service.List(ctx, models.ListRequest{TenantID: tenantID, StartMs: startMs, EndMs: endMs})
	})
}

func (h *Handler) Compare(w http.ResponseWriter, r *http.Request) {
	h.handleDetail(w, r, "Failed to compare deployment", func(ctx context.Context, req models.DetailRequest) (any, error) {
		return h.service.Compare(ctx, req)
	})
}

func (h *Handler) Traffic(w http.ResponseWriter, r *http.Request) {
	h.handleDetail(w, r, "Failed to query deployment traffic", func(ctx context.Context, req models.DetailRequest) (any, error) {
		return h.service.Traffic(ctx, req)
	})
}

func (h *Handler) Errors(w http.ResponseWriter, r *http.Request) {
	h.handleDetail(w, r, "Failed to query deployment errors", func(ctx context.Context, req models.DetailRequest) (any, error) {
		return h.service.Errors(ctx, req)
	})
}

func (h *Handler) Endpoints(w http.ResponseWriter, r *http.Request) {
	h.handleDetail(w, r, "Failed to query deployment endpoints", func(ctx context.Context, req models.DetailRequest) (any, error) {
		return h.service.Endpoints(ctx, req)
	})
}

func (h *Handler) Dependencies(w http.ResponseWriter, r *http.Request) {
	h.handleDetail(w, r, "Failed to query deployment dependencies", func(ctx context.Context, req models.DetailRequest) (any, error) {
		return h.service.Dependencies(ctx, req)
	})
}

func (h *Handler) handleDetail(
	w http.ResponseWriter,
	r *http.Request,
	errMessage string,
	query func(context.Context, models.DetailRequest) (any, error),
) {
	limit := httputil.ParsePageSize(r, "limit", defaultDetailLimit)
	httputil.HandleRangeQuery(w, r, errMessage, func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		_, environmentSet := r.URL.Query()["environment"]
		req := models.DetailRequest{
			ListRequest: models.ListRequest{
				TenantID: tenantID,
				StartMs:  startMs,
				EndMs:    endMs,
			},
			Service:        chi.URLParam(r, "service"),
			Version:        chi.URLParam(r, "version"),
			Environment:    r.URL.Query().Get("environment"),
			EnvironmentSet: environmentSet,
			Limit:          limit,
		}
		return query(ctx, req)
	})
}
