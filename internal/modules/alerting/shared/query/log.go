package query

import (
	"context"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
	models "github.com/optikklabs/query/internal/modules/alerting/shared/models"
)

// LogBackend evaluates log monitors against optikk.logs.
type LogBackend struct {
	db clickhouse.Conn
}

func NewLogBackend(db clickhouse.Conn) *LogBackend { return &LogBackend{db: db} }

func (b *LogBackend) Scalar(ctx context.Context, m models.MonitorRow, q models.MonitorQuery, _ models.Scope, _ models.Conditions, now time.Time) (ScalarResult, error) {
	if q.Log == nil {
		return ScalarResult{}, nil
	}
	windowSec := int64(q.Log.WindowSec)
	if windowSec <= 0 {
		windowSec = 300
	}
	endMs := now.UnixMilli()
	startMs := endMs - windowSec*1000

	const query = `
		SELECT count() AS value
		FROM optikk.logs
		PREWHERE tenant_id   = @tenantID
		     AND ts_bucket BETWEEN @bucketStart AND @bucketEnd
		WHERE timestamp BETWEEN @start AND @end
		  AND (@searchTerm = '' OR hasToken(body, @searchTerm))`

	args := logArgs(m.TenantID, q.Log.Query, startMs, endMs)
	var rows []logCountRow
	if err := dbutil.SelectCH(dbutil.DashboardCtx(ctx), b.db, "alerting.log.Scalar", &rows, query, args...); err != nil {
		return ScalarResult{}, err
	}
	if len(rows) == 0 {
		return ScalarResult{HasData: false}, nil
	}
	return ScalarResult{Value: float64(rows[0].Value), HasData: true}, nil
}

func (b *LogBackend) Series(ctx context.Context, m models.MonitorRow, q models.MonitorQuery, _ models.Scope, _ models.Conditions, windowMs int64, now time.Time) ([]Point, error) {
	if q.Log == nil {
		return nil, nil
	}
	endMs := now.UnixMilli()
	startMs := endMs - windowMs

	query := `
		SELECT ` + timebucket.DisplayGrainSQL(windowMs) + ` AS bucket,
		       count() AS value
		FROM optikk.logs
		PREWHERE tenant_id   = @tenantID
		     AND ts_bucket BETWEEN @bucketStart AND @bucketEnd
		WHERE timestamp BETWEEN @start AND @end
		  AND (@searchTerm = '' OR hasToken(body, @searchTerm))
		GROUP BY bucket
		ORDER BY bucket`

	args := logArgs(m.TenantID, q.Log.Query, startMs, endMs)
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
		clickhouse.Named("searchTerm", strings.ToLower(strings.TrimSpace(queryText))),
		clickhouse.Named("start", time.UnixMilli(startMs)),
		clickhouse.Named("end", time.UnixMilli(endMs)),
	}
}

type logCountRow struct {
	Value uint64 `ch:"value"`
}

type logBucketRow struct {
	Bucket time.Time `ch:"bucket"`
	Value  uint64    `ch:"value"`
}
