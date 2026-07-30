package repository

import (
	"context"

	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/shared/chargs"
	"github.com/optikklabs/query/internal/shared/spanstats"
)

func clientsQuery(startMs, endMs int64) string {
	return `
		SELECT DISTINCT service
		FROM ` + timebucket.SpanStatsRollup(startMs, endMs) + `
		PREWHERE tenant_id = @tenantID AND timestamp >= @start AND timestamp < @end AND service != ''
		WHERE messaging_system = 'kafka' AND messaging_destination != ''
		ORDER BY service
		LIMIT 200`
}

type EdgeRow struct {
	Service       string    `ch:"service"`
	Topic         string    `ch:"topic"`
	ConsumerGroup string    `ch:"consumer_group"`
	CallCount     uint64    `ch:"call_count"`
	ErrorCount    uint64    `ch:"error_count"`
	QS            []float64 `ch:"qs"`
}

func (r *Repository) QueryClients(ctx context.Context, tenantID, startMs, endMs int64) ([]string, error) {
	query := clientsQuery(startMs, endMs)
	var rows []struct {
		Service string `ch:"service"`
	}
	if err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "kafka.QueryClients", &rows, query, chargs.RangeArgs(tenantID, startMs, endMs)...); err != nil {
		return nil, err
	}
	clients := make([]string, len(rows))
	for i, row := range rows {
		clients[i] = row.Service
	}
	return clients, nil
}

// Topic scoping is applied in Go: keep every edge on a topic that services touch.
func (r *Repository) QueryEdges(ctx context.Context, tenantID, startMs, endMs int64, services []string) ([]EdgeRow, error) {
	query := edgesQuery(timebucket.SpanStatsRollup(startMs, endMs))
	var rows []EdgeRow
	if err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "kafka.QueryEdges", &rows, query,
		chargs.RangeArgs(tenantID, startMs, endMs)...); err != nil {
		return nil, err
	}
	return scopeEdgesToTopics(rows, services), nil
}

func scopeEdgesToTopics(rows []EdgeRow, services []string) []EdgeRow {
	inScope := make(map[string]struct{}, len(services))
	for _, s := range services {
		inScope[s] = struct{}{}
	}
	topics := make(map[string]struct{})
	for _, row := range rows {
		if _, ok := inScope[row.Service]; ok {
			topics[row.Topic] = struct{}{}
		}
	}
	out := make([]EdgeRow, 0, len(rows))
	for _, row := range rows {
		if _, ok := topics[row.Topic]; ok {
			out = append(out, row)
		}
	}
	return out
}

func edgesQuery(rollupTable string) string {
	return `
		SELECT service                                           AS service,
		       messaging_destination                             AS topic,
		       messaging_consumer_group                          AS consumer_group,
		       sum(request_count)                                AS call_count,
		       sumIf(request_count, ` + spanstats.ErrorPred + `) AS error_count,
		       quantilesTDigestMerge(0.5, 0.95, 0.99)(latency_state) AS qs
		FROM ` + rollupTable + `
		PREWHERE tenant_id = @tenantID
		     AND timestamp >= @start AND timestamp < @end
		     AND service != ''
		WHERE messaging_system = 'kafka'
		  AND messaging_destination != ''
		GROUP BY service, topic, consumer_group
		ORDER BY call_count DESC, service ASC, topic ASC, consumer_group ASC`
}
