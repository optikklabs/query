package explorer

import (
	"context"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/modules/saturation/database/filter"
	"github.com/optikklabs/query/internal/shared/chargs"
	"github.com/optikklabs/query/internal/shared/seriesattr"
)

type Repository struct {
	db clickhouse.Conn
}

func NewRepository(db clickhouse.Conn) *Repository {
	return &Repository{db: db}
}

type systemSummaryRawDTO struct {
	DBSystem     string    `ch:"db_system"`
	QueryCount   uint64    `ch:"query_count"`
	ErrorCount   uint64    `ch:"error_count"`
	AvgLatencyMs float64   `ch:"avg_latency_ms"`
	P95Ms        float32   `ch:"p95_ms"`
	LastSeen     time.Time `ch:"last_seen"`
}

type connRawRow struct {
	DBSystem string `ch:"db_system"`
	Active   int64  `ch:"active_count"`
}

func (r *Repository) GetSystemSummariesRaw(ctx context.Context, tenantID, startMs, endMs int64) ([]systemSummaryRawDTO, error) {
	startMs, endMs = timebucket.SnapRangeForRollup(startMs, endMs)
	query := `
		WITH series AS (
		    SELECT fingerprint,
		           ` + seriesattr.DBSystem + ` AS db_system,
		           ` + seriesattr.StatusCode + ` AS status_code
		    FROM optikk.metrics_series
		    PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end AND metric_name = 'traces.span.metrics.duration'
		    WHERE ` + seriesattr.DBSpanPred + `
		    GROUP BY fingerprint, db_system, status_code
		)
		SELECT series.db_system                                                AS db_system,
		       sum(m.hist_count)                                               AS query_count,
		       sumIf(m.hist_count, ` + seriesattr.StatusErrorPred + `)         AS error_count,
		       sum(m.hist_sum) / nullIf(sum(m.hist_count), 0)                  AS avg_latency_ms,
		       toFloat32(quantilesPrometheusHistogramMerge(0.95)(m.latency_state)[1]) AS p95_ms,
		       max(m.timestamp)                                                AS last_seen
		FROM ` + timebucket.MetricsHistRollup(endMs-startMs) + ` AS m
		INNER JOIN series ON m.fingerprint = series.fingerprint
		PREWHERE m.tenant_id     = @tenantID
		     AND m.timestamp   BETWEEN @start AND @end
		     AND m.metric_name = 'traces.span.metrics.duration'
		GROUP BY db_system
		ORDER BY query_count DESC`

	args := chargs.RangeArgs(tenantID, startMs, endMs)
	var rows []systemSummaryRawDTO
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "datastoreSystems.GetSystemSummariesRaw", &rows, query, args...)
}

// GetActiveConnectionsBySystem returns active connections by database system.
func (r *Repository) GetActiveConnectionsBySystem(ctx context.Context, tenantID, startMs, endMs int64) (map[string]int64, error) {
	startMs, endMs = timebucket.SnapRangeForRollup(startMs, endMs)

	query := activeConnectionsQuery(timebucket.MetricsRollup(endMs - startMs))
	args := append(chargs.RangeArgs(tenantID, startMs, endMs),
		clickhouse.Named("metricName", filter.MetricDBSQLConnectionOpen),
	)
	var rows []connRawRow
	if err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "datastoreSystems.GetActiveConnectionsBySystem", &rows, query, args...); err != nil {
		return nil, err
	}
	out := make(map[string]int64, len(rows))
	for _, row := range rows {
		out[row.DBSystem] = row.Active
	}
	return out, nil
}

func activeConnectionsQuery(rollupTable string) string {
	return `
		WITH series AS (
		    SELECT fingerprint, ` + seriesattr.DBSystem + ` AS db_system
		    FROM optikk.metrics_series
		    PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end AND metric_name = @metricName
		    WHERE ` + seriesattr.DBSpanPred + `
		    GROUP BY fingerprint, db_system
		), latest AS (
		    SELECT series.db_system AS db_system,
		           m.fingerprint    AS fingerprint,
		           argMaxMerge(m.val_last) AS latest_value
		    FROM ` + rollupTable + ` AS m
		    INNER JOIN series ON m.fingerprint = series.fingerprint
		    PREWHERE m.tenant_id     = @tenantID
		         AND m.metric_name = @metricName
		         AND m.timestamp   BETWEEN @start AND @end
		    GROUP BY db_system, fingerprint
		)
		SELECT db_system,
		       toInt64(round(ifNotFinite(sum(latest_value), 0))) AS active_count
		FROM latest
		GROUP BY db_system`
}
