package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// AlertingAuditWriteFailures counts best-effort monitor audit writes that
	// failed (event history, channel-delivery marks), so the loss is observable.
	AlertingAuditWriteFailures = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "optikk",
		Subsystem: "alerting",
		Name:      "audit_write_failures_total",
		Help:      "Failed monitor audit writes by kind (event/delivery).",
	}, []string{"kind"})
)
