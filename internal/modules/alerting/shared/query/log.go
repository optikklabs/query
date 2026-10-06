package query

import (
	"context"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"

	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
	models "github.com/optikklabs/query/internal/modules/alerting/shared/models"
	"github.com/optikklabs/query/internal/shared/chargs"
	"github.com/optikklabs/query/internal/shared/filterutil"
)

type LogBackend struct {
	db clickhouse.Conn
}

func NewLogBackend(db clickhouse.Conn) *LogBackend { return &LogBackend{db: db} }

func (b *LogBackend) Scalar(ctx context.Context, m models.MonitorRow, now time.Time) (ScalarResult, error) {
	q := m.Query.Log
	if q == nil {
		return ScalarResult{}, errMissingQuery(m)
	}
	windowSec := int64(q.WindowSec)
	endMs := now.UnixMilli()
	startMs := endMs - windowSec*1000

	query := `
		SELECT count() AS value
		FROM optikk.logs
		PREWHERE tenant_id   = @tenantID
		     AND ts_bucket BETWEEN @bucketStart AND @bucketEnd
		     AND timestamp >= @start AND timestamp < @end
		WHERE (@searchTerm = '' OR lowerUTF8(body) LIKE @searchTerm)`

	scopeSQL, args, err := CompileScope("log", m.Scope, logArgs(m.TenantID, q.Query, startMs, endMs))
	if err != nil {
		return ScalarResult{}, err
	}
	query += scopeSQL
	var row logCountRow
	if err := dbutil.QueryRowCH(dbutil.DashboardCtx(ctx), b.db, "alerting.log.Scalar", &row, query, args...); err != nil {
		return ScalarResult{}, err
	}
	// Zero matching logs is a real value, not missing data.
	return ScalarResult{Value: float64(row.Value), HasData: true}, nil
}

func (b *LogBackend) Series(ctx context.Context, m models.MonitorRow, windowMs int64, now time.Time) ([]Point, error) {
	q := m.Query.Log
	if q == nil {
		return nil, errMissingQuery(m)
	}
	endMs := now.UnixMilli()
	startMs := endMs - windowMs

	scopeSQL, args, err := CompileScope("log", m.Scope, logArgs(m.TenantID, q.Query, startMs, endMs))
	if err != nil {
		return nil, err
	}
	query := `
		SELECT ` + timebucket.DisplayGrainSQL(windowMs) + ` AS bucket,
		       count() AS value
		FROM optikk.logs
		PREWHERE tenant_id   = @tenantID
		     AND ts_bucket BETWEEN @bucketStart AND @bucketEnd
		     AND timestamp >= @start AND timestamp < @end
		WHERE (@searchTerm = '' OR lowerUTF8(body) LIKE @searchTerm)` + scopeSQL + `
		GROUP BY bucket
		ORDER BY bucket`

	var rows []logBucketRow
	if err := dbutil.SelectCH(dbutil.DashboardCtx(ctx), b.db, "alerting.log.Series", &rows, query, args...); err != nil {
		return nil, err
	}
	out := make([]Point, 0, len(rows))
	for _, r := range rows {
		out = append(out, Point{BucketMs: r.Bucket.UnixMilli(), Value: float64(r.Value)})
	}
	return out, nil
}

func logArgs(tenantID int64, queryText string, startMs, endMs int64) []any {
	return []any{
		tenantIDArg(tenantID),
		clickhouse.Named("searchTerm", filterutil.LikeSubstringPattern(strings.TrimSpace(queryText))),
		chargs.Millis("start", startMs),
		chargs.Millis("end", endMs),
		clickhouse.Named("bucketStart", timebucket.LogBucket(startMs)),
		clickhouse.Named("bucketEnd", timebucket.LogBucket(endMs)),
	}
}

type logCountRow struct {
	Value uint64 `ch:"value"`
}

type logBucketRow struct {
	Bucket time.Time `ch:"bucket"`
	Value  uint64    `ch:"value"`
}
