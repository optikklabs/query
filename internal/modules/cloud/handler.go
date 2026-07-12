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

func (h *Handler) GetCloudOverview(w http.ResponseWriter, r *http.Request) {
	modulecommon.HandleRangeQuery(w, r, "Failed to query cloud overview", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetOverview(ctx, tenantID, startMs, endMs)
	})
}

func (h *Handler) GetCloudProvider(w http.ResponseWriter, r *http.Request) {
	provider := strings.ToLower(chi.URLParam(r, "provider"))
	modulecommon.HandleRangeQuery(w, r, "Failed to query cloud provider", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetProviderDetail(ctx, tenantID, provider, startMs, endMs)
	})
}
