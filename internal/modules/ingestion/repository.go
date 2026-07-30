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

func (r *Repository) DailySignals(ctx context.Context, tenantID, startMs, endMs int64) ([]signalDateCountRow, error) {
	query := `
	SELECT signal, toDate(bucket_hour) AS d,
	       sum(record_count) AS c, sum(byte_count) AS b
	FROM optikk.ingestion_stats
	PREWHERE tenant_id = @tenantID
	     AND bucket_hour >= @start AND bucket_hour < @end
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
	       argMax(environment, (bucket_hour, environment)) AS env, sum(record_count) AS c,
	       sum(byte_count) AS b
	FROM optikk.ingestion_stats
	PREWHERE tenant_id = @tenantID
	     AND bucket_hour >= @start AND bucket_hour < @end
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
	       uniqExact(fingerprint) AS c
	FROM optikk.metrics_series
	PREWHERE tenant_id = @tenantID AND timestamp >= @start AND timestamp < @end
	GROUP BY GROUPING SETS ((metric_name), ())
	ORDER BY is_total DESC, c DESC, name ASC`
	var rows []metricCardinalityRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db,
		"ingestion.MetricCardinality", &rows, query,
		chargs.RangeArgs(tenantID, startMs, endMs)...)
}

func (r *Repository) ServiceTimeseries(ctx context.Context, tenantID, startMs, endMs int64) ([]svcCountRow, error) {
	query := `
	SELECT service AS svc, '' AS env, uniqExact(fingerprint) AS c, toUInt64(0) AS b
	FROM optikk.metrics_series
	PREWHERE tenant_id = @tenantID AND timestamp >= @start AND timestamp < @end
	GROUP BY svc
	ORDER BY c DESC, svc ASC`
	var rows []svcCountRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "ingestion.ServiceTimeseries", &rows, query, chargs.RangeArgs(tenantID, startMs, endMs)...)
}
