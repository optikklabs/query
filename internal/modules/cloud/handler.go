package cloud

import (
	"context"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	modulecommon "github.com/optikklabs/query/internal/shared/httputil"
)

type Handler struct {
	Service *Service
}

func (h *Handler) GetInventory(w http.ResponseWriter, r *http.Request) {
	modulecommon.HandleRangeQuery(w, r, "Failed to query cloud inventory", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetInventory(ctx, tenantID, startMs, endMs)
	})
}

func (h *Handler) GetCategories(w http.ResponseWriter, r *http.Request) {
	modulecommon.HandleRangeQuery(w, r, "Failed to query cloud categories", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetCategories(ctx, tenantID, startMs, endMs)
	})
}

func (h *Handler) GetHealth(w http.ResponseWriter, r *http.Request) {
	modulecommon.HandleRangeQuery(w, r, "Failed to query cloud health", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetHealth(ctx, tenantID, startMs, endMs)
	})
}

func (h *Handler) GetRestarts(w http.ResponseWriter, r *http.Request) {
	modulecommon.HandleRangeQuery(w, r, "Failed to query cloud restarts", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetRestarts(ctx, tenantID, startMs, endMs)
	})
}

func (h *Handler) GetProviderPlatforms(w http.ResponseWriter, r *http.Request) {
	provider := strings.ToLower(chi.URLParam(r, "provider"))
	modulecommon.HandleRangeQuery(w, r, "Failed to query cloud provider platforms", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetProviderPlatforms(ctx, tenantID, provider, startMs, endMs)
	})
}

func (h *Handler) GetProviderAccounts(w http.ResponseWriter, r *http.Request) {
	provider := strings.ToLower(chi.URLParam(r, "provider"))
	modulecommon.HandleRangeQuery(w, r, "Failed to query cloud provider accounts", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetProviderAccounts(ctx, tenantID, provider, startMs, endMs)
	})
}

func (h *Handler) GetProviderResources(w http.ResponseWriter, r *http.Request) {
	provider := strings.ToLower(chi.URLParam(r, "provider"))
	modulecommon.HandleRangeQuery(w, r, "Failed to query cloud provider resources", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetProviderResources(ctx, tenantID, provider, startMs, endMs)
	})
}
