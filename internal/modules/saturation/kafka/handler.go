package kafka

import (
	"context"
	"net/http"
	"strings"

	"github.com/optikklabs/query/internal/modules/saturation/kafka/service"
	"github.com/optikklabs/query/internal/shared/httputil"
)

type Handler struct {
	Service *service.Service
}

func (h *Handler) GetTopicThroughput(w http.ResponseWriter, r *http.Request) {
	topic := r.URL.Query().Get("topic")
	httputil.HandleRangeQuery(w, r, "Failed to query topic throughput", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetTopicThroughput(ctx, tenantID, startMs, endMs, topic)
	})
}

func (h *Handler) GetGroupPartitions(w http.ResponseWriter, r *http.Request) {
	group := r.URL.Query().Get("group")
	httputil.HandleRangeQuery(w, r, "Failed to query group partitions", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetGroupPartitions(ctx, tenantID, startMs, endMs, group)
	})
}

func (h *Handler) GetClients(w http.ResponseWriter, r *http.Request) {
	httputil.HandleRangeQuery(w, r, "Failed to list kafka clients", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetClients(ctx, tenantID, startMs, endMs)
	})
}

func (h *Handler) GetTopology(w http.ResponseWriter, r *http.Request) {
	services := parseServices(r.URL.Query().Get("services"))
	httputil.HandleRangeQuery(w, r, "Failed to build kafka topology", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetTopology(ctx, tenantID, startMs, endMs, services)
	})
}

func parseServices(raw string) []string {
	out := make([]string, 0, strings.Count(raw, ",")+1)
	for _, s := range strings.Split(raw, ",") {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}
