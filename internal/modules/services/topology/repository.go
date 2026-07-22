package topology

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/shared/chargs"
	"github.com/optikklabs/query/internal/shared/seriesattr"
)

type Repository struct {
	db clickhouse.Conn
}

func NewRepository(db clickhouse.Conn) *Repository {
	return &Repository{db: db}
}

// GetNodes returns per-service RED aggregates and p50/p95/p99 latency.
func (r *Repository) GetNodes(ctx context.Context, tenantID, startMs, endMs int64, focusService string) ([]nodeAggRow, error) {
	query := `
		WITH neighbor_services AS (
		    SELECT ` + seriesattr.Client + ` AS service FROM optikk.metrics_series PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end WHERE ` + seriesattr.Server + ` = @focusService AND @focusService != '' AND ` + seriesattr.Client + ` != ''
		    UNION ALL
		    SELECT ` + seriesattr.Server + ` AS service FROM optikk.metrics_series PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end WHERE ` + seriesattr.Client + ` = @focusService AND @focusService != '' AND ` + seriesattr.Server + ` != ''
		),
		series AS (
		    SELECT fingerprint,
		           service,
		           ` + seriesattr.StatusCode + ` AS status_code
		    FROM optikk.metrics_series AS s
		    PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end AND metric_name = 'traces.span.metrics.duration' AND s.service != ''
		      AND (@focusService = '' OR s.service = @focusService OR s.service IN (SELECT service FROM neighbor_services))
		    GROUP BY fingerprint, service, status_code
		)
		SELECT series.service                                                       AS service,
		       sum(m.hist_count)                                                    AS request_count,
		       sumIf(m.hist_count, ` + seriesattr.StatusErrorPred + `)              AS error_count,
		       quantilesPrometheusHistogramMerge(0.5, 0.95, 0.99)(m.latency_state)  AS qs
		FROM ` + timebucket.MetricsHistRollup(endMs-startMs) + ` AS m
		INNER JOIN series ON m.fingerprint = series.fingerprint
		PREWHERE m.tenant_id = @tenantID AND m.timestamp BETWEEN @start AND @end
		  AND m.metric_name = 'traces.span.metrics.duration'
		GROUP BY service`
	var rows []nodeAggRow
	args := append(chargs.RollupRangeArgs(tenantID, startMs, endMs), clickhouse.Named("focusService", focusService))
	if err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "topology.GetNodes", &rows, query, args...); err != nil {
		return nil, err
	}
	for i := range rows {
		if len(rows[i].QS) >= 3 {
			rows[i].P50Ms = float32(rows[i].QS[0])
			rows[i].P95Ms = float32(rows[i].QS[1])
			rows[i].P99Ms = float32(rows[i].QS[2])
		}
	}
	return rows, nil
}

// GetEdges builds directed edges: request/error counts come from the
// service-graph counters (no status.code dimension) and latency from the server
// histogram. Counts are authoritative for the edge set; latency LEFT-joins in.
func (r *Repository) GetEdges(ctx context.Context, tenantID, startMs, endMs int64, focusService string) ([]edgeAggRow, error) {
	rollup := timebucket.MetricsHistRollup(endMs - startMs)
	edgeFilter := seriesattr.Client + ` != '' AND ` + seriesattr.Server + ` != '' AND ` +
		seriesattr.Client + ` != ` + seriesattr.Server + ` AND (@focusService = '' OR ` +
		seriesattr.Client + ` = @focusService OR ` + seriesattr.Server + ` = @focusService)`

	query := `
		WITH counter_series AS (
		    SELECT fingerprint,
		           ` + seriesattr.Client + ` AS client,
		           ` + seriesattr.Server + ` AS server,
		           metric_name
		    FROM optikk.metrics_series
		    PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end
		      AND metric_name IN ('traces_service_graph_request_total', 'traces_service_graph_request_failed_total')
		      AND ` + edgeFilter + `
		    GROUP BY fingerprint, client, server, metric_name
		),
		counts AS (
		    SELECT cs.client AS client,
		           cs.server AS server,
		           toUInt64(sumIf(m.val_sum, cs.metric_name = 'traces_service_graph_request_total'))        AS call_count,
		           toUInt64(sumIf(m.val_sum, cs.metric_name = 'traces_service_graph_request_failed_total')) AS error_count
		    FROM ` + rollup + ` AS m
		    INNER JOIN counter_series cs ON m.fingerprint = cs.fingerprint
		    PREWHERE m.tenant_id = @tenantID AND m.timestamp BETWEEN @start AND @end
		      AND m.metric_name IN ('traces_service_graph_request_total', 'traces_service_graph_request_failed_total')
		    GROUP BY client, server
		),
		hist_series AS (
		    SELECT fingerprint,
		           ` + seriesattr.Client + ` AS client,
		           ` + seriesattr.Server + ` AS server
		    FROM optikk.metrics_series
		    PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end AND metric_name = 'traces_service_graph_request_server'
		      AND ` + edgeFilter + `
		    GROUP BY fingerprint, client, server
		),
		latency AS (
		    SELECT hs.client AS client,
		           hs.server AS server,
		           quantilesPrometheusHistogramMerge(0.5, 0.95)(m.latency_state) AS qs
		    FROM ` + rollup + ` AS m
		    INNER JOIN hist_series hs ON m.fingerprint = hs.fingerprint
		    PREWHERE m.tenant_id = @tenantID AND m.timestamp BETWEEN @start AND @end
		      AND m.metric_name = 'traces_service_graph_request_server'
		    GROUP BY client, server
		)
		SELECT counts.client  AS source,
		       counts.server  AS target,
		       counts.call_count  AS call_count,
		       counts.error_count AS error_count,
		       latency.qs         AS qs
		FROM counts
		LEFT JOIN latency ON counts.client = latency.client AND counts.server = latency.server`
	var rows []edgeAggRow
	if err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "topology.GetEdges", &rows, query, spanArgs(tenantID, startMs, endMs, focusService)...); err != nil {
		return nil, err
	}
	for i := range rows {
		if len(rows[i].QS) >= 2 {
			rows[i].P50Ms = float32(rows[i].QS[0])
			rows[i].P95Ms = float32(rows[i].QS[1])
		}
	}
	return rows, nil
}

func spanArgs(tenantID, startMs, endMs int64, focusService string) []any {
	return append(chargs.RangeArgs(tenantID, startMs, endMs), clickhouse.Named("focusService", focusService))
}
