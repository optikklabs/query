package app

import (
	"log/slog"

	"github.com/optikklabs/query/internal/infra/llmproviders"
	"github.com/optikklabs/query/internal/infra/secretbox"

	alerting_evaluator "github.com/optikklabs/query/internal/modules/alerting/evaluator"
	alerting_monitors "github.com/optikklabs/query/internal/modules/alerting/monitors"
	alerting_notifications "github.com/optikklabs/query/internal/modules/alerting/notifications"
	billing "github.com/optikklabs/query/internal/modules/billing"
	dashboards "github.com/optikklabs/query/internal/modules/dashboards"
	"github.com/optikklabs/query/internal/modules/deployments"
	"github.com/optikklabs/query/internal/modules/infrastructure"
	ingestion "github.com/optikklabs/query/internal/modules/ingestion"
	llm "github.com/optikklabs/query/internal/modules/llm"
	llm_datasets "github.com/optikklabs/query/internal/modules/llm/datasets"
	llm_evaluators "github.com/optikklabs/query/internal/modules/llm/evaluators"
	llm_playground "github.com/optikklabs/query/internal/modules/llm/playground"
	llm_prompts "github.com/optikklabs/query/internal/modules/llm/prompts"
	llm_providerkeys "github.com/optikklabs/query/internal/modules/llm/providerkeys"
	llm_scores "github.com/optikklabs/query/internal/modules/llm/scores"
	llm_sessions "github.com/optikklabs/query/internal/modules/llm/sessions"
	llm_users "github.com/optikklabs/query/internal/modules/llm/users"
	"github.com/optikklabs/query/internal/modules/logs"
	metrics_explorer "github.com/optikklabs/query/internal/modules/metrics/explorer"
	saturation_database "github.com/optikklabs/query/internal/modules/saturation/database"
	saturation_kafka "github.com/optikklabs/query/internal/modules/saturation/kafka"
	services_errors "github.com/optikklabs/query/internal/modules/services/errors"
	services_redfleet "github.com/optikklabs/query/internal/modules/services/redfleet"
	services_topology "github.com/optikklabs/query/internal/modules/services/topology"
	"github.com/optikklabs/query/internal/modules/traces"
	traces_explorer "github.com/optikklabs/query/internal/modules/traces/explorer"
	user_auth "github.com/optikklabs/query/internal/modules/user/auth"
	user_device "github.com/optikklabs/query/internal/modules/user/device"
	user_signup "github.com/optikklabs/query/internal/modules/user/signup"
	user_tenant "github.com/optikklabs/query/internal/modules/user/tenant"
	user_users "github.com/optikklabs/query/internal/modules/user/users"
)

func configuredModules(infraDeps *Infra) []Module {
	ch := infraDeps.CH
	authService := user_auth.NewService(user_auth.NewRepository(infraDeps.DB), infraDeps.Tokens, infraDeps.Config.Email)
	deviceService := user_device.NewService(user_device.NewRepository(infraDeps.DB), authService)
	signupService := user_signup.NewService(user_signup.NewRepository(infraDeps.DB), authService, infraDeps.Config.Email)
	tenantService := user_tenant.NewService(user_tenant.NewRepository(infraDeps.DB), infraDeps.Config.Ingestion)
	usersService := user_users.NewService(user_users.NewRepository(infraDeps.DB), authService)

	box, err := secretbox.New(infraDeps.Config.LLM.KeyEncryptionKey)
	if err != nil {
		slog.Warn("llm: provider-key encryption disabled", slog.Any("reason", err))
	}
	providerKeySvc := llm_providerkeys.NewService(llm_providerkeys.NewRepository(infraDeps.DB), box)
	llmProviders := llmproviders.NewRegistry()

	return []Module{
		deployments.NewModule(ch),
		logs.NewModule(ch),
		infrastructure.NewModule(ch),
		metrics_explorer.NewModule(ch),
		ingestion.NewModule(ch, infraDeps.Config.Billing),
		llm.NewModule(ch),
		llm_scores.NewModule(ch),
		llm_sessions.NewModule(ch),
		llm_users.NewModule(ch),
		llm_prompts.NewModule(infraDeps.DB),
		llm_datasets.NewModule(infraDeps.DB, providerKeySvc, llmProviders),
		llm_evaluators.NewModule(infraDeps.DB, ch),
		llm_providerkeys.NewModule(providerKeySvc),
		llm_playground.NewModule(providerKeySvc, llmProviders),
		services_errors.NewModule(ch),
		services_redfleet.NewModule(ch),
		saturation_database.NewModule(ch),
		saturation_kafka.NewModule(ch),
		services_topology.NewModule(ch),
		traces.NewModule(ch),
		traces_explorer.NewModule(ch),

		user_auth.NewModule(authService, infraDeps.Tokens),
		user_device.NewModule(deviceService, infraDeps.Tokens),
		user_signup.NewModule(signupService, infraDeps.Tokens),
		user_tenant.NewModule(tenantService),
		user_users.NewModule(usersService),

		alerting_monitors.NewModule(infraDeps.DB, ch),
		alerting_notifications.NewModule(infraDeps.DB),
		alerting_evaluator.NewModule(infraDeps.DB, ch),

		billing.NewModule(infraDeps.DB),

		dashboards.NewModule(infraDeps.DB),
	}
}
