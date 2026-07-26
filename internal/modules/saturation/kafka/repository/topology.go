package repository

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/shared/chargs"
	"github.com/optikklabs/query/internal/shared/spanstats"
)

const clientsQuery = `
	SELECT DISTINCT service
	FROM optikk.span_stats_1h
	PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end AND service != ''
	WHERE messaging_system = 'kafka' AND messaging_destination != ''
	ORDER BY service
	LIMIT 200`

// EdgeRow is one (service, topic, consumer group) aggregation. An empty
// ConsumerGroup marks a produce row; anything else is a consume row.
type EdgeRow struct {
	Service       string    `ch:"service"`
	Topic         string    `ch:"topic"`
	ConsumerGroup string    `ch:"consumer_group"`
	CallCount     uint64    `ch:"call_count"`
	ErrorCount    uint64    `ch:"error_count"`
	QS            []float64 `ch:"qs"`
}

// QueryClients lists Kafka services in deterministic name order. The frontend
// uses the first result as its initial selection.
func (r *Repository) QueryClients(ctx context.Context, tenantID, startMs, endMs int64) ([]string, error) {
	var rows []struct {
		Service string `ch:"service"`
	}
	if err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "kafka.QueryClients", &rows, clientsQuery, chargs.RangeArgs(tenantID, startMs, endMs)...); err != nil {
		return nil, err
	}
	clients := make([]string, len(rows))
	for i, row := range rows {
		clients[i] = row.Service
	}
	return clients, nil
}

// QueryEdges returns one row per (service, topic, consumer group) for the
// topics the given services touch. Produce rows carry an empty group, which is
// what separates the two sides of the graph -- so one scan of the rollup
// answers both, rather than a near-identical query per side.
func (r *Repository) QueryEdges(ctx context.Context, tenantID, startMs, endMs int64, services []string) ([]EdgeRow, error) {
	query := edgesQuery(timebucket.SpanStatsRollup(endMs - startMs))
	args := append(chargs.RollupRangeArgs(tenantID, startMs, endMs), clickhouse.Named("services", services))
	var rows []EdgeRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "kafka.QueryEdges", &rows, query, args...)
}

func edgesQuery(rollupTable string) string {
	return `
		WITH scoped_topics AS (
		    SELECT DISTINCT messaging_destination AS topic
		    FROM ` + rollupTable + `
		    PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end AND service IN @services
		    WHERE messaging_system = 'kafka' AND messaging_destination != ''
		)
		SELECT service                                           AS service,
		       messaging_destination                             AS topic,
		       messaging_consumer_group                          AS consumer_group,
		       sum(request_count)                                AS call_count,
		       sumIf(request_count, ` + spanstats.ErrorPred + `) AS error_count,
		       quantilesTDigestMerge(0.5, 0.95, 0.99)(latency_state) AS qs
		FROM ` + rollupTable + `
		PREWHERE tenant_id = @tenantID
		     AND timestamp BETWEEN @start AND @end
		     AND service != ''
		WHERE messaging_system = 'kafka'
		  AND messaging_destination IN (SELECT topic FROM scoped_topics)
		GROUP BY service, topic, consumer_group
		LIMIT 1000`
}
