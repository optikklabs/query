package app

import (
	"log/slog"

	"github.com/ClickHouse/clickhouse-go/v2"

	"github.com/optikklabs/query/internal/app/registry"

	alerting_evaluator "github.com/optikklabs/query/internal/modules/alerting/evaluator"
	alerting_monitors "github.com/optikklabs/query/internal/modules/alerting/monitors"
	alerting_notifications "github.com/optikklabs/query/internal/modules/alerting/notifications"
	dashboards "github.com/optikklabs/query/internal/modules/dashboards"
	infrastructure_cpu "github.com/optikklabs/query/internal/modules/infrastructure/cpu"
	infrastructure_fleet "github.com/optikklabs/query/internal/modules/infrastructure/fleet"
	infrastructure_hosts "github.com/optikklabs/query/internal/modules/infrastructure/hosts"
	infrastructure_memory "github.com/optikklabs/query/internal/modules/infrastructure/memory"
	infrastructure_nodes "github.com/optikklabs/query/internal/modules/infrastructure/nodes"
	ingestion "github.com/optikklabs/query/internal/modules/ingestion"
	llm "github.com/optikklabs/query/internal/modules/llm"
	log_explorer "github.com/optikklabs/query/internal/modules/logs/explorer"
	log_facets "github.com/optikklabs/query/internal/modules/logs/facets"
	log_detail "github.com/optikklabs/query/internal/modules/logs/logdetail"
	log_trace_logs "github.com/optikklabs/query/internal/modules/logs/trace_logs"
	log_trends "github.com/optikklabs/query/internal/modules/logs/trends"
	metrics_explorer "github.com/optikklabs/query/internal/modules/metrics/explorer"
	onboarding "github.com/optikklabs/query/internal/modules/onboarding"
	onboarding_provisioner "github.com/optikklabs/query/internal/modules/onboarding/provisioner"
	saturation_explorer "github.com/optikklabs/query/internal/modules/saturation/database/explorer"
	saturation_database_latency "github.com/optikklabs/query/internal/modules/saturation/database/latency"
	saturation_database_querydetail "github.com/optikklabs/query/internal/modules/saturation/database/querydetail"
	saturation_database_slowqueries "github.com/optikklabs/query/internal/modules/saturation/database/slowqueries"
	saturation_database_volume "github.com/optikklabs/query/internal/modules/saturation/database/volume"
	saturation_kafka_explorer "github.com/optikklabs/query/internal/modules/saturation/kafka/explorer"
	saturation_kafka_topology "github.com/optikklabs/query/internal/modules/saturation/kafka/topology"
	services_errors "github.com/optikklabs/query/internal/modules/services/errors"
	services_redfleet "github.com/optikklabs/query/internal/modules/services/redfleet"
	services_topology "github.com/optikklabs/query/internal/modules/services/topology"
	traces_detail "github.com/optikklabs/query/internal/modules/traces/detail"
	traces_explorer "github.com/optikklabs/query/internal/modules/traces/explorer"
	traces_paths "github.com/optikklabs/query/internal/modules/traces/paths"
	traces_servicemap "github.com/optikklabs/query/internal/modules/traces/servicemap"
	"github.com/optikklabs/query/internal/modules/user"
)

func configuredModules(
	nativeQuerier clickhouse.Conn,
	appConfig registry.AppConfig,
	infraDeps *Infra,
) []registry.Module {
	userRepo := user.NewRepository(infraDeps.DB, appConfig)
	userService := user.NewService(userRepo, infraDeps.Tokens)

	// Seed the platform super-admin so the first tenant can be provisioned.
	if err := userService.EnsureSuperAdmin(appConfig.Admin.Email, appConfig.Admin.Password); err != nil {
		slog.Error("failed to seed super-admin", slog.Any("error", err))
	}

	return []registry.Module{
		infrastructure_cpu.NewModule(nativeQuerier),
		infrastructure_memory.NewModule(nativeQuerier),
		infrastructure_fleet.NewModule(nativeQuerier),
		infrastructure_hosts.NewModule(nativeQuerier),
		infrastructure_nodes.NewModule(nativeQuerier),
		log_explorer.NewModule(nativeQuerier),
		log_detail.NewModule(nativeQuerier),
		log_facets.NewModule(nativeQuerier),
		log_trends.NewModule(nativeQuerier),
		log_trace_logs.NewModule(nativeQuerier),
		metrics_explorer.NewModule(nativeQuerier),
		ingestion.NewModule(nativeQuerier),
		llm.NewModule(nativeQuerier),
		services_errors.NewModule(nativeQuerier),
		services_redfleet.NewModule(nativeQuerier),
		saturation_explorer.NewModule(nativeQuerier),
		saturation_database_latency.NewModule(nativeQuerier),
		saturation_database_slowqueries.NewModule(nativeQuerier),
		saturation_database_querydetail.NewModule(nativeQuerier),
		saturation_database_volume.NewModule(nativeQuerier),
		saturation_kafka_explorer.NewModule(nativeQuerier),
		saturation_kafka_topology.NewModule(nativeQuerier),
		services_topology.NewModule(nativeQuerier),
		traces_explorer.NewModule(nativeQuerier),
		traces_detail.NewModule(nativeQuerier),
		traces_paths.NewModule(nativeQuerier),
		traces_servicemap.NewModule(nativeQuerier),

		user.NewModule(userService, infraDeps.Tokens),

		onboarding.NewModule(infraDeps.DB, nativeQuerier),
		onboarding_provisioner.NewModule(infraDeps.DB),

		alerting_monitors.NewModule(infraDeps.DB, nativeQuerier),
		alerting_notifications.NewModule(infraDeps.DB),
		alerting_evaluator.NewModule(infraDeps.DB, nativeQuerier),

		dashboards.NewModule(infraDeps.DB),
	}
}
