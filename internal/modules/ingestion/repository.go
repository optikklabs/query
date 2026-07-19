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

// dateCountRow is one calendar day with record and byte counts.
type dateCountRow struct {
	Day   time.Time `ch:"d"`
	Count uint64    `ch:"c"`
	Bytes uint64    `ch:"b"`
}

type signalDateCountRow struct {
	Signal string    `ch:"signal"`
	Day    time.Time `ch:"d"`
	Count  uint64    `ch:"c"`
	Bytes  uint64    `ch:"b"`
}

// svcDateCountRow is one (service, day) bucket with record and byte counts.
type svcDateCountRow struct {
	Day     time.Time `ch:"d"`
	Service string    `ch:"svc"`
	Count   uint64    `ch:"c"`
	Bytes   uint64    `ch:"b"`
}

type serviceUsageRow struct {
	Period  string    `ch:"period"`
	Signal  string    `ch:"signal"`
	Day     time.Time `ch:"d"`
	Service string    `ch:"svc"`
	Env     string    `ch:"env"`
	Count   uint64    `ch:"c"`
	Bytes   uint64    `ch:"b"`
}

// svcCountRow is a per-service aggregate with a representative environment.
type svcCountRow struct {
	Service string `ch:"svc"`
	Env     string `ch:"env"`
	Count   uint64 `ch:"c"`
	Bytes   uint64 `ch:"b"`
}

type nameCountRow struct {
	Name  string `ch:"name"`
	Count uint64 `ch:"c"`
}

type metricCardinalityRow struct {
	Name    string `ch:"name"`
	IsTotal uint64 `ch:"is_total"`
	Count   uint64 `ch:"c"`
}

type scalarRow struct {
	Count uint64 `ch:"c"`
}

// statsArgs binds the meter's named params plus the signal filter.
func statsArgs(tenantID, startMs, endMs int64, signal string) []any {
	return append(chargs.RangeArgs(tenantID, startMs, endMs), clickhouse.Named("signal", signal))
}

// dailyBySignal returns per-day record+byte counts for one signal, read from the
// ingestion_stats meter (long-retained, so month-to-date is always complete).
func (r *Repository) dailyBySignal(ctx context.Context, signal, op string, tenantID, startMs, endMs int64) ([]dateCountRow, error) {
	query := `
	SELECT toDate(bucket_hour) AS d, sum(record_count) AS c, sum(byte_count) AS b
	FROM optikk.ingestion_stats
	PREWHERE tenant_id = @tenantID AND bucket_hour BETWEEN @start AND @end AND signal = @signal
	GROUP BY d
	ORDER BY d`
	var rows []dateCountRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, op, &rows, query, statsArgs(tenantID, startMs, endMs, signal)...)
}

func (r *Repository) DailyLogs(ctx context.Context, tenantID, startMs, endMs int64) ([]dateCountRow, error) {
	return r.dailyBySignal(ctx, "logs", "ingestion.DailyLogs", tenantID, startMs, endMs)
}

func (r *Repository) DailySpans(ctx context.Context, tenantID, startMs, endMs int64) ([]dateCountRow, error) {
	return r.dailyBySignal(ctx, "spans", "ingestion.DailySpans", tenantID, startMs, endMs)
}

func (r *Repository) DailyMetricDatapoints(ctx context.Context, tenantID, startMs, endMs int64) ([]dateCountRow, error) {
	return r.dailyBySignal(ctx, "metrics", "ingestion.DailyMetricDatapoints", tenantID, startMs, endMs)
}

func (r *Repository) DailySignals(ctx context.Context, tenantID, startMs, endMs int64) ([]signalDateCountRow, error) {
	query := `
	SELECT signal, toDate(bucket_hour) AS d,
	       sum(record_count) AS c, sum(byte_count) AS b
	FROM optikk.ingestion_stats
	PREWHERE tenant_id = @tenantID
	     AND bucket_hour BETWEEN @start AND @end
	     AND signal IN @signals
	GROUP BY signal, d
	ORDER BY d, signal`
	args := append(chargs.RangeArgs(tenantID, startMs, endMs),
		clickhouse.Named("signals", []string{"logs", "spans", "metrics"}))
	var rows []signalDateCountRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db,
		"ingestion.DailySignals", &rows, query, args...)
}

func (r *Repository) ServiceUsage(
	ctx context.Context,
	tenantID, priorStartMs, currentStartMs, endMs int64,
) ([]serviceUsageRow, error) {
	query := `
	SELECT if(bucket_hour < @currentStart, 'prior', 'current') AS period,
	       signal, toDate(bucket_hour) AS d, service AS svc,
	       any(environment) AS env, sum(record_count) AS c,
	       sum(byte_count) AS b
	FROM optikk.ingestion_stats
	PREWHERE tenant_id = @tenantID
	     AND bucket_hour BETWEEN @start AND @end
	     AND signal IN @signals
	GROUP BY period, signal, d, svc
	ORDER BY period, d, svc, signal`
	args := append(chargs.RangeArgs(tenantID, priorStartMs, endMs),
		clickhouse.Named("currentStart", time.UnixMilli(currentStartMs)),
		clickhouse.Named("signals", []string{"logs", "spans"}))
	var rows []serviceUsageRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db,
		"ingestion.ServiceUsage", &rows, query, args...)
}

