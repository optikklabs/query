package users

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2"

	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/modules/llm/pricing"
	"github.com/optikklabs/query/internal/modules/llm/scores"
	"github.com/optikklabs/query/internal/shared/chargs"
)

type Repository struct {
	db clickhouse.Conn
}

func NewRepository(db clickhouse.Conn) *Repository {
	return &Repository{db: db}
}

func (r *Repository) TopUsers(ctx context.Context, tenantID, startMs, endMs int64, limit int) ([]userRow, error) {
	query := `
		SELECT llm_user_id AS user_id,
		       arrayElement(topK(1)(service), 1) AS top_service,
		       uniqExact(trace_id) AS traces,
		       sum(gen_ai_input_tokens + gen_ai_output_tokens) AS tokens,
		       sum(` + pricing.SpanCostSQL + `) AS cost,
		       max(timestamp) AS last_seen
		FROM optikk.llm_spans
		PREWHERE tenant_id = @tenantID AND timestamp >= @start AND timestamp < @end
		WHERE llm_user_id != ''
		GROUP BY user_id
		ORDER BY cost DESC, user_id ASC
		LIMIT @limit`
	args := append(chargs.RangeArgs(tenantID, startMs, endMs), pricing.Args()...)
	args = append(args, clickhouse.Named("limit", uint64(limit)))
	var rows []userRow
	err := dbutil.SelectCH(dbutil.ExplorerCtx(ctx), r.db, "llm.users.TopUsers", &rows, query, args...)
	return rows, err
}

func (r *Repository) Overview(ctx context.Context, tenantID, startMs, endMs int64) (overviewRow, error) {
	query := `
		SELECT uniqExact(llm_user_id) AS active_users,
		       uniqExact(trace_id)     AS traces,
		       sum(` + pricing.SpanCostSQL + `) AS cost
		FROM optikk.llm_spans
		PREWHERE tenant_id = @tenantID AND timestamp >= @start AND timestamp < @end
		WHERE llm_user_id != ''`
	args := append(chargs.RangeArgs(tenantID, startMs, endMs), pricing.Args()...)
	var row overviewRow
	err := dbutil.QueryRowCH(dbutil.OverviewCtx(ctx), r.db, "llm.users.Overview", &row, query, args...)
	return row, err
}

func (r *Repository) MeanScoreByUser(ctx context.Context, tenantID, startMs, endMs int64, userIDs []string) (map[string]*float64, error) {
	return scores.MeanScores(ctx, r.db, "llm.users.MeanScoreByUser", scores.ByUser, tenantID, startMs, endMs, userIDs)
}

func (r *Repository) LowScoreUserCount(ctx context.Context, tenantID, startMs, endMs int64, threshold float64) (uint64, error) {
	query := `
		SELECT countIf(mean < @threshold) AS low_score
		FROM (
		    SELECT avg(value) AS mean
		    FROM optikk.llm_scores
		    PREWHERE tenant_id = @tenantID AND timestamp >= @start AND timestamp < @end
		    WHERE user_id != '' AND data_type = 'numeric'
		    GROUP BY user_id
		)`
	args := append(chargs.RangeArgs(tenantID, startMs, endMs),
		clickhouse.Named("threshold", threshold))
	var row struct {
		LowScore uint64 `ch:"low_score"`
	}
	if err := dbutil.QueryRowCH(dbutil.OverviewCtx(ctx), r.db, "llm.users.LowScoreUserCount", &row, query, args...); err != nil {
		return 0, err
	}
	return row.LowScore, nil
}
