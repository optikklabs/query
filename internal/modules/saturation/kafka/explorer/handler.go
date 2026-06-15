package explorer

import (
	"context"
	"net/http"

	modulecommon "github.com/optikklabs/query/internal/shared/httputil"
)

type Handler struct {
	Service *Service
}

func (h *Handler) GetTopicThroughput(w http.ResponseWriter, r *http.Request) {
	topic := r.URL.Query().Get("topic")
	modulecommon.HandleRangeQuery(w, r, "Failed to query topic throughput", func(ctx context.Context, teamID, startMs, endMs int64) (any, error) {
		return h.Service.GetTopicThroughput(ctx, teamID, startMs, endMs, topic)
	})
}

func (h *Handler) GetTopicLag(w http.ResponseWriter, r *http.Request) {
	topic := r.URL.Query().Get("topic")
	modulecommon.HandleRangeQuery(w, r, "Failed to query topic lag", func(ctx context.Context, teamID, startMs, endMs int64) (any, error) {
		return h.Service.GetTopicLag(ctx, teamID, startMs, endMs, topic)
	})
}

func (h *Handler) GetTopicConsumers(w http.ResponseWriter, r *http.Request) {
	topic := r.URL.Query().Get("topic")
	modulecommon.HandleRangeQuery(w, r, "Failed to query topic consumers", func(ctx context.Context, teamID, startMs, endMs int64) (any, error) {
		return h.Service.GetTopicConsumers(ctx, teamID, startMs, endMs, topic)
	})
}

func (h *Handler) GetGroupPartitions(w http.ResponseWriter, r *http.Request) {
	group := r.URL.Query().Get("group")
	modulecommon.HandleRangeQuery(w, r, "Failed to query group partitions", func(ctx context.Context, teamID, startMs, endMs int64) (any, error) {
		return h.Service.GetGroupPartitions(ctx, teamID, startMs, endMs, group)
	})
}

func (h *Handler) GetGroupCommits(w http.ResponseWriter, r *http.Request) {
	group := r.URL.Query().Get("group")
	modulecommon.HandleRangeQuery(w, r, "Failed to query group commits", func(ctx context.Context, teamID, startMs, endMs int64) (any, error) {
		return h.Service.GetGroupCommits(ctx, teamID, startMs, endMs, group)
	})
}

func (h *Handler) GetGroupFetches(w http.ResponseWriter, r *http.Request) {
	group := r.URL.Query().Get("group")
	modulecommon.HandleRangeQuery(w, r, "Failed to query group fetches", func(ctx context.Context, teamID, startMs, endMs int64) (any, error) {
		return h.Service.GetGroupFetches(ctx, teamID, startMs, endMs, group)
	})
}

func (h *Handler) GetGroupHealth(w http.ResponseWriter, r *http.Request) {
	group := r.URL.Query().Get("group")
	modulecommon.HandleRangeQuery(w, r, "Failed to query group health", func(ctx context.Context, teamID, startMs, endMs int64) (any, error) {
		return h.Service.GetGroupHealth(ctx, teamID, startMs, endMs, group)
	})
}
