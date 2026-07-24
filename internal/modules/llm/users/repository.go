package users

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/modules/llm/pricing"
	"github.com/optikklabs/query/internal/shared/chargs"
)

type Repository struct {
	db clickhouse.Conn
}

func NewRepository(db clickhouse.Conn) *Repository {
	return &Repository{db: db}
}

// TopUsers ranks users by cost over the window from raw gen_ai spans. Cost is
// priced per-model at query time so it stays re-priceable.
func (r *Repository) TopUsers(ctx context.Context, tenantID, startMs, endMs int64, limit int) ([]userRow, error) {
	query := `
		SELECT llm_user_id AS user_id,
		       arrayElement(topK(1)(service), 1) AS top_service,
		       uniqCombined64(trace_id) AS traces,
		       sum(gen_ai_input_tokens + gen_ai_output_tokens) AS tokens,
		       sum(` + pricing.TokenCostSQL("gen_ai_input_tokens", "gen_ai_output_tokens", "gen_ai_request_model") + `) AS cost,
		       max(timestamp) AS last_seen
		FROM optikk.spans
		PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end
		WHERE is_gen_ai AND llm_user_id != ''
		GROUP BY user_id
		ORDER BY cost DESC
		LIMIT @limit`
	args := append(chargs.RangeArgs(tenantID, startMs, endMs), pricing.Args()...)
	args = append(args, clickhouse.Named("limit", uint64(limit)))
	var rows []userRow
	return rows, dbutil.SelectCH(dbutil.ExplorerCtx(ctx), r.db, "llm.users.TopUsers", &rows, query, args...)
}

// Overview aggregates active users, total traces and total cost in one pass.
func (r *Repository) Overview(ctx context.Context, tenantID, startMs, endMs int64) (overviewRow, error) {
	query := `
		SELECT uniqCombined64(llm_user_id) AS active_users,
		       uniqCombined64(trace_id)     AS traces,
		       sum(` + pricing.TokenCostSQL("gen_ai_input_tokens", "gen_ai_output_tokens", "gen_ai_request_model") + `) AS cost
		FROM optikk.spans
		PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end
		WHERE is_gen_ai AND llm_user_id != ''`
	args := append(chargs.RangeArgs(tenantID, startMs, endMs), pricing.Args()...)
	var row overviewRow
	return row, dbutil.QueryRowCH(dbutil.OverviewCtx(ctx), r.db, "llm.users.Overview", &row, query, args...)
}

// MeanScoreByUser returns the mean numeric score per user for scored users.
func (r *Repository) MeanScoreByUser(ctx context.Context, tenantID, startMs, endMs int64) ([]userScoreRow, error) {
	query := `
		SELECT user_id, avg(value) AS mean
		FROM optikk.llm_scores
		PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end
		WHERE user_id != '' AND data_type = 'numeric'
		GROUP BY user_id`
	var rows []userScoreRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "llm.users.MeanScoreByUser", &rows, query,
		chargs.RangeArgs(tenantID, startMs, endMs)...)
}
