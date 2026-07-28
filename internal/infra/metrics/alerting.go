package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	AlertingAuditWriteFailures = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "optikk",
		Subsystem: "alerting",
		Name:      "audit_write_failures_total",
		Help:      "Failed monitor audit writes by kind (event/delivery).",
	}, []string{"kind"})
)
