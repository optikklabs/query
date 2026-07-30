package sessions

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/modules/llm/pricing"
	"github.com/optikklabs/query/internal/shared/chargs"
)

const durationMsSQL = "dateDiff('millisecond', min(timestamp), max(timestamp + toIntervalNanosecond(duration_nano)))"

type Repository struct {
	db clickhouse.Conn
}

func NewRepository(db clickhouse.Conn) *Repository {
	return &Repository{db: db}
}

func (r *Repository) TopSessions(ctx context.Context, tenantID, startMs, endMs int64, limit int) ([]sessionRow, error) {
	query := `
		SELECT llm_session_id AS session_id,
		       arrayElement(topK(1)(service), 1) AS service,
		       argMaxIf(llm_user_id, (timestamp, span_id), llm_user_id != '') AS user_id,
		       argMinIf(substring(gen_ai_prompt, 1, 140), (timestamp, span_id), gen_ai_prompt != '') AS preview,
		       uniqExact(trace_id) AS turns,
		       ` + durationMsSQL + ` AS duration_ms,
		       sum(` + pricing.TokenCostSQL("gen_ai_input_tokens", "gen_ai_output_tokens", "gen_ai_request_model") + `) AS cost,
		       max(timestamp) AS last_ts
		FROM optikk.spans
		PREWHERE tenant_id = @tenantID AND timestamp >= @start AND timestamp < @end
		WHERE is_gen_ai AND llm_session_id != ''
		GROUP BY session_id
		ORDER BY last_ts DESC, session_id ASC
		LIMIT @limit`
	args := append(chargs.RangeArgs(tenantID, startMs, endMs), pricing.Args()...)
	args = append(args, clickhouse.Named("limit", uint64(limit)))
	var rows []sessionRow
	return rows, dbutil.SelectCH(dbutil.ExplorerCtx(ctx), r.db, "llm.sessions.TopSessions", &rows, query, args...)
}

func (r *Repository) Overview(ctx context.Context, tenantID, startMs, endMs int64) (overviewRow, error) {

	query := `
		SELECT count()      AS sessions,
		       sum(turns)    AS turns,
		       avg(dur)      AS duration_ms,
		       sum(cost)     AS cost
		FROM (
		    SELECT uniqExact(trace_id) AS turns,
		           ` + durationMsSQL + ` AS dur,
		           sum(` + pricing.TokenCostSQL("gen_ai_input_tokens", "gen_ai_output_tokens", "gen_ai_request_model") + `) AS cost
		    FROM optikk.spans
		    PREWHERE tenant_id = @tenantID AND timestamp >= @start AND timestamp < @end
		    WHERE is_gen_ai AND llm_session_id != ''
		    GROUP BY llm_session_id
		)`
	args := append(chargs.RangeArgs(tenantID, startMs, endMs), pricing.Args()...)
	var row overviewRow
	return row, dbutil.QueryRowCH(dbutil.OverviewCtx(ctx), r.db, "llm.sessions.Overview", &row, query, args...)
}

func (r *Repository) MeanScoreBySession(ctx context.Context, tenantID, startMs, endMs int64, sessionIDs []string) ([]sessionScoreRow, error) {
	query := `
		SELECT session_id, avg(value) AS mean
		FROM optikk.llm_scores
		PREWHERE tenant_id = @tenantID AND timestamp >= @start AND timestamp < @end
		WHERE session_id IN @sessionIDs AND data_type = 'numeric'
		GROUP BY session_id`
	args := append(chargs.RangeArgs(tenantID, startMs, endMs),
		clickhouse.Named("sessionIDs", sessionIDs))
	var rows []sessionScoreRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "llm.sessions.MeanScoreBySession", &rows, query, args...)
}

func (r *Repository) Detail(ctx context.Context, tenantID int64, sessionID string, startMs, endMs int64) ([]turnRow, error) {
	query := `
		SELECT trace_id,
		       min(timestamp) AS start_ts,
		       ` + durationMsSQL + ` AS duration_ms,
		       argMaxIf(gen_ai_request_model, gen_ai_input_tokens + gen_ai_output_tokens, gen_ai_request_model != '') AS model,
		       argMinIf(gen_ai_prompt, (timestamp, span_id), gen_ai_prompt != '') AS user_text,
		       argMaxIf(gen_ai_completion, (timestamp, span_id), gen_ai_completion != '') AS output_text,
		       sum(` + pricing.TokenCostSQL("gen_ai_input_tokens", "gen_ai_output_tokens", "gen_ai_request_model") + `) AS cost
		FROM optikk.spans
		PREWHERE tenant_id = @tenantID AND timestamp >= @start AND timestamp < @end
		WHERE is_gen_ai AND llm_session_id = @sessionID
		GROUP BY trace_id
		ORDER BY start_ts ASC, trace_id ASC`
	args := append(chargs.RangeArgs(tenantID, startMs, endMs), pricing.Args()...)
	args = append(args, clickhouse.Named("sessionID", sessionID))
	var rows []turnRow
	return rows, dbutil.SelectCH(dbutil.ExplorerCtx(ctx), r.db, "llm.sessions.Detail", &rows, query, args...)
}
