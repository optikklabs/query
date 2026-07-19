package topology

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/modules/saturation/kafka/filter"
	"github.com/optikklabs/query/internal/shared/chargs"
	"github.com/optikklabs/query/internal/shared/seriesattr"
)

type Repository struct {
	db clickhouse.Conn
}

func NewRepository(db clickhouse.Conn) *Repository {
	return &Repository{db: db}
}

// QueryClients lists the services with Kafka messaging spans in the window,
// busiest first so callers can default to the tenant's most active client.
func (r *Repository) QueryClients(ctx context.Context, tenantID, startMs, endMs int64) ([]string, error) {
	query := `
		SELECT service AS service
		FROM optikk.metrics_series
		PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end AND metric_name = 'traces.span.metrics.duration' AND service != ''
		WHERE ` + filter.AttrSystem + ` = 'kafka' AND ` + filter.AttrTopic + ` != ''
		GROUP BY service
		ORDER BY count() DESC, service`
	var rows []string
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "kafka.QueryClients", &rows, query, chargs.RangeArgs(tenantID, startMs, endMs)...)
}

// QueryEdges returns one row per (service, topic, consumer group) for the
// topics the given services touch. Produce rows carry an empty group, which is
// what separates the two sides of the graph -- so one scan of the rollup
// answers both, rather than a near-identical query per side.
func (r *Repository) QueryEdges(ctx context.Context, tenantID, startMs, endMs int64, services []string) ([]edgeRow, error) {
	query := `
		WITH scoped_topics AS (
		    SELECT DISTINCT ` + filter.AttrTopic + ` AS topic
		    FROM optikk.metrics_series
		    PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end AND metric_name = 'traces.span.metrics.duration' AND service IN @services
		    WHERE ` + filter.AttrSystem + ` = 'kafka' AND ` + filter.AttrTopic + ` != ''
		),
		series AS (
		    SELECT fingerprint,
		           service,
		           ` + seriesattr.StatusCode + `      AS status_code,
		           ` + filter.AttrTopic + `           AS topic,
		           ` + filter.AttrConsumerGroup + `   AS consumer_group
		    FROM optikk.metrics_series AS s
		    PREWHERE s.tenant_id = @tenantID AND s.timestamp BETWEEN @start AND @end AND s.metric_name = 'traces.span.metrics.duration' AND s.service != ''
		    WHERE ` + filter.AttrSystem + ` = 'kafka' AND ` + filter.AttrTopic + ` IN (SELECT topic FROM scoped_topics)
		    GROUP BY fingerprint, service, status_code, topic, consumer_group
		)
		SELECT series.service                                                  AS service,
		       series.topic                                                    AS topic,
		       series.consumer_group                                           AS consumer_group,
		       sum(m.hist_count)                                               AS call_count,
		       sumIf(m.hist_count, ` + seriesattr.StatusErrorPred + `)         AS error_count,
		       quantilesPrometheusHistogramMerge(0.5, 0.95, 0.99)(m.latency_state) AS qs
		FROM ` + timebucket.MetricsHistRollup(endMs-startMs) + ` AS m
		INNER JOIN series ON m.fingerprint = series.fingerprint
		PREWHERE m.tenant_id = @tenantID
		     AND m.timestamp BETWEEN @start AND @end
		     AND m.metric_name = 'traces.span.metrics.duration'
		GROUP BY service, topic, consumer_group
		ORDER BY call_count DESC`
	args := append(chargs.RollupRangeArgs(tenantID, startMs, endMs), clickhouse.Named("services", services))
	var rows []edgeRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "kafka.QueryEdges", &rows, query, args...)
}
