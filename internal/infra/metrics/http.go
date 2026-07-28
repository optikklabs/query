package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var httpBuckets = []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10, 30, 60}

var (
	HTTPRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "optikk",
		Subsystem: "http",
		Name:      "requests_total",
		Help:      "Total HTTP requests served, labelled by route template, method, and status class (2xx/3xx/4xx/5xx).",
	}, []string{"route", "method", "status_class"})

	HTTPDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "optikk",
		Subsystem: "http",
		Name:      "request_duration_seconds",
		Help:      "HTTP request handling duration in seconds, by route + method.",
		Buckets:   httpBuckets,
	}, []string{"route", "method"})

	HTTPInFlight = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "optikk",
		Subsystem: "http",
		Name:      "in_flight_requests",
		Help:      "HTTP requests currently being processed.",
	})

	APIRateLimitedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "optikk",
		Subsystem: "http",
		Name:      "api_rate_limited_total",
		Help:      "Total HTTP requests rate limited, labeled by tenant_id.",
	}, []string{"tenant_id"})

	QueryInFlight = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "optikk",
		Subsystem: "query",
		Name:      "in_flight_requests",
		Help:      "Telemetry queries currently executing, by workload class.",
	}, []string{"workload"})

	QueryWaitDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "optikk",
		Subsystem: "query",
		Name:      "wait_duration_seconds",
		Help:      "Time spent waiting for telemetry query capacity, by workload class.",
		Buckets:   httpBuckets,
	}, []string{"workload"})
)
