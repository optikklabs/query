package infrastructure

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/optikklabs/query/internal/modules/infrastructure/service"
	"github.com/optikklabs/query/internal/shared/errorcode"
	"github.com/optikklabs/query/internal/shared/httputil"
)

type Handler struct {
	Service *service.Service
}

func (h *Handler) GetAvgCPU(w http.ResponseWriter, r *http.Request) {
	httputil.HandleRangeQuery(w, r, "Failed to query avg CPU", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetAvgCPU(ctx, tenantID, startMs, endMs)
	})
}

func (h *Handler) GetCPUByInstance(w http.ResponseWriter, r *http.Request) {
	httputil.HandleRangeQuery(w, r, "Failed to query CPU by instance", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetCPUByInstance(ctx, tenantID, startMs, endMs)
	})
}

func (h *Handler) GetAvgMemory(w http.ResponseWriter, r *http.Request) {
	httputil.HandleRangeQuery(w, r, "Failed to query avg memory", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetAvgMemory(ctx, tenantID, startMs, endMs)
	})
}

func (h *Handler) GetMemoryByInstance(w http.ResponseWriter, r *http.Request) {
	tenantID := httputil.Tenant(r).TenantID
	startMs, endMs, ok := httputil.ParseRequiredRange(w, r)
	if !ok {
		return
	}
	host := r.URL.Query().Get("host")
	pod := r.URL.Query().Get("pod")
	serviceName := r.URL.Query().Get("serviceName")
	if serviceName == "" {
		httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.BadRequest, "serviceName is required", nil)
		return
	}
	resp, err := h.Service.GetMemoryByInstance(r.Context(), tenantID, host, pod, serviceName, startMs, endMs)
	if err != nil {
		httputil.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to query memory by instance", err)
		return
	}
	httputil.RespondOK(w, resp)
}

func (h *Handler) GetHosts(w http.ResponseWriter, r *http.Request) {
	httputil.HandleRangeQuery(w, r, "Failed to query hosts", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetHosts(ctx, tenantID, startMs, endMs, r.URL.Query().Get("service"))
	})
}

func (h *Handler) GetFleetPods(w http.ResponseWriter, r *http.Request) {
	host := r.URL.Query().Get("host")
	httputil.HandleRangeQuery(w, r, "Failed to query fleet pods", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetFleetPods(ctx, tenantID, startMs, endMs, host)
	})
}

func (h *Handler) GetInfrastructureNodes(w http.ResponseWriter, r *http.Request) {
	httputil.HandleRangeQuery(w, r, "Failed to query node health", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetInfrastructureNodes(ctx, tenantID, startMs, endMs)
	})
}

func (h *Handler) GetInfrastructureNodeSummary(w http.ResponseWriter, r *http.Request) {
	httputil.HandleRangeQuery(w, r, "Failed to query node summary", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetInfrastructureNodeSummary(ctx, tenantID, startMs, endMs)
	})
}

func (h *Handler) GetInfrastructureNodeServices(w http.ResponseWriter, r *http.Request) {
	host := chi.URLParam(r, "host")
	httputil.HandleRangeQuery(w, r, "Failed to query node services", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetInfrastructureNodeServices(ctx, tenantID, host, startMs, endMs)
	})
}

func (h *Handler) GetHostOverview(w http.ResponseWriter, r *http.Request) {
	host := chi.URLParam(r, "host")
	httputil.HandleRangeQuery(w, r, "Failed to query host overview", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetHostOverview(ctx, tenantID, host, startMs, endMs)
	})
}

func (h *Handler) GetHostSeries(w http.ResponseWriter, r *http.Request) {
	host := chi.URLParam(r, "host")
	h.respondSeries(w, r, "Failed to query host series", func(ctx context.Context, tenantID, startMs, endMs int64, metricID string) (any, bool, error) {
		return h.Service.GetHostSeries(ctx, tenantID, host, metricID, startMs, endMs)
	})
}

func (h *Handler) GetPodOverview(w http.ResponseWriter, r *http.Request) {
	pod := chi.URLParam(r, "pod")
	httputil.HandleRangeQuery(w, r, "Failed to query pod overview", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetPodOverview(ctx, tenantID, pod, startMs, endMs)
	})
}

func (h *Handler) GetPodSeries(w http.ResponseWriter, r *http.Request) {
	pod := chi.URLParam(r, "pod")
	h.respondSeries(w, r, "Failed to query pod series", func(ctx context.Context, tenantID, startMs, endMs int64, metricID string) (any, bool, error) {
		return h.Service.GetPodSeries(ctx, tenantID, pod, metricID, startMs, endMs)
	})
}

func (h *Handler) respondSeries(
	w http.ResponseWriter, r *http.Request, failMsg string,
	query func(ctx context.Context, tenantID, startMs, endMs int64, metricID string) (any, bool, error),
) {
	tenantID := httputil.Tenant(r).TenantID
	startMs, endMs, ok := httputil.ParseRequiredRange(w, r)
	if !ok {
		return
	}
	rows, known, err := query(r.Context(), tenantID, startMs, endMs, r.URL.Query().Get("metric"))
	if !known {
		httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.BadRequest, "unknown metric group", nil)
		return
	}
	if err != nil {
		httputil.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, failMsg, err)
		return
	}
	httputil.RespondOK(w, rows)
}
