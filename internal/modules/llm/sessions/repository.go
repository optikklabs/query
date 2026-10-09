package sessions

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2"

	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/modules/llm/pricing"
	"github.com/optikklabs/query/internal/modules/llm/scores"
	"github.com/optikklabs/query/internal/shared/chargs"
)

const (
	durationMsSQL = "dateDiff('millisecond', min(timestamp), max(timestamp + toIntervalNanosecond(duration_nano)))"
	// Shared by the list and detail queries so a session shows the same
	// service and user in both places.
	serviceSQL = "arrayElement(topK(1)(service), 1)"
	userIDSQL  = "argMaxIf(llm_user_id, (timestamp, span_id), llm_user_id != '')"
)

type Repository struct {
	db clickhouse.Conn
}

func NewRepository(db clickhouse.Conn) *Repository {
	return &Repository{db: db}
}

func (r *Repository) TopSessions(ctx context.Context, tenantID, startMs, endMs int64, limit int) ([]sessionRow, error) {
	query := `
		SELECT llm_session_id AS session_id,
		       ` + serviceSQL + ` AS service,
		       ` + userIDSQL + ` AS user_id,
		       argMinIf(prompt_preview, (timestamp, span_id), prompt_preview != '') AS preview,
		       uniqExact(trace_id) AS turns,
		       ` + durationMsSQL + ` AS duration_ms,
		       sum(` + pricing.SpanCostSQL + `) AS cost,
		       max(timestamp) AS last_ts
		FROM optikk.llm_spans
		PREWHERE tenant_id = @tenantID AND timestamp >= @start AND timestamp < @end
		WHERE llm_session_id != ''
		GROUP BY session_id
		ORDER BY last_ts DESC, session_id ASC
		LIMIT @limit`
	args := append(chargs.RangeArgs(tenantID, startMs, endMs), pricing.Args()...)
	args = append(args, clickhouse.Named("limit", uint64(limit)))
	var rows []sessionRow
	err := dbutil.SelectCH(dbutil.ExplorerCtx(ctx), r.db, "llm.sessions.TopSessions", &rows, query, args...)
	return rows, err
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
		           sum(` + pricing.SpanCostSQL + `) AS cost
		    FROM optikk.llm_spans
		    PREWHERE tenant_id = @tenantID AND timestamp >= @start AND timestamp < @end
		    WHERE llm_session_id != ''
		    GROUP BY llm_session_id
		)`
	args := append(chargs.RangeArgs(tenantID, startMs, endMs), pricing.Args()...)
	var row overviewRow
	err := dbutil.QueryRowCH(dbutil.OverviewCtx(ctx), r.db, "llm.sessions.Overview", &row, query, args...)
	return row, err
}

func (r *Repository) MeanScoreBySession(ctx context.Context, tenantID, startMs, endMs int64, sessionIDs []string) (map[string]*float64, error) {
	return scores.MeanScores(ctx, r.db, "llm.sessions.MeanScoreBySession", scores.BySession, tenantID, startMs, endMs, sessionIDs)
}

func (r *Repository) Detail(ctx context.Context, tenantID int64, sessionID string, startMs, endMs int64) ([]turnRow, error) {
	query := `
		SELECT trace_id,
		       min(timestamp) AS start_ts,
		       ` + durationMsSQL + ` AS duration_ms,
		       argMaxIf(gen_ai_request_model, gen_ai_input_tokens + gen_ai_output_tokens, gen_ai_request_model != '') AS model,
		       argMinIf(gen_ai_prompt, (timestamp, span_id), gen_ai_prompt != '') AS user_text,
		       argMaxIf(gen_ai_completion, (timestamp, span_id), gen_ai_completion != '') AS output_text,
		       sum(` + pricing.SpanCostSQL + `) AS cost
		FROM optikk.spans
		PREWHERE tenant_id = @tenantID AND timestamp >= @start AND timestamp < @end
		WHERE is_gen_ai AND llm_session_id = @sessionID
		GROUP BY trace_id
		ORDER BY start_ts ASC, trace_id ASC`
	args := append(chargs.RangeArgs(tenantID, startMs, endMs), pricing.Args()...)
	args = append(args, clickhouse.Named("sessionID", sessionID))
	var rows []turnRow
	err := dbutil.SelectCH(dbutil.ExplorerCtx(ctx), r.db, "llm.sessions.Detail", &rows, query, args...)
	return rows, err
}

func (r *Repository) Identity(ctx context.Context, tenantID int64, sessionID string, startMs, endMs int64) (identityRow, error) {
	query := `
		SELECT ` + serviceSQL + ` AS service,
		       ` + userIDSQL + ` AS user_id
		FROM optikk.llm_spans
		PREWHERE tenant_id = @tenantID AND timestamp >= @start AND timestamp < @end
		WHERE llm_session_id = @sessionID`
	args := append(chargs.RangeArgs(tenantID, startMs, endMs), clickhouse.Named("sessionID", sessionID))
	var row identityRow
	err := dbutil.QueryRowCH(dbutil.ExplorerCtx(ctx), r.db, "llm.sessions.Identity", &row, query, args...)
	return row, err
}
