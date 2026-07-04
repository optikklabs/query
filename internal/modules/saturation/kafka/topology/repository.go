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

// Produce spans carry a topic but no consumer group; consume spans carry both.
// This splits the two sides without depending on the span.kind enum spelling.
const (
	produceWhere = filter.AttrSystem + " = 'kafka' AND " + filter.AttrTopic + " != '' AND " + filter.AttrConsumerGroup + " = ''"
	consumeWhere = filter.AttrSystem + " = 'kafka' AND " + filter.AttrConsumerGroup + " != ''"
)

func (r *Repository) QueryProduceEdges(ctx context.Context, tenantID, startMs, endMs int64) ([]produceEdgeRow, error) {
	query := `
		WITH series AS (
		    SELECT fingerprint,
		           service,
		           ` + seriesattr.StatusCode + ` AS status_code,
		           ` + filter.AttrTopic + `      AS topic
		    FROM optikk.metrics_series AS s
		    PREWHERE s.tenant_id = @tenantID AND s.timestamp BETWEEN @start AND @end AND s.metric_name = 'traces.span.metrics.duration' AND s.service != ''
		    WHERE ` + produceWhere + `
		    GROUP BY fingerprint, service, status_code, topic
		)
		SELECT series.service                                                  AS service,
		       series.topic                                                    AS topic,
		       sum(m.hist_count)                                               AS call_count,
		       sumIf(m.hist_count, ` + seriesattr.StatusErrorPred + `)         AS error_count,
		       quantilesPrometheusHistogramMerge(0.5, 0.95, 0.99)(m.latency_state) AS qs
		FROM ` + timebucket.MetricsHistRollup(endMs-startMs) + ` AS m
		INNER JOIN series ON m.fingerprint = series.fingerprint
		PREWHERE m.tenant_id = @tenantID
		     AND m.timestamp BETWEEN @start AND @end
		     AND m.metric_name = 'traces.span.metrics.duration'
		GROUP BY service, topic
		ORDER BY call_count DESC`
	var rows []produceEdgeRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "kafka.QueryProduceEdges", &rows, query, chargs.RollupRangeArgs(tenantID, startMs, endMs)...)
}

func (r *Repository) QueryConsumeEdges(ctx context.Context, tenantID, startMs, endMs int64) ([]consumeEdgeRow, error) {
	query := `
		WITH series AS (
		    SELECT fingerprint,
		           service,
		           ` + seriesattr.StatusCode + `      AS status_code,
		           ` + filter.AttrTopic + `           AS topic,
		           ` + filter.AttrConsumerGroup + `   AS consumer_group
		    FROM optikk.metrics_series AS s
		    PREWHERE s.tenant_id = @tenantID AND s.timestamp BETWEEN @start AND @end AND s.metric_name = 'traces.span.metrics.duration' AND s.service != ''
		    WHERE ` + consumeWhere + `
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
	var rows []consumeEdgeRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "kafka.QueryConsumeEdges", &rows, query, chargs.RollupRangeArgs(tenantID, startMs, endMs)...)
}
