package repository

import (
	"context"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/modules/logs/filter"
	"github.com/optikklabs/query/internal/modules/logs/models"
	"github.com/optikklabs/query/internal/shared/filterutil"
)

func (r *Repository) ListLogs(ctx context.Context, f filter.Filters, limit int, cur models.Cursor) ([]models.LogRow, bool, error) {
	prewhere, where, args := filter.BuildClauses(f)
	if !cur.IsZero() {
		where += ` AND (timestamp, log_id) < (@curTs, @curLid)`
		args = append(args,

			clickhouse.DateNamed("curTs", cur.Timestamp, clickhouse.NanoSeconds),
			clickhouse.Named("curLid", cur.LogID),
		)
	}
	args = append(args, clickhouse.Named("pgLimit", uint64(limit+1)))

	query := `
		SELECT ` + models.LogColumns + `
		FROM optikk.logs
		` + prewhere + ` ` + where + `
		ORDER BY ts_bucket DESC, timestamp DESC, log_id DESC
		LIMIT @pgLimit`

	var rows []models.LogRow
	if err := dbutil.SelectCH(dbutil.ExplorerCtx(ctx), r.db, "logs.ListLogs", &rows, query, args...); err != nil {
		return nil, false, err
	}

	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	return rows, hasMore, nil
}

var suggestResourceColumns = map[string]string{
	"service_name": "service",
	"host":         "host",
	"pod":          "pod",
	"container":    "container",
	"environment":  "environment",
}

func IsSuggestableScalarField(field string) bool {
	if field == "severity_text" {
		return true
	}
	_, ok := suggestResourceColumns[field]
	return ok
}

func (r *Repository) SuggestScalar(ctx context.Context, tenantID, startMs, endMs int64, field, prefix string, limit int) ([]models.Suggestion, error) {
	if field == "severity_text" {
		return r.suggestSeverity(ctx, tenantID, startMs, endMs, prefix, limit)
	}
	column, ok := suggestResourceColumns[field]
	if !ok {
		return nil, nil
	}
	query := `
		SELECT ` + column + `        AS value,
		       count()               AS count
		FROM optikk.logs
		PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end AND ts_bucket BETWEEN @startBucket AND @endBucket
		WHERE value != ''
		  AND (length(@prefix) = 0 OR positionCaseInsensitive(value, @prefix) > 0)
		GROUP BY value
		ORDER BY count DESC
		LIMIT @limit`
	return r.runSuggest(ctx, "suggest.SuggestResource", query, suggestArgs(tenantID, startMs, endMs, prefix, limit))
}

func (r *Repository) suggestSeverity(ctx context.Context, tenantID, startMs, endMs int64, prefix string, limit int) ([]models.Suggestion, error) {
	const query = `
		SELECT upper(severity_text)  AS value,
		       count()               AS count
		FROM optikk.logs
		PREWHERE tenant_id = @tenantID
		WHERE ts_bucket BETWEEN @startBucket AND @endBucket
		  AND value != ''
		  AND (length(@prefix) = 0 OR positionCaseInsensitive(value, @prefix) > 0)
		GROUP BY value
		ORDER BY count DESC
		LIMIT @limit`
	return r.runSuggest(ctx, "suggest.SuggestSeverity", query, suggestArgs(tenantID, startMs, endMs, prefix, limit))
}

func (r *Repository) SuggestAttribute(ctx context.Context, tenantID, startMs, endMs int64, attrKey, prefix string, limit int) ([]models.Suggestion, error) {
	const query = `
		SELECT attributes_string[@attrKey] AS value,
		       count()                     AS count
		FROM optikk.logs
		PREWHERE tenant_id = @tenantID
		WHERE ts_bucket BETWEEN @startBucket AND @endBucket
		  AND value != ''
		  AND (length(@prefix) = 0 OR positionCaseInsensitive(value, @prefix) > 0)
		GROUP BY value
		ORDER BY count DESC
		LIMIT @limit`
	args := append(suggestArgs(tenantID, startMs, endMs, prefix, limit),
		clickhouse.Named("attrKey", strings.TrimPrefix(attrKey, "@")),
	)
	return r.runSuggest(ctx, "suggest.SuggestAttribute", query, args)
}

func (r *Repository) runSuggest(ctx context.Context, op, query string, args []any) ([]models.Suggestion, error) {
	var rows []filterutil.SuggestionRow
	if err := dbutil.SelectCH(dbutil.DashboardCtx(ctx), r.db, op, &rows, query, args...); err != nil {
		return nil, err
	}
	return filterutil.MapSuggestionRows(rows), nil
}

func suggestArgs(tenantID, startMs, endMs int64, prefix string, limit int) []any {
	return []any{
		clickhouse.Named("tenantID", uint32(tenantID)),
		clickhouse.Named("start", time.UnixMilli(startMs)),
		clickhouse.Named("end", time.UnixMilli(endMs)),
		clickhouse.Named("startBucket", uint32((startMs/1000)/300*300)),
		clickhouse.Named("endBucket", uint32((endMs/1000)/300*300)),
		clickhouse.Named("prefix", prefix),
		clickhouse.Named("limit", uint64(limit)),
	}
}
