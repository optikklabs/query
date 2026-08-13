package query

import (
	"context"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
	models "github.com/optikklabs/query/internal/modules/alerting/shared/models"
	"github.com/optikklabs/query/internal/shared/filterutil"
)

type LogBackend struct {
	db clickhouse.Conn
}

const logBucketSeconds int64 = 300

func NewLogBackend(db clickhouse.Conn) *LogBackend { return &LogBackend{db: db} }

func (b *LogBackend) Scalar(ctx context.Context, m models.MonitorRow, q models.MonitorQuery, scope models.Scope, _ models.Conditions, now time.Time) (ScalarResult, error) {
	if q.Log == nil {
		return ScalarResult{}, nil
	}
	windowSec := monitorWindowSec(q.Log.WindowSec)
	endMs := now.UnixMilli()
	startMs := endMs - windowSec*1000

	query := `
		SELECT count() AS value
		FROM optikk.logs
		PREWHERE tenant_id   = @tenantID
		     AND ts_bucket BETWEEN @bucketStart AND @bucketEnd
		     AND timestamp >= @start AND timestamp < @end
		WHERE (@searchTerm = '' OR lowerUTF8(body) LIKE @searchTerm)`

	scopeSQL, args, err := CompileScope("log", scope, logArgs(m.TenantID, q.Log.Query, startMs, endMs))
	if err != nil {
		return ScalarResult{}, err
	}
	query += scopeSQL
	var rows []logCountRow
	if err := dbutil.SelectCH(dbutil.DashboardCtx(ctx), b.db, "alerting.log.Scalar", &rows, query, args...); err != nil {
		return ScalarResult{}, err
	}
	if len(rows) == 0 {
		return ScalarResult{HasData: false}, nil
	}
	return ScalarResult{Value: float64(rows[0].Value), HasData: true}, nil
}

func (b *LogBackend) Series(ctx context.Context, m models.MonitorRow, q models.MonitorQuery, scope models.Scope, _ models.Conditions, windowMs int64, now time.Time) ([]Point, error) {
	if q.Log == nil {
		return nil, nil
	}
	endMs := now.UnixMilli()
	startMs := endMs - windowMs

	scopeSQL, args, err := CompileScope("log", scope, logArgs(m.TenantID, q.Log.Query, startMs, endMs))
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
	bucketStart, bucketEnd := logBucketBounds(startMs, endMs)
	return []any{
		tenantIDArg(tenantID),
		clickhouse.Named("searchTerm", filterutil.LikeSubstringPattern(strings.TrimSpace(queryText))),
		clickhouse.Named("start", time.UnixMilli(startMs)),
		clickhouse.Named("end", time.UnixMilli(endMs)),
		clickhouse.Named("bucketStart", bucketStart),
		clickhouse.Named("bucketEnd", bucketEnd),
	}
}

func logBucketBounds(startMs, endMs int64) (time.Time, time.Time) {
	return time.UnixMilli(timebucket.FloorMsToBucket(startMs, logBucketSeconds)),
		time.UnixMilli(timebucket.FloorMsToBucket(endMs, logBucketSeconds))
}

type logCountRow struct {
	Value uint64 `ch:"value"`
}

type logBucketRow struct {
	Bucket time.Time `ch:"bucket"`
	Value  uint64    `ch:"value"`
}
