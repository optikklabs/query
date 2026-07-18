package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// TraceIndexMiss counts trace-by-id reads that found no optikk.trace_index
	// row. The index covers every span, so a miss means the trace has no spans
	// at all. A sustained rise means the ingest materialized view has stopped
	// populating and trace reads are silently returning nothing.
	TraceIndexMiss = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "optikk",
		Subsystem: "traces",
		Name:      "index_miss_total",
		Help:      "Trace lookups that resolved no window from optikk.trace_index.",
	})
)
