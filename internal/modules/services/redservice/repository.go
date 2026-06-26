package redservice

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

// NewRepository returns a new instance of Repository.
func NewRepository(db clickhouse.Conn) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetServiceREDMetrics(ctx context.Context, teamID int64, startMs, endMs int64, serviceName string) (*redMetricsRow, error) {
	query := `
		WITH series AS (
		    SELECT fingerprint,
		           any(service)                       AS service,
		           any(` + seriesattr.StatusCode + `) AS status_code
		    FROM optikk.metrics_series AS s
		    PREWHERE team_id = @teamID AND timestamp BETWEEN @start AND @end AND metric_name = 'traces.span.metrics.duration'
		    WHERE s.service = @serviceName AND ` + seriesattr.ServerKindPred + `
		    GROUP BY fingerprint
		)
		SELECT any(series.service)                                          AS service,
		       sum(m.hist_count)                                           AS total_count,
		       sumIf(m.hist_count, ` + seriesattr.StatusErrorPred + `)     AS error_count,
		       quantilesPrometheusHistogramMerge(0.5, 0.95, 0.99)(m.latency_state) AS qs
		FROM ` + timebucket.MetricsHistRollup(endMs-startMs) + ` AS m
		INNER JOIN series ON m.fingerprint = series.fingerprint
		PREWHERE m.team_id     = @teamID
		     AND m.timestamp   BETWEEN @start AND @end
		     AND m.metric_name = 'traces.span.metrics.duration'`
	var rows []redMetricsRow
	if err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "redservice.GetServiceREDMetrics",
		&rows, query, detailArgs(teamID, startMs, endMs, serviceName)...); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	row := rows[0]
	if len(row.QS) >= 3 {
		row.P50Ms = float32(row.QS[0])
		row.P95Ms = float32(row.QS[1])
		row.P99Ms = float32(row.QS[2])
	}
	return &row, nil
}

func (r *Repository) GetOperationBaseline(ctx context.Context, teamID int64, startMs, endMs int64, serviceName, operationName string) (operationBaselineRow, error) {
	query := `
		WITH series AS (
		    SELECT fingerprint
		    FROM optikk.metrics_series
		    PREWHERE team_id = @teamID AND timestamp BETWEEN @start AND @end AND metric_name = 'traces.span.metrics.duration'
		    WHERE ` + seriesattr.SpanName + ` = @operationName AND service = @serviceName
		    GROUP BY fingerprint
		)
		SELECT sum(m.hist_count)                                            AS span_count,
		       quantilesPrometheusHistogramMerge(0.5, 0.95, 0.99)(m.latency_state) AS qs
		FROM ` + timebucket.MetricsHistRollup(endMs-startMs) + ` AS m
		INNER JOIN series ON m.fingerprint = series.fingerprint
		PREWHERE m.team_id     = @teamID
		     AND m.timestamp   BETWEEN @start AND @end
		     AND m.metric_name = 'traces.span.metrics.duration'`
	args := append(chargs.RollupRangeArgs(teamID, startMs, endMs),
		clickhouse.Named("serviceName", serviceName),
		clickhouse.Named("operationName", operationName),
	)
	var rows []operationBaselineRow
	if err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "redservice.GetOperationBaseline",
		&rows, query, args...); err != nil {
		return operationBaselineRow{}, err
	}
	if len(rows) == 0 {
		return operationBaselineRow{}, nil
	}
	row := rows[0]
	if len(row.QS) >= 3 {
		row.P50Ms = float32(row.QS[0])
		row.P95Ms = float32(row.QS[1])
		row.P99Ms = float32(row.QS[2])
	}
	return row, nil
}

