package query

import (
	"context"
	"errors"
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

func DecodeScope(row models.MonitorRow) models.Scope {
	return row.Scope
}

func DecodeQuery(row models.MonitorRow) models.MonitorQuery {
	return row.Query
}

func DecodeConditions(row models.MonitorRow) models.Conditions {
	return row.Conditions
}

func tenantIDArg(tenantID int64) any {
	return clickhouse.Named("tenantID", uint32(tenantID))
}
