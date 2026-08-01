package repository

import (
	"context"
	"time"

	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/modules/logs/filter"
)

type SummaryRow struct {
	Total  uint64 `ch:"total"`
	Errors uint64 `ch:"errors"`
	Warns  uint64 `ch:"warns"`
}

type TrendRow struct {
	TimeBucket time.Time `ch:"time_bucket"`
	Total      uint64    `ch:"total"`
	Error      uint64    `ch:"error"`
	Warn       uint64    `ch:"warn"`
	Info       uint64    `ch:"info"`
	Debug      uint64    `ch:"debug"`
}

func (r *Repository) Summary(ctx context.Context, f filter.Filters) (SummaryRow, error) {
	source, args := aggregateSource(f, "severity_bucket", "severity_bucket", "severity_bucket")
	query := `
	SELECT sum(log_count)                         AS total,
	       sumIf(log_count, severity_bucket >= 4) AS errors,
	       sumIf(log_count, severity_bucket = 3)  AS warns
	FROM ` + source

	var row SummaryRow
	return row, dbutil.QueryRowCH(dbutil.OverviewCtx(ctx), r.db, "logsTrends.Summary",
		&row, query, args...)
}

func (r *Repository) Trend(ctx context.Context, f filter.Filters) ([]TrendRow, error) {
	grainSQL := timebucket.DisplayGrainSQL(f.EndMs - f.StartMs)
	source, args := aggregateSource(f, "toStartOfMinute(timestamp) AS timestamp, severity_bucket",
		"timestamp, severity_bucket", "timestamp, severity_bucket")
	query := `
	SELECT ` + grainSQL + `                        AS time_bucket,
	       sum(log_count)                         AS total,
	       sumIf(log_count, severity_bucket >= 4) AS error,
	       sumIf(log_count, severity_bucket = 3)  AS warn,
	       sumIf(log_count, severity_bucket = 2)  AS info,
	       sumIf(log_count, severity_bucket <= 1) AS debug
	FROM ` + source + `
	GROUP BY time_bucket
	ORDER BY time_bucket ASC`

	var rows []TrendRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "logsTrends.Trend",
		&rows, query, args...)
}