func (r *Repository) GetServiceSaturationAggs(
	ctx context.Context, teamID int64, startMs, endMs int64, serviceName string, metricNames []string,
) ([]serviceMetricRow, error) {

	hostQuery := `
		SELECT DISTINCT host
		FROM optikk.metrics_series
		PREWHERE team_id     = @teamID
		     AND metric_name = 'traces.span.metrics.duration'
		WHERE service = @serviceName
		  AND host    != ''`
	var hostRows []struct {
		Host string `ch:"host"`
	}
	hostArgs := []any{
		clickhouse.Named("teamID", uint32(teamID)),
		clickhouse.Named("serviceName", serviceName),
	}
	if err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "redservice.GetServiceHosts",
		&hostRows, hostQuery, hostArgs...); err != nil {
		return nil, err
	}

	hosts := make([]string, len(hostRows))
	for i, row := range hostRows {
		hosts[i] = row.Host
	}

	query := `
		SELECT
		    @serviceName                      AS service,
		    m.metric_name                     AS metric_name,
		    sum(m.val_sum) / sum(m.val_count) AS value
		FROM ` + timebucket.MetricsRollup(endMs-startMs) + ` AS m
		INNER JOIN optikk.metrics_series AS s ON m.fingerprint = s.fingerprint
		PREWHERE m.team_id     = @teamID
		     AND m.metric_name IN @metricNames
		     AND m.timestamp   BETWEEN @start AND @end
		WHERE s.service = @serviceName
		   OR (s.host != '' AND s.host IN @hosts)
		GROUP BY metric_name`

	args := append(chargs.RollupRangeArgs(teamID, startMs, endMs),
		clickhouse.Named("serviceName", serviceName),
		clickhouse.Named("metricNames", metricNames),
		clickhouse.Named("hosts", hosts),
	)
	var rows []serviceMetricRow
	if err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "redservice.GetServiceSaturationAggs",
		&rows, query, args...); err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *Repository) GetServiceSaturationTimeSeries(
	ctx context.Context, teamID int64, startMs, endMs int64, serviceName string, metricNames []string,
) ([]saturationTimeSeriesRawRow, error) {

	hostQuery := `
		SELECT DISTINCT host
		FROM optikk.metrics_series
		PREWHERE team_id     = @teamID
		     AND metric_name = 'traces.span.metrics.duration'
		WHERE service = @serviceName
		  AND host    != ''`
	var hostRows []struct {
		Host string `ch:"host"`
	}
	hostArgs := []any{
		clickhouse.Named("teamID", uint32(teamID)),
		clickhouse.Named("serviceName", serviceName),
	}
	if err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "redservice.GetServiceHosts",
		&hostRows, hostQuery, hostArgs...); err != nil {
		return nil, err
	}

	hosts := make([]string, len(hostRows))
	for i, row := range hostRows {
		hosts[i] = row.Host
	}

	grainSQL := timebucket.DisplayGrainSQL(endMs - startMs)
	query := `
		SELECT
		    ` + grainSQL + ` AS bucket_at,
		    sum(m.val_sum) / sum(m.val_count) AS value
		FROM ` + timebucket.MetricsRollup(endMs-startMs) + ` AS m
		INNER JOIN optikk.metrics_series AS s ON m.fingerprint = s.fingerprint
		PREWHERE m.team_id     = @teamID
		     AND m.metric_name IN @metricNames
		     AND m.timestamp   BETWEEN @start AND @end
		WHERE s.service = @serviceName
		   OR (s.host != '' AND s.host IN @hosts)
		GROUP BY bucket_at
		ORDER BY bucket_at ASC`

	args := append(chargs.RollupRangeArgs(teamID, startMs, endMs),
		clickhouse.Named("serviceName", serviceName),
		clickhouse.Named("metricNames", metricNames),
		clickhouse.Named("hosts", hosts),
	)
	var rows []saturationTimeSeriesRawRow
	if err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "redservice.GetServiceSaturationTimeSeries",
		&rows, query, args...); err != nil {
		return nil, err
	}
	return rows, nil
}

func detailArgs(teamID int64, startMs, endMs int64, serviceName string) []any {
	args := chargs.RollupRangeArgs(teamID, startMs, endMs)
	if serviceName != "" {
		args = append(args, clickhouse.Named("serviceName", serviceName))
	}
	return args
}
