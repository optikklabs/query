package cloud

const (
	CategoryCompute   = "compute"
	CategoryData      = "data"
	CategoryStorage   = "storage"
	CategoryNetwork   = "network"
	CategoryStreaming = "streaming"
	CategoryAI        = "ai"
	CategoryOther     = "other"
)

var platformCategory = map[string]string{

	"aws_ec2":               CategoryCompute,
	"aws_eks":               CategoryCompute,
	"aws_ecs":               CategoryCompute,
	"aws_lambda":            CategoryCompute,
	"aws_elastic_beanstalk": CategoryCompute,
	"aws_app_runner":        CategoryCompute,
	"aws_rds":               CategoryData,
	"aws_dynamodb":          CategoryData,
	"aws_elasticache":       CategoryData,
	"aws_redshift":          CategoryData,
	"aws_s3":                CategoryStorage,
	"aws_ebs":               CategoryStorage,
	"aws_efs":               CategoryStorage,
	"aws_elb":               CategoryNetwork,
	"aws_cloudfront":        CategoryNetwork,
	"aws_route53":           CategoryNetwork,
	"aws_msk":               CategoryStreaming,
	"aws_sqs":               CategoryStreaming,
	"aws_sns":               CategoryStreaming,
	"aws_kinesis":           CategoryStreaming,
	"aws_bedrock":           CategoryAI,

	"gcp_compute_engine":    CategoryCompute,
	"gcp_kubernetes_engine": CategoryCompute,
	"gcp_cloud_run":         CategoryCompute,
	"gcp_cloud_functions":   CategoryCompute,
	"gcp_app_engine":        CategoryCompute,
	"gcp_cloud_sql":         CategoryData,
	"gcp_spanner":           CategoryData,
	"gcp_bigtable":          CategoryData,
	"gcp_bigquery":          CategoryData,
	"gcp_memorystore":       CategoryData,
	"gcp_cloud_storage":     CategoryStorage,
	"gcp_load_balancing":    CategoryNetwork,
	"gcp_pubsub":            CategoryStreaming,
	"gcp_dataflow":          CategoryStreaming,
	"gcp_vertex_ai":         CategoryAI,

	"azure_vm":             CategoryCompute,
	"azure_aks":            CategoryCompute,
	"azure_functions":      CategoryCompute,
	"azure_app_service":    CategoryCompute,
	"azure_container_apps": CategoryCompute,
	"azure_sql":            CategoryData,
	"azure_cosmosdb":       CategoryData,
	"azure_redis_cache":    CategoryData,
	"azure_blob_storage":   CategoryStorage,
	"azure_files":          CategoryStorage,
	"azure_app_gateway":    CategoryNetwork,
	"azure_front_door":     CategoryNetwork,
	"azure_event_hubs":     CategoryStreaming,
	"azure_service_bus":    CategoryStreaming,
	"azure_openai":         CategoryAI,
}

func CategoryFor(platform string) string {
	if c, ok := platformCategory[platform]; ok {
		return c
	}
	return CategoryOther
}

const (
	unhealthyErrorRate = 10.0
	degradedErrorRate  = 2.0
)

func classifyHealth(errorRate float64) string {
	switch {
	case errorRate > unhealthyErrorRate:
		return "unhealthy"
	case errorRate > degradedErrorRate:
		return "degraded"
	default:
		return "healthy"
	}
}