func (r *Repository) MetricCardinality(ctx context.Context, tenantID, startMs, endMs int64) ([]metricCardinalityRow, error) {
	query := `
	SELECT metric_name AS name, grouping(metric_name) AS is_total,
	       uniq(fingerprint) AS c
	FROM optikk.metrics_series
	PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end
	GROUP BY GROUPING SETS ((metric_name), ())
	ORDER BY is_total DESC, c DESC`
	var rows []metricCardinalityRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db,
		"ingestion.MetricCardinality", &rows, query,
		chargs.RangeArgs(tenantID, startMs, endMs)...)
}

// dailyBySignalService returns per-(service, day) record+byte counts for a signal.
func (r *Repository) dailyBySignalService(ctx context.Context, signal, op string, tenantID, startMs, endMs int64) ([]svcDateCountRow, error) {
	query := `
	SELECT toDate(bucket_hour) AS d, service AS svc, sum(record_count) AS c, sum(byte_count) AS b
	FROM optikk.ingestion_stats
	PREWHERE tenant_id = @tenantID AND bucket_hour BETWEEN @start AND @end AND signal = @signal
	GROUP BY d, svc
	ORDER BY d, svc`
	var rows []svcDateCountRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, op, &rows, query, statsArgs(tenantID, startMs, endMs, signal)...)
}

func (r *Repository) DailyLogsByService(ctx context.Context, tenantID, startMs, endMs int64) ([]svcDateCountRow, error) {
	return r.dailyBySignalService(ctx, "logs", "ingestion.DailyLogsByService", tenantID, startMs, endMs)
}

func (r *Repository) DailySpansByService(ctx context.Context, tenantID, startMs, endMs int64) ([]svcDateCountRow, error) {
	return r.dailyBySignalService(ctx, "spans", "ingestion.DailySpansByService", tenantID, startMs, endMs)
}

// serviceTotalsBySignal returns per-service record+byte counts with a
// representative environment (now correct for every signal, not just logs).
func (r *Repository) serviceTotalsBySignal(ctx context.Context, signal, op string, tenantID, startMs, endMs int64) ([]svcCountRow, error) {
	query := `
	SELECT service AS svc, any(environment) AS env, sum(record_count) AS c, sum(byte_count) AS b
	FROM optikk.ingestion_stats
	PREWHERE tenant_id = @tenantID AND bucket_hour BETWEEN @start AND @end AND signal = @signal
	GROUP BY svc
	ORDER BY c DESC`
	var rows []svcCountRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, op, &rows, query, statsArgs(tenantID, startMs, endMs, signal)...)
}

func (r *Repository) ServiceLogTotals(ctx context.Context, tenantID, startMs, endMs int64) ([]svcCountRow, error) {
	return r.serviceTotalsBySignal(ctx, "logs", "ingestion.ServiceLogTotals", tenantID, startMs, endMs)
}

func (r *Repository) ServiceSpanTotals(ctx context.Context, tenantID, startMs, endMs int64) ([]svcCountRow, error) {
	return r.serviceTotalsBySignal(ctx, "spans", "ingestion.ServiceSpanTotals", tenantID, startMs, endMs)
}

// ServiceTimeseries returns active timeseries (distinct fingerprints) per
// service. Cardinality is distinct from volume, so it stays on metrics_series.
func (r *Repository) ServiceTimeseries(ctx context.Context, tenantID, startMs, endMs int64) ([]svcCountRow, error) {
	query := `
	SELECT service AS svc, '' AS env, uniq(fingerprint) AS c, toUInt64(0) AS b
	FROM optikk.metrics_series
	PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end
	GROUP BY svc
	ORDER BY c DESC`
	var rows []svcCountRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "ingestion.ServiceTimeseries", &rows, query, chargs.RangeArgs(tenantID, startMs, endMs)...)
}

// ActiveTimeseries returns distinct metric fingerprints in the window. Sourced
// from metrics_series (30-day TTL); over a longer month this is a lower bound.
func (r *Repository) ActiveTimeseries(ctx context.Context, tenantID, startMs, endMs int64) (uint64, error) {
	query := `
	SELECT uniq(fingerprint) AS c
	FROM optikk.metrics_series
	PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end`
	var row scalarRow
	err := dbutil.QueryRowCH(dbutil.OverviewCtx(ctx), r.db, "ingestion.ActiveTimeseries", &row, query, chargs.RangeArgs(tenantID, startMs, endMs)...)
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
	err := dbutil.QueryRowCH(dbutil.OverviewCtx(ctx), r.db, "ingestion.TopCardinalityMetric", &row, query, chargs.RangeArgs(tenantID, startMs, endMs)...)
	return row, err
}
