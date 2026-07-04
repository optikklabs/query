package ingestion

import (
	"context"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/shared/chargs"
)

type Repository struct {
	db clickhouse.Conn
}

func NewRepository(db clickhouse.Conn) *Repository { return &Repository{db: db} }

// dateCountRow is one calendar day with a record count.
type dateCountRow struct {
	Day   time.Time `ch:"d"`
	Count uint64    `ch:"c"`
}

// svcDateCountRow is one (service, day) bucket with a record count.
type svcDateCountRow struct {
	Day     time.Time `ch:"d"`
	Service string    `ch:"svc"`
	Count   uint64    `ch:"c"`
}

// svcCountRow is a per-service aggregate; Env is populated only by the logs query.
type svcCountRow struct {
	Service string `ch:"svc"`
	Env     string `ch:"env"`
	Count   uint64 `ch:"c"`
}

type nameCountRow struct {
	Name  string `ch:"name"`
	Count uint64 `ch:"c"`
}

type scalarRow struct {
	Count uint64 `ch:"c"`
}

// metricsArgs binds only the params the rollup-free metrics tables use (no ts_bucket).
func metricsArgs(tenantID, startMs, endMs int64) []any {
	return []any{
		clickhouse.Named("tenantID", uint32(tenantID)),
		clickhouse.Named("start", time.UnixMilli(startMs)),
		clickhouse.Named("end", time.UnixMilli(endMs)),
	}
}

const eventDailyTail = `
	GROUP BY d
	ORDER BY d`

// dailyEvents returns per-day record counts for an event table (logs or spans).
func (r *Repository) dailyEvents(ctx context.Context, table, op string, tenantID, startMs, endMs int64) ([]dateCountRow, error) {
	query := `
	SELECT toDate(timestamp) AS d, count() AS c
	FROM optikk.` + table + `
	PREWHERE tenant_id = @tenantID
	     AND timestamp BETWEEN @start AND @end
	WHERE timestamp BETWEEN @start AND @end` + eventDailyTail
	var rows []dateCountRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, op, &rows, query, chargs.RangeArgs(tenantID, startMs, endMs)...)
}

func (r *Repository) DailyLogs(ctx context.Context, tenantID, startMs, endMs int64) ([]dateCountRow, error) {
	return r.dailyEvents(ctx, "logs", "ingestion.DailyLogs", tenantID, startMs, endMs)
}

func (r *Repository) DailySpans(ctx context.Context, tenantID, startMs, endMs int64) ([]dateCountRow, error) {
	return r.dailyEvents(ctx, "spans", "ingestion.DailySpans", tenantID, startMs, endMs)
}

// DailyMetricDatapoints returns per-day metric datapoint counts (no ts_bucket on this table).
func (r *Repository) DailyMetricDatapoints(ctx context.Context, tenantID, startMs, endMs int64) ([]dateCountRow, error) {
	query := `
	SELECT toDate(timestamp) AS d, count() AS c
	FROM optikk.metrics
	PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end` + eventDailyTail
	var rows []dateCountRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "ingestion.DailyMetricDatapoints", &rows, query, metricsArgs(tenantID, startMs, endMs)...)
}

// dailyEventsByService returns per-(service, day) counts for an event table.
func (r *Repository) dailyEventsByService(ctx context.Context, table, op string, tenantID, startMs, endMs int64) ([]svcDateCountRow, error) {
	query := `
	SELECT toDate(timestamp) AS d, service AS svc, count() AS c
	FROM optikk.` + table + `
	PREWHERE tenant_id = @tenantID
	     AND timestamp BETWEEN @start AND @end
	WHERE timestamp BETWEEN @start AND @end
	GROUP BY d, svc
	ORDER BY d, svc`
	var rows []svcDateCountRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, op, &rows, query, chargs.RangeArgs(tenantID, startMs, endMs)...)
}

func (r *Repository) DailyLogsByService(ctx context.Context, tenantID, startMs, endMs int64) ([]svcDateCountRow, error) {
	return r.dailyEventsByService(ctx, "logs", "ingestion.DailyLogsByService", tenantID, startMs, endMs)
}

func (r *Repository) DailySpansByService(ctx context.Context, tenantID, startMs, endMs int64) ([]svcDateCountRow, error) {
	return r.dailyEventsByService(ctx, "spans", "ingestion.DailySpansByService", tenantID, startMs, endMs)
}

// ServiceLogTotals returns per-service log counts plus a representative environment.
func (r *Repository) ServiceLogTotals(ctx context.Context, tenantID, startMs, endMs int64) ([]svcCountRow, error) {
	query := `
	SELECT service AS svc, any(environment) AS env, count() AS c
	FROM optikk.logs
	PREWHERE tenant_id = @tenantID
	     AND timestamp BETWEEN @start AND @end
	WHERE timestamp BETWEEN @start AND @end
	GROUP BY svc
	ORDER BY c DESC`
	var rows []svcCountRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "ingestion.ServiceLogTotals", &rows, query, chargs.RangeArgs(tenantID, startMs, endMs)...)
}

// ServiceSpanTotals returns per-service span counts.
func (r *Repository) ServiceSpanTotals(ctx context.Context, tenantID, startMs, endMs int64) ([]svcCountRow, error) {
	query := `
	SELECT service AS svc, '' AS env, count() AS c
	FROM optikk.spans
	PREWHERE tenant_id = @tenantID
	     AND timestamp BETWEEN @start AND @end
	WHERE timestamp BETWEEN @start AND @end
	GROUP BY svc
	ORDER BY c DESC`
	var rows []svcCountRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "ingestion.ServiceSpanTotals", &rows, query, chargs.RangeArgs(tenantID, startMs, endMs)...)
}

// ServiceTimeseries returns active timeseries (distinct fingerprints) per service.
func (r *Repository) ServiceTimeseries(ctx context.Context, tenantID, startMs, endMs int64) ([]svcCountRow, error) {
	query := `
	SELECT service AS svc, '' AS env, uniq(fingerprint) AS c
	FROM optikk.metrics_series
	PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end
	GROUP BY svc
	ORDER BY c DESC`
	var rows []svcCountRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "ingestion.ServiceTimeseries", &rows, query, metricsArgs(tenantID, startMs, endMs)...)
}

// ActiveTimeseries returns the count of distinct metric fingerprints in the window.
func (r *Repository) ActiveTimeseries(ctx context.Context, tenantID, startMs, endMs int64) (uint64, error) {
	query := `
	SELECT uniq(fingerprint) AS c
	FROM optikk.metrics_series
	PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end`
	var row scalarRow
	err := dbutil.QueryRowCH(dbutil.OverviewCtx(ctx), r.db, "ingestion.ActiveTimeseries", &row, query, metricsArgs(tenantID, startMs, endMs)...)
	return row.Count, err
}

// TopCardinalityMetric returns the metric name with the most distinct timeseries.
func (r *Repository) TopCardinalityMetric(ctx context.Context, tenantID, startMs, endMs int64) (nameCountRow, error) {
	query := `
	SELECT metric_name AS name, uniq(fingerprint) AS c
	FROM optikk.metrics_series
	PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end
	GROUP BY name
	ORDER BY c DESC
	LIMIT 1`
	var row nameCountRow
	err := dbutil.QueryRowCH(dbutil.OverviewCtx(ctx), r.db, "ingestion.TopCardinalityMetric", &row, query, metricsArgs(tenantID, startMs, endMs)...)
	return row, err
}
