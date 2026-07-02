package log_trends //nolint:revive,stylecheck

import (
	"context"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/modules/logs/filter"
)

type Repository struct {
	db clickhouse.Conn
}

func NewRepository(db clickhouse.Conn) *Repository { return &Repository{db: db} }

type SummaryRow struct {
	Total  uint64 `ch:"total"`
	Errors uint64 `ch:"errors"`
	Warns  uint64 `ch:"warns"`
}

// TrendRow is one display-grain bucket carrying total and per-tier counts.
// Tier thresholds mirror the Summary query.
type TrendRow struct {
	TimeBucket time.Time `ch:"time_bucket"`
	Total      uint64    `ch:"total"`
	Error      uint64    `ch:"error"`
	Warn       uint64    `ch:"warn"`
	Info       uint64    `ch:"info"`
	Debug      uint64    `ch:"debug"`
}

func (r *Repository) Summary(ctx context.Context, f filter.Filters) (SummaryRow, error) {
	resourceWhere, where, args := filter.BuildClauses(f)
	cte, prewhereFP := filter.BuildFingerprintCTE(resourceWhere)

	query := cte + `
	SELECT count()                       AS total,
	       countIf(severity_bucket >= 4) AS errors,
	       countIf(severity_bucket = 3)  AS warns
	FROM optikk.logs
	PREWHERE team_id = @teamID` + prewhereFP + `
	     AND timestamp BETWEEN @start AND @end
	WHERE timestamp BETWEEN @start AND @end` + where

	var row SummaryRow
	return row, dbutil.QueryRowCH(dbutil.OverviewCtx(ctx), r.db, "logsTrends.Summary",
		&row, query, args...)
}

func (r *Repository) Trend(ctx context.Context, f filter.Filters) ([]TrendRow, error) {
	resourceWhere, where, args := filter.BuildClauses(f)
	grainSQL := timebucket.DisplayGrainSQL(f.EndMs - f.StartMs)
	cte, prewhereFP := filter.BuildFingerprintCTE(resourceWhere)

	query := cte + `
	SELECT ` + grainSQL + ` AS time_bucket,
	       count()                       AS total,
	       countIf(severity_bucket >= 4) AS error,
	       countIf(severity_bucket = 3)  AS warn,
	       countIf(severity_bucket = 2)  AS info,
	       countIf(severity_bucket <= 1) AS debug
	FROM optikk.logs
	PREWHERE team_id = @teamID` + prewhereFP + `
	     AND timestamp BETWEEN @start AND @end
	WHERE timestamp BETWEEN @start AND @end` + where + `
	GROUP BY time_bucket
	ORDER BY time_bucket ASC`

	var rows []TrendRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "logsTrends.Trend",
		&rows, query, args...)
}
