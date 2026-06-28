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
func (r *Repository) GetNodes(ctx context.Context, teamID, startMs, endMs int64, _ string) ([]nodeAggRow, error) {
	query := `
		WITH series AS (
		    SELECT fingerprint,
		           service,
		           ` + seriesattr.StatusCode + ` AS status_code
		    FROM optikk.metrics_series AS s
		    PREWHERE team_id = @teamID AND timestamp BETWEEN @start AND @end AND metric_name = 'traces.span.metrics.duration'
		    WHERE s.service != ''
		    GROUP BY fingerprint, service, status_code
		)
		SELECT series.service                                                       AS service,
		       sum(m.hist_count)                                                    AS request_count,
		       sumIf(m.hist_count, ` + seriesattr.StatusErrorPred + `)              AS error_count,
		       quantilesPrometheusHistogramMerge(0.5, 0.95, 0.99)(m.latency_state)  AS qs
		FROM ` + timebucket.MetricsHistRollup(endMs-startMs) + ` AS m
		INNER JOIN series ON m.fingerprint = series.fingerprint
		PREWHERE m.team_id = @teamID AND m.timestamp BETWEEN @start AND @end
		  AND m.metric_name = 'traces.span.metrics.duration'
		GROUP BY service`
	var rows []nodeAggRow
	if err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "topology.GetNodes", &rows, query, chargs.RollupRangeArgs(teamID, startMs, endMs)...); err != nil {
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

func (r *Repository) GetEdges(ctx context.Context, teamID, startMs, endMs int64, focusService string) ([]edgeAggRow, error) {
	query := `
		WITH series AS (
		    SELECT fingerprint,
		           ` + seriesattr.Client + `     AS client,
		           ` + seriesattr.Server + `     AS server,
		           ` + seriesattr.StatusCode + ` AS status_code
		    FROM optikk.metrics_series
		    PREWHERE team_id = @teamID AND timestamp BETWEEN @start AND @end AND metric_name = 'traces_service_graph_request_server'
		    WHERE ` + seriesattr.Client + ` != ''
		      AND ` + seriesattr.Server + ` != ''
		      AND ` + seriesattr.Client + ` != ` + seriesattr.Server + `
		      AND (@focusService = '' OR ` + seriesattr.Client + ` = @focusService OR ` + seriesattr.Server + ` = @focusService)
		    GROUP BY fingerprint, client, server, status_code
		)
		SELECT series.client                                                     AS source,
		       series.server                                                     AS target,
		       sum(m.hist_count)                                                  AS call_count,
		       sumIf(m.hist_count, ` + seriesattr.StatusErrorPred + `)            AS error_count,
		       quantilesPrometheusHistogramMerge(0.5, 0.95)(m.latency_state)      AS qs
		FROM ` + timebucket.MetricsHistRollup(endMs-startMs) + ` AS m
		INNER JOIN series ON m.fingerprint = series.fingerprint
		PREWHERE m.team_id = @teamID AND m.timestamp BETWEEN @start AND @end
		  AND m.metric_name = 'traces_service_graph_request_server'
		GROUP BY source, target`
	var rows []edgeAggRow
	if err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "topology.GetEdges", &rows, query, spanArgs(teamID, startMs, endMs, focusService)...); err != nil {
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

func spanArgs(teamID, startMs, endMs int64, focusService string) []any {
	return append(chargs.RangeArgs(teamID, startMs, endMs), clickhouse.Named("focusService", focusService))
}
