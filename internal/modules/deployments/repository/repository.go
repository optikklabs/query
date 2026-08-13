package repository

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/modules/deployments/models"
	"github.com/optikklabs/query/internal/shared/chargs"
	"github.com/optikklabs/query/internal/shared/errorgroups"
	"github.com/optikklabs/query/internal/shared/spanstats"
)

type Repository struct {
	db clickhouse.Conn
}

func NewRepository(db clickhouse.Conn) *Repository {
	return &Repository{db: db}
}

func (r *Repository) ListRows(ctx context.Context, req models.ListRequest) ([]models.RawDeploymentRow, error) {
	query := `
		SELECT service,
		       environment,
		       service_version,
		       min(timestamp) AS first_seen,
		       ` + spanstats.Requests + `,
		       ` + spanstats.Errors + `,
		       ` + spanstats.LatencyP95.SQL() + `
		FROM ` + timebucket.SpanStatsRollup(req.StartMs, req.EndMs) + `
		PREWHERE tenant_id = @tenantID
		     AND timestamp >= @start AND timestamp < @end
		     AND service_version != ''
		WHERE ` + spanstats.InboundPred + `
		GROUP BY service, environment, service_version
		ORDER BY service ASC, environment ASC, first_seen ASC, service_version ASC`

	var rows []models.RawDeploymentRow
	err := dbutil.SelectCH(
		dbutil.OverviewCtx(ctx),
		r.db,
		"deployments.List",
		&rows,
		query,
		chargs.RangeArgs(req.TenantID, req.StartMs, req.EndMs)...,
	)
	return rows, err
}

func (r *Repository) ComparisonRow(
	ctx context.Context,
	req models.DetailRequest,
	comparison models.Context,
) (models.RawComparisonRow, error) {
	current, baseline := comparisonConditions()
	query := `
		SELECT sumIf(request_count, ` + current + `) AS current_requests,
		       sumIf(request_count, ` + current + ` AND ` + spanstats.ErrorPred + `) AS current_errors,
		       quantilesTDigestMergeIf(0.5, 0.75, 0.9, 0.95, 0.99)(latency_state, ` + current + `) AS current_qs,
		       sumIf(request_count, ` + baseline + `) AS baseline_requests,
		       sumIf(request_count, ` + baseline + ` AND ` + spanstats.ErrorPred + `) AS baseline_errors,
		       quantilesTDigestMergeIf(0.5, 0.75, 0.9, 0.95, 0.99)(latency_state, ` + baseline + `) AS baseline_qs
		FROM ` + comparisonRollup(comparison) + `
		PREWHERE tenant_id = @tenantID
		     AND timestamp >= @baselineStart AND timestamp < @currentEnd
		     AND service = @service AND environment = @environment
		WHERE ` + spanstats.InboundPred + `
		     AND (` + current + ` OR ` + baseline + `)`

	var row models.RawComparisonRow
	err := dbutil.QueryRowCH(
		dbutil.OverviewCtx(ctx),
		r.db,
		"deployments.Compare",
		&row,
		query,
		comparisonArgs(req, comparison)...,
	)
	return row, err
}

func (r *Repository) TrafficRows(
	ctx context.Context,
	req models.DetailRequest,
	comparison models.Context,
) ([]models.RawTrafficRow, error) {
	query := `
		SELECT ` + timebucket.DisplayGrainSQL(
		comparison.Window.CurrentEnd.UnixMilli()-comparison.Window.BaselineStart.UnixMilli(),
	) + ` AS bucket_at,
		       service_version,
		       ` + spanstats.Requests + `
		FROM ` + comparisonRollup(comparison) + `
		PREWHERE tenant_id = @tenantID
		     AND timestamp >= @baselineStart AND timestamp < @currentEnd
		     AND service = @service AND environment = @environment
		     AND service_version != ''
		WHERE ` + spanstats.InboundPred + `
		GROUP BY bucket_at, service_version
		ORDER BY bucket_at ASC, service_version ASC`

	var rows []models.RawTrafficRow
	err := dbutil.SelectCH(
		dbutil.OverviewCtx(ctx),
		r.db,
		"deployments.Traffic",
		&rows,
		query,
		trafficArgs(req, comparison)...,
	)
	return rows, err
}

