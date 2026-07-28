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
	prewhere, where, args := filter.BuildClauses(f)

	query := `
	SELECT count()                       AS total,
	       countIf(severity_bucket >= 4) AS errors,
	       countIf(severity_bucket = 3)  AS warns
	FROM optikk.logs
	` + prewhere + ` ` + where

	var row SummaryRow
	return row, dbutil.QueryRowCH(dbutil.OverviewCtx(ctx), r.db, "logsTrends.Summary",
		&row, query, args...)
}

func (r *Repository) Trend(ctx context.Context, f filter.Filters) ([]TrendRow, error) {
	prewhere, where, args := filter.BuildClauses(f)
	grainSQL := timebucket.DisplayGrainSQL(f.EndMs - f.StartMs)

	query := `
	SELECT ` + grainSQL + ` AS time_bucket,
	       count()                       AS total,
	       countIf(severity_bucket >= 4) AS error,
	       countIf(severity_bucket = 3)  AS warn,
	       countIf(severity_bucket = 2)  AS info,
	       countIf(severity_bucket <= 1) AS debug
	FROM optikk.logs
	` + prewhere + ` ` + where + `
	GROUP BY time_bucket
	ORDER BY time_bucket ASC`

	var rows []TrendRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "logsTrends.Trend",
		&rows, query, args...)
}
