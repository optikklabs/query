package redservice

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/shared/chargs"
	"github.com/optikklabs/query/internal/shared/mathutil"
)

type Repository struct {
	db clickhouse.Conn
}

func NewRepository(db clickhouse.Conn) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetServiceREDMetrics(ctx context.Context, teamID int64, startMs, endMs int64, serviceName string) (*redMetricsRow, error) {
	query := `
		WITH active_fps AS (
		    SELECT DISTINCT fingerprint
		    FROM optikk.spans_resource
		    PREWHERE team_id   = @teamID
		         AND ts_bucket BETWEEN @bucketStart AND @bucketEnd
		         AND service   = @serviceName
		)
		SELECT service                                                                               AS service,
		       sumIf(value, metric_name = 'calls')                                                   AS total_count,
		       sumIf(value, metric_name = 'calls' AND JSONExtractString(attributes, 'status.code') = 'STATUS_CODE_ERROR') AS error_count,
		       sumMap(hist_buckets, hist_counts)                                                     AS hist
		FROM optikk.metrics
		PREWHERE team_id     = @teamID
		     AND ts_bucket   BETWEEN @bucketStart AND @bucketEnd
		     AND fingerprint IN active_fps
		     AND timestamp   BETWEEN @start AND @end
		     AND metric_name IN ('calls', 'duration')
		WHERE service = @serviceName
		GROUP BY service`
	var rows []redMetricsRow
	if err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "redservice.GetServiceREDMetrics",
		&rows, query, detailArgs(teamID, startMs, endMs, serviceName)...); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	row := rows[0]
	row.QS = mathutil.Quantiles([]float64{0.5, 0.95, 0.99}, row.HistTuple)
	if len(row.QS) >= 3 {
		row.P50Ms = row.QS[0]
		row.P95Ms = row.QS[1]
		row.P99Ms = row.QS[2]
	}
	return &row, nil
}

func (r *Repository) GetOperationBaseline(ctx context.Context, teamID int64, startMs, endMs int64, serviceName, operationName string) (operationBaselineRow, error) {
	query := `
		WITH active_fps AS (
		    SELECT DISTINCT fingerprint
		    FROM optikk.spans_resource
		    PREWHERE team_id   = @teamID
		         AND ts_bucket BETWEEN @bucketStart AND @bucketEnd
		         AND service   = @serviceName
		)
		SELECT sumIf(value, metric_name = 'calls') AS span_count,
		       sumMap(hist_buckets, hist_counts)   AS hist
		FROM optikk.metrics
		PREWHERE team_id     = @teamID
		     AND ts_bucket   BETWEEN @bucketStart AND @bucketEnd
		     AND fingerprint IN active_fps
		     AND timestamp   BETWEEN @start AND @end
		     AND metric_name IN ('calls', 'duration')
		WHERE service = @serviceName
		  AND JSONExtractString(attributes, 'span.name') = @operationName`
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
	row.QS = mathutil.Quantiles([]float64{0.5, 0.95, 0.99}, row.HistTuple)
	if len(row.QS) >= 3 {
		row.P50Ms = row.QS[0]
		row.P95Ms = row.QS[1]
		row.P99Ms = row.QS[2]
	}
	return row, nil
}

func (r *Repository) GetServiceSaturationAggs(
	ctx context.Context, teamID int64, startMs, endMs int64, serviceName string, metricNames []string,
) ([]serviceMetricRow, error) {
	query := `
		WITH service_hosts AS (
		    SELECT DISTINCT host
		    FROM optikk.metrics
		    PREWHERE team_id     = @teamID
		         AND ts_bucket   BETWEEN @bucketStart AND @bucketEnd
		         AND service     = @serviceName
		         AND host        != ''
		         AND metric_name = 'calls'
		),
		active_fps AS (
		    SELECT fingerprint, any(service) AS service
		    FROM optikk.metrics_resource AS mr
		    PREWHERE team_id     = @teamID
		         AND ts_bucket   BETWEEN @bucketStart AND @bucketEnd
		    WHERE (mr.service = @serviceName OR mr.host IN service_hosts)
		    GROUP BY fingerprint
		)
		SELECT
		    r.service                         AS service,
		    m.metric_name                     AS metric_name,
		    sum(m.val_sum) / sum(m.val_count) AS value
		FROM ` + timebucket.MetricsRollup(endMs-startMs) + ` AS m
		INNER JOIN active_fps AS r ON m.fingerprint = r.fingerprint
		PREWHERE m.team_id     = @teamID
		     AND m.ts_bucket   BETWEEN @bucketStart AND @bucketEnd
		     AND m.metric_name IN @metricNames
		     AND m.timestamp   BETWEEN @start AND @end
		GROUP BY service, metric_name`
	args := append(chargs.RollupRangeArgs(teamID, startMs, endMs),
		clickhouse.Named("serviceName", serviceName),
		clickhouse.Named("metricNames", metricNames),
	)
	var rows []serviceMetricRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "redservice.GetServiceSaturationAggs",
		&rows, query, args...)
}

func (r *Repository) GetServiceSaturationTimeSeries(
	ctx context.Context, teamID int64, startMs, endMs int64, serviceName string, metricNames []string,
) ([]saturationTimeSeriesRawRow, error) {
	grainSQL := timebucket.DisplayGrainSQL(endMs - startMs)
	query := `
		WITH service_hosts AS (
		    SELECT DISTINCT host
		    FROM optikk.metrics
		    PREWHERE team_id     = @teamID
		         AND ts_bucket   BETWEEN @bucketStart AND @bucketEnd
		         AND service     = @serviceName
		         AND host        != ''
		         AND metric_name = 'calls'
		),
		active_fps AS (
		    SELECT fingerprint, any(service) AS service
		    FROM optikk.metrics_resource AS mr
		    PREWHERE team_id     = @teamID
		         AND ts_bucket   BETWEEN @bucketStart AND @bucketEnd
		    WHERE (mr.service = @serviceName OR mr.host IN service_hosts)
		    GROUP BY fingerprint
		)
		SELECT
		    ` + grainSQL + ` AS bucket_at,
		    sum(m.val_sum) / sum(m.val_count) AS value
		FROM ` + timebucket.MetricsRollup(endMs-startMs) + ` AS m
		INNER JOIN active_fps AS r ON m.fingerprint = r.fingerprint
		PREWHERE m.team_id     = @teamID
		     AND m.ts_bucket   BETWEEN @bucketStart AND @bucketEnd
		     AND m.metric_name IN @metricNames
		     AND m.timestamp   BETWEEN @start AND @end
		GROUP BY bucket_at
		ORDER BY bucket_at ASC`
	args := append(chargs.RollupRangeArgs(teamID, startMs, endMs),
		clickhouse.Named("serviceName", serviceName),
		clickhouse.Named("metricNames", metricNames),
	)
	var rows []saturationTimeSeriesRawRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "redservice.GetServiceSaturationTimeSeries",
		&rows, query, args...)
}

func detailArgs(teamID int64, startMs, endMs int64, serviceName string) []any {
	args := chargs.RollupRangeArgs(teamID, startMs, endMs)
	if serviceName != "" {
		args = append(args, clickhouse.Named("serviceName", serviceName))
	}
	return args
}