func (r *Repository) ErrorChangeRows(
	ctx context.Context,
	req models.DetailRequest,
	comparison models.Context,
) ([]models.RawErrorChangeRow, error) {
	current, baseline := comparisonConditions()
	query := `
		SELECT ` + errorgroups.IdentityProjection("") + `,
		       countIf(` + current + `) AS current_count,
		       countIf(` + baseline + `) AS baseline_count
		FROM optikk.spans
		PREWHERE tenant_id = @tenantID
		     AND timestamp >= @baselineStart AND timestamp < @currentEnd
		     AND service = @service AND environment = @environment
		     AND ` + errorgroups.Predicate + `
		WHERE error_group_id != ''
		     AND (` + current + ` OR ` + baseline + `)
		GROUP BY error_group_id
		HAVING current_count = 0 OR baseline_count = 0
		ORDER BY greatest(current_count, baseline_count) DESC, error_group_id ASC
		LIMIT @limit`

	args := append(
		comparisonArgs(req, comparison),
		clickhouse.Named("limit", req.Limit),
	)
	var rows []models.RawErrorChangeRow
	err := dbutil.SelectCH(
		dbutil.OverviewCtx(ctx),
		r.db,
		"deployments.Errors",
		&rows,
		query,
		args...,
	)
	return rows, err
}

func (r *Repository) DimensionDiffRows(
	ctx context.Context,
	req models.DetailRequest,
	comparison models.Context,
	dimension string,
) ([]models.RawDimensionDiffRow, error) {
	column, predicate := dimensionSQL(dimension)
	current, baseline := comparisonConditions()
	query := `
		SELECT ` + column + ` AS name,
		       sumIf(request_count, ` + current + `) AS current_requests,
		       sumIf(request_count, ` + current + ` AND ` + spanstats.ErrorPred + `) AS current_errors,
		       quantilesTDigestMergeIf(0.95)(latency_state, ` + current + `) AS current_qs,
		       sumIf(request_count, ` + baseline + `) AS baseline_requests,
		       sumIf(request_count, ` + baseline + ` AND ` + spanstats.ErrorPred + `) AS baseline_errors,
		       quantilesTDigestMergeIf(0.95)(latency_state, ` + baseline + `) AS baseline_qs
		FROM ` + comparisonRollup(comparison) + `
		PREWHERE tenant_id = @tenantID
		     AND timestamp >= @baselineStart AND timestamp < @currentEnd
		     AND service = @service AND environment = @environment
		WHERE ` + predicate + `
		     AND (` + current + ` OR ` + baseline + `)
		GROUP BY name
		HAVING current_requests > 0 OR baseline_requests > 0
		ORDER BY current_requests + baseline_requests DESC, name ASC
		LIMIT @limit`

	args := append(
		comparisonArgs(req, comparison),
		clickhouse.Named("limit", req.Limit),
	)
	var rows []models.RawDimensionDiffRow
	err := dbutil.SelectCH(
		dbutil.OverviewCtx(ctx),
		r.db,
		"deployments."+dimension,
		&rows,
		query,
		args...,
	)
	return rows, err
}

func comparisonConditions() (current, baseline string) {
	current = `(service_version = @currentVersion
		          AND timestamp >= @currentStart AND timestamp < @currentEnd)`
	baseline = `(@hasBaseline = 1 AND service_version = @baselineVersion
		           AND timestamp >= @baselineStart AND timestamp < @baselineEnd)`
	return current, baseline
}

func comparisonRollup(comparison models.Context) string {
	return timebucket.SpanStatsRollup(
		comparison.Window.BaselineStart.UnixMilli(),
		comparison.Window.CurrentEnd.UnixMilli(),
	)
}

func comparisonArgs(req models.DetailRequest, comparison models.Context) []any {
	baselineVersion := ""
	hasBaseline := uint8(0)
	if comparison.BaselineVersion != nil {
		baselineVersion = *comparison.BaselineVersion
		hasBaseline = 1
	}
	return []any{
		clickhouse.Named("tenantID", uint32(req.TenantID)),
		clickhouse.Named("service", req.Service),
		clickhouse.Named("environment", req.Environment),
		clickhouse.Named("currentVersion", req.Version),
		clickhouse.Named("baselineVersion", baselineVersion),
		clickhouse.Named("hasBaseline", hasBaseline),
		clickhouse.Named("currentStart", comparison.Window.CurrentStart),
		clickhouse.Named("currentEnd", comparison.Window.CurrentEnd),
		clickhouse.Named("baselineStart", comparison.Window.BaselineStart),
		clickhouse.Named("baselineEnd", comparison.Window.BaselineEnd),
	}
}

func trafficArgs(req models.DetailRequest, comparison models.Context) []any {
	return []any{
		clickhouse.Named("tenantID", uint32(req.TenantID)),
		clickhouse.Named("service", req.Service),
		clickhouse.Named("environment", req.Environment),
		clickhouse.Named("currentEnd", comparison.Window.CurrentEnd),
		clickhouse.Named("baselineStart", comparison.Window.BaselineStart),
	}
}

func dimensionSQL(dimension string) (column, predicate string) {
	switch dimension {
	case "endpoints":
		return "http_route", spanstats.InboundPred + " AND http_route != ''"
	case "dependencies":
		return "peer_name", "kind_string IN ('CLIENT', 'PRODUCER') AND peer_name != ''"
	default:
		panic("deployments: unsupported dimension " + dimension)
	}
}
