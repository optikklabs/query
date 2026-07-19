package topology

import (
	"context"
	"net/http"
	"strings"

	modulecommon "github.com/optikklabs/query/internal/shared/httputil"
)

type Handler struct {
	Service *Service
}

// GetClients returns the tenant's Kafka client roster for the picker.
func (h *Handler) GetClients(w http.ResponseWriter, r *http.Request) {
	modulecommon.HandleRangeQuery(w, r, "Failed to list kafka clients", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetClients(ctx, tenantID, startMs, endMs)
	})
}

// GetTopology returns the producers->topics->consumers graph for the services
// named in the comma-separated `services` param.
func (h *Handler) GetTopology(w http.ResponseWriter, r *http.Request) {
	services := parseServices(r.URL.Query().Get("services"))
	modulecommon.HandleRangeQuery(w, r, "Failed to build kafka topology", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetTopology(ctx, tenantID, startMs, endMs, services)
	})
}

// parseServices splits the CSV param, dropping blanks so a trailing comma or
// an empty param cannot widen the query to every service.
func parseServices(raw string) []string {
	out := make([]string, 0, strings.Count(raw, ",")+1)
	for _, s := range strings.Split(raw, ",") {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}
