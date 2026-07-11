package app

import (
	"github.com/ClickHouse/clickhouse-go/v2"

	"github.com/optikklabs/query/internal/app/registry"

	alerting_evaluator "github.com/optikklabs/query/internal/modules/alerting/evaluator"
	alerting_monitors "github.com/optikklabs/query/internal/modules/alerting/monitors"
	alerting_notifications "github.com/optikklabs/query/internal/modules/alerting/notifications"
	billing "github.com/optikklabs/query/internal/modules/billing"
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
	user_auth "github.com/optikklabs/query/internal/modules/user/auth"
	user_device "github.com/optikklabs/query/internal/modules/user/device"
	user_signup "github.com/optikklabs/query/internal/modules/user/signup"
	user_tenant "github.com/optikklabs/query/internal/modules/user/tenant"
	user_users "github.com/optikklabs/query/internal/modules/user/users"
)

func configuredModules(
	nativeQuerier clickhouse.Conn,
	appConfig registry.AppConfig,
	infraDeps *Infra,
) []registry.Module {
	// Auth owns token issuance, signup, and session lifecycle.
	// Device flow reuses auth's IssueTokens for CLI login.
	authService := user_auth.NewService(user_auth.NewRepository(infraDeps.DB), infraDeps.Tokens)
	deviceService := user_device.NewService(user_device.NewRepository(infraDeps.DB), authService)
	signupService := user_signup.NewService(user_signup.NewRepository(infraDeps.DB), authService, infraDeps.Config.Bot.TurnstileSecret, infraDeps.Config.Email.ResendAPIKey, infraDeps.Config.Email.From, infraDeps.Config.Email.VerifyBaseURL)
	tenantService := user_tenant.NewService(user_tenant.NewRepository(infraDeps.DB))
	usersService := user_users.NewService(user_users.NewRepository(infraDeps.DB))

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

		user_auth.NewModule(authService, infraDeps.Tokens),
		user_device.NewModule(deviceService, infraDeps.Tokens),
		user_signup.NewModule(signupService, infraDeps.Tokens),
		user_tenant.NewModule(tenantService),
		user_users.NewModule(usersService),

		alerting_monitors.NewModule(infraDeps.DB, nativeQuerier),
		alerting_notifications.NewModule(infraDeps.DB),
		alerting_evaluator.NewModule(infraDeps.DB, nativeQuerier),

		billing.NewModule(infraDeps.DB),

		dashboards.NewModule(infraDeps.DB),
	}
}
