package query

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	models "github.com/optikklabs/query/internal/modules/alerting/shared/models"
)

type ScalarResult struct {
	Value   float64
	HasData bool
}

type Point struct {
	BucketMs int64   `json:"bucketMs"`
	Value    float64 `json:"value"`
}

type Backend interface {
	Scalar(ctx context.Context, m models.MonitorRow, q models.MonitorQuery, scope models.Scope, cond models.Conditions, now time.Time) (ScalarResult, error)
	Series(ctx context.Context, m models.MonitorRow, q models.MonitorQuery, scope models.Scope, cond models.Conditions, windowMs int64, now time.Time) ([]Point, error)
}

type Registry struct {
	Metric Backend
	APM    Backend
	Log    Backend
}

func (r Registry) For(t string) (Backend, error) {
	switch t {
	case "metric":
		return r.Metric, nil
	case "apm":
		return r.APM, nil
	case "log":
		return r.Log, nil
	default:
		return nil, errors.New("unknown monitor type: " + t)
	}
}

func tenantIDArg(tenantID int64) any {
	return clickhouse.Named("tenantID", uint32(tenantID))
}

var scopeAliases = map[string]string{
	"service": "service", "service.name": "service", "host": "host", "host.name": "host",
	"environment": "environment", "deployment.environment": "environment", "pod": "pod", "k8s.pod.name": "pod",
	"container": "container", "container.name": "container", "k8s.namespace.name": "k8s_namespace", "k8s.node.name": "k8s_node",
	"cloud.provider": "cloud_provider", "cloud.account.id": "cloud_account", "cloud.region": "cloud_region", "cloud.platform": "cloud_platform",
	"http.route": "http_route", "http.method": "http_method", "db.system": "db_system", "messaging.system": "messaging_system",
	"messaging.destination.name":    "messaging_destination",
	"messaging.consumer.group.name": "messaging_consumer_group",
}

var scopeColumns = map[string]string{
	"metric": "|service|host|environment|pod|container|k8s_namespace|k8s_node|cloud_provider|cloud_account|cloud_region|cloud_platform|",
	"apm":    "|service|host|environment|pod|k8s_node|cloud_provider|cloud_region|cloud_platform|http_route|http_method|db_system|messaging_system|messaging_destination|messaging_consumer_group|",
	"log":    "|service|host|environment|pod|container|",
}

func CompileScope(signal string, scope models.Scope, args []any) (string, []any, error) {
	allowed := scopeColumns[signal]
	var clause string
	for i, tag := range scope.Tags {
		key, value := strings.TrimSpace(tag.Key), strings.TrimSpace(tag.Value)
		column := scopeAliases[key]
		if column == "" || !strings.Contains(allowed, "|"+column+"|") {
			return "", nil, fmt.Errorf("%s monitor does not support scope %q", signal, key)
		}
		if value == "" {
			return "", nil, fmt.Errorf("scope %q requires a value", key)
		}
		bind := "scope" + strconv.Itoa(i)
		clause += " AND " + column + " = @" + bind
		args = append(args, clickhouse.Named(bind, value))
	}
	return clause, args, nil
}

func monitorWindowSec(v int) int64 {
	if v <= 0 {
		return 300
	}
	return int64(v)
}

func completeWindow(now time.Time, windowSec, grainSec int64) (int64, int64) {
	end := now.UTC().Truncate(time.Duration(grainSec) * time.Second).UnixMilli()
	return end - windowSec*1000, end
}
