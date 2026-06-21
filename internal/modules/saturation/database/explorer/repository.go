package explorer

import (
	"context"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/modules/saturation/database/filter"
	"github.com/optikklabs/query/internal/shared/seriesattr"
)

type Repository struct {
	db clickhouse.Conn
}

func NewRepository(db clickhouse.Conn) *Repository {
	return &Repository{db: db}
}

type systemSummaryRawDTO struct {
	DBSystem      string    `ch:"db_system"`
	QueryCount    uint64    `ch:"query_count"`
	ErrorCount    uint64    `ch:"error_count"`
	AvgLatencyMs  float64   `ch:"avg_latency_ms"`
	P95Ms         float32   `ch:"p95_ms"`
	ServerAddress string    `ch:"server_address"`
	LastSeen      time.Time `ch:"last_seen"`
}

type connRawRow struct {
	DBSystem string  `ch:"db_system"`
	Avg      float64 `ch:"avg_used"`
}

func (r *Repository) GetSystemSummariesRaw(ctx context.Context, teamID, startMs, endMs int64) ([]systemSummaryRawDTO, error) {
	startMs, endMs = timebucket.SnapRangeForRollup(startMs, endMs)
	query := `
		WITH series AS (
		    SELECT fingerprint,
		           any(attributes.` + "`db.system`" + `::String)      AS db_system,
		           any(attributes.` + "`server.address`" + `::String) AS server_address,
		           any(` + seriesattr.StatusCode + `)                 AS status_code
		    FROM optikk.metrics_series
		    PREWHERE team_id = @teamID AND timestamp BETWEEN @start AND @end AND metric_name = 'traces.span.metrics.duration'
		    WHERE attributes.` + "`db.system`" + `::String != ''
		    GROUP BY fingerprint
		)
		SELECT series.db_system                                                AS db_system,
		       sum(m.hist_count)                                               AS query_count,
		       sumIf(m.hist_count, ` + seriesattr.StatusErrorPred + `)         AS error_count,
		       sum(m.hist_sum) / nullIf(sum(m.hist_count), 0)                  AS avg_latency_ms,
		       toFloat32(quantilesPrometheusHistogramMerge(0.95)(m.latency_state)[1]) AS p95_ms,
		       any(series.server_address)                                      AS server_address,
		       max(m.timestamp)                                                AS last_seen
		FROM ` + timebucket.MetricsHistRollup(endMs-startMs) + ` AS m
		INNER JOIN series ON m.fingerprint = series.fingerprint
		PREWHERE m.team_id     = @teamID
		     AND m.timestamp   BETWEEN @start AND @end
		     AND m.metric_name = 'traces.span.metrics.duration'
		GROUP BY db_system
		ORDER BY query_count DESC`

	args := filter.SpanArgs(teamID, startMs, endMs)
	var rows []systemSummaryRawDTO
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "datastoreSystems.GetSystemSummariesRaw", &rows, query, args...)
}

// GetActiveConnectionsBySystem returns active connections by database system.
func (r *Repository) GetActiveConnectionsBySystem(ctx context.Context, teamID, startMs, endMs int64) (map[string]int64, error) {
	startMs, endMs = timebucket.SnapRangeForRollup(startMs, endMs)
	// otelsql emits the connection count as db.sql.connection.open with the
	// state encoded in the metric name (no db.client.connection.state attr), so
	// there is no 'used' state to filter on.
	query := `
		WITH series AS (
		    SELECT fingerprint, any(attributes.` + "`db.system`" + `::String) AS db_system
		    FROM optikk.metrics_series
		    PREWHERE team_id = @teamID AND timestamp BETWEEN @start AND @end AND metric_name = @metricName
		    WHERE attributes.` + "`db.system`" + `::String != ''
		    GROUP BY fingerprint
		)
		SELECT series.db_system AS db_system,
		       ifNotFinite(sum(val_sum) / sum(val_count), 0) AS avg_used
		FROM ` + timebucket.MetricsRollup(endMs-startMs) + ` AS m
		INNER JOIN series ON m.fingerprint = series.fingerprint
		PREWHERE m.team_id     = @teamID
		     AND m.metric_name = @metricName
		     AND m.timestamp   BETWEEN @start AND @end
		GROUP BY db_system`

	args := filter.MetricArgs(teamID, startMs, endMs, filter.MetricDBSQLConnectionOpen)
	var rows []connRawRow
	if err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "datastoreSystems.GetActiveConnectionsBySystem", &rows, query, args...); err != nil {
		return nil, err
	}
	out := make(map[string]int64, len(rows))
	for _, r := range rows {
		out[r.DBSystem] = int64(r.Avg + 0.5)
	}
	return out, nil
}
