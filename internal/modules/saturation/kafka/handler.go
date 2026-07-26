package kafka

import (
	"context"
	"net/http"
	"strings"

	"github.com/optikklabs/query/internal/modules/saturation/kafka/service"
	modulecommon "github.com/optikklabs/query/internal/shared/httputil"
)

type Handler struct {
	Service *service.Service
}

func (h *Handler) GetTopicThroughput(w http.ResponseWriter, r *http.Request) {
	topic := r.URL.Query().Get("topic")
	modulecommon.HandleRangeQuery(w, r, "Failed to query topic throughput", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetTopicThroughput(ctx, tenantID, startMs, endMs, topic)
	})
}

func (h *Handler) GetGroupPartitions(w http.ResponseWriter, r *http.Request) {
	group := r.URL.Query().Get("group")
	modulecommon.HandleRangeQuery(w, r, "Failed to query group partitions", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetGroupPartitions(ctx, tenantID, startMs, endMs, group)
	})
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
