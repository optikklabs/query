package volume

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/modules/saturation/database/filter"
)

type Repository struct {
	db clickhouse.Conn
}

func NewRepository(db clickhouse.Conn) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetOpsBySystem(ctx context.Context, tenantID, startMs, endMs int64, f filter.Filters) ([]opsRawDTO, error) {
	return r.opsSeriesByGroup(ctx, tenantID, startMs, endMs, f, filter.AttrDBSystem, "volume.GetOpsBySystem")
}

func (r *Repository) opsSeriesByGroup(ctx context.Context, tenantID, startMs, endMs int64, f filter.Filters, attr, traceLabel string) ([]opsRawDTO, error) {
	startMs, endMs = timebucket.SnapRangeForRollup(startMs, endMs)
	groupCol := filter.MetricsGroupColumn(attr)
	if groupCol == "" {
		return nil, nil
	}
	filterWhere, filterArgs := filter.BuildMetricsClauses(f)

	query := `
		WITH series AS (
		    SELECT fingerprint, ` + groupCol + ` AS group_by
		    FROM optikk.metrics_series
		    PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end AND metric_name = 'traces.span.metrics.duration'
		    WHERE attributes.` + "`db.system`" + `::String != ''` + filterWhere + `
		    GROUP BY fingerprint, group_by
		)
		SELECT ` + timebucket.DisplayGrainSQL(endMs-startMs) + `                   AS time_bucket,
		       series.group_by                                                     AS group_by,
		       sum(m.hist_count) / @bucketGrainSec                                 AS ops_per_sec
		FROM ` + timebucket.MetricsHistRollup(endMs-startMs) + ` AS m
		INNER JOIN series ON m.fingerprint = series.fingerprint
		PREWHERE m.tenant_id     = @tenantID
		     AND m.timestamp   BETWEEN @start AND @end
		     AND m.metric_name = 'traces.span.metrics.duration'
		GROUP BY time_bucket, group_by
		ORDER BY time_bucket, group_by`

	args := append(filter.SpanArgs(tenantID, startMs, endMs), filterArgs...)
	args = timebucket.WithBucketGrainSec(args, startMs, endMs)
	var rows []opsRawDTO
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, traceLabel, &rows, query, args...)
}
