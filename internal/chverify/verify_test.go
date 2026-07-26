// Package chverify drives the repository methods touched by the span_stats
// alias fix against a real ClickHouse, so the rendered SQL, its named params,
// and the `ch:` column binding are all exercised together.
//
// Local verification only — skipped unless CHVERIFY_DSN is set.
package chverify_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	database "github.com/optikklabs/query/internal/infra/database"
	models "github.com/optikklabs/query/internal/modules/alerting/shared/models"
	apmquery "github.com/optikklabs/query/internal/modules/alerting/shared/query"
	"github.com/optikklabs/query/internal/modules/cloud"
	infrarepo "github.com/optikklabs/query/internal/modules/infrastructure/repository"
	dbfilter "github.com/optikklabs/query/internal/modules/saturation/database/filter"
	dbrepo "github.com/optikklabs/query/internal/modules/saturation/database/repository"
	errmod "github.com/optikklabs/query/internal/modules/services/errors"
	"github.com/optikklabs/query/internal/modules/services/redfleet"
	"github.com/optikklabs/query/internal/modules/services/topology"
)

const (
	tenantID = 1
	service  = "cart"
	dbSystem = "redis"
)

var startMs, endMs int64

func conn(t *testing.T) clickhouse.Conn {
	t.Helper()
	dsn := os.Getenv("CHVERIFY_DSN")
	if dsn == "" {
		t.Skip("set CHVERIFY_DSN to run (e.g. clickhouse://default:@127.0.0.1:9000/optikk)")
	}
	c, err := database.OpenClickHouseConn(dsn, 4, 2)
	if err != nil {
		t.Fatalf("open clickhouse: %v", err)
	}
	return c
}

func TestMain(m *testing.M) {
	now := time.Now().UTC()
	endMs = now.UnixMilli()
	startMs = now.Add(-6 * time.Hour).UnixMilli()
	os.Exit(m.Run())
}

// run reports the error instead of failing fast, so one broken query does not
// hide the state of the rest.
func run(t *testing.T, name string, fn func() error) {
	t.Helper()
	if err := fn(); err != nil {
		t.Errorf("%s: %v", name, err)
		return
	}
	t.Logf("%s: ok", name)
}

func TestRedfleet(t *testing.T) {
	ctx, r := context.Background(), redfleet.NewRepository(conn(t))
	// Both filter states matter: the top-endpoints bug only appeared once a
	// service predicate reached the PREWHERE alongside `any(service)`.
	for _, f := range []redfleet.REDFilters{
		{TenantID: tenantID, StartMs: startMs, EndMs: endMs},
		{TenantID: tenantID, StartMs: startMs, EndMs: endMs, Services: []string{service}},
		{TenantID: tenantID, StartMs: startMs, EndMs: endMs, Services: []string{service, "checkout"}},
	} {
		label := "fleet-wide"
		if n := len(f.Services); n > 0 {
			label = "filtered/" + string(rune('0'+n)) + "svc"
		}
		run(t, label+" GetFleetREDMetrics", func() error { _, e := r.GetFleetREDMetrics(ctx, f); return e })
		run(t, label+" GetRequestAndErrorRateTimeSeries", func() error {
			_, e := r.GetRequestAndErrorRateTimeSeries(ctx, f)
			return e
		})
		run(t, label+" GetStatusTimeSeries", func() error { _, e := r.GetStatusTimeSeries(ctx, f); return e })
		run(t, label+" GetLatencyPercentilesTimeSeries", func() error {
			_, e := r.GetLatencyPercentilesTimeSeries(ctx, f)
			return e
		})
		run(t, label+" GetREDByEndpointTimeSeries", func() error { _, e := r.GetREDByEndpointTimeSeries(ctx, f); return e })
		run(t, label+" GetRequestRateTimeSeries", func() error { _, e := r.GetRequestRateTimeSeries(ctx, f); return e })
		// Cursor set and unset: the pagination predicate references the
		// renamed measure alias.
		for _, cur := range []redfleet.TopEndpointsCursor{{}, {TotalCount: 1000, OperationName: "a"}} {
			run(t, label+" GetTopEndpointsCombined", func() error {
				_, e := r.GetTopEndpointsCombined(ctx, f, 20, cur)
				return e
			})
			run(t, label+" GetTopDBQueriesCombined", func() error {
				_, e := r.GetTopDBQueriesCombined(ctx, f, 20, cur)
				return e
			})
		}
	}
	run(t, "GetOperationBaseline", func() error {
		_, e := r.GetOperationBaseline(ctx, tenantID, startMs, endMs, service, "oteldemo.CartService/GetCart")
		return e
	})
}

func TestErrors(t *testing.T) {
	ctx, r := context.Background(), errmod.NewRepository(conn(t))
	run(t, "ServiceErrorRateRowsAll", func() error {
		_, e := r.ServiceErrorRateRowsAll(ctx, tenantID, startMs, endMs)
		return e
	})
	run(t, "ServiceErrorRateRowsByService", func() error {
		_, e := r.ServiceErrorRateRowsByService(ctx, tenantID, startMs, endMs, service)
		return e
	})
	run(t, "ErrorVolumeRowsAll", func() error { _, e := r.ErrorVolumeRowsAll(ctx, tenantID, startMs, endMs); return e })
	run(t, "ErrorVolumeRowsByService", func() error {
		_, e := r.ErrorVolumeRowsByService(ctx, tenantID, startMs, endMs, service)
		return e
	})
}

func TestTopology(t *testing.T) {
	ctx, r := context.Background(), topology.NewRepository(conn(t))
	for _, focus := range []string{"", service} {
		run(t, "GetNodes focus="+focus, func() error { _, e := r.GetNodes(ctx, tenantID, startMs, endMs, focus); return e })
		run(t, "GetEdges focus="+focus, func() error { _, e := r.GetEdges(ctx, tenantID, startMs, endMs, focus); return e })
	}
}

func TestCloud(t *testing.T) {
	ctx, r := context.Background(), cloud.NewRepository(conn(t))
	run(t, "QueryProviderHealth", func() error { _, e := r.QueryProviderHealth(ctx, tenantID, startMs, endMs); return e })
	run(t, "QueryProviderResources", func() error {
		_, e := r.QueryProviderResources(ctx, tenantID, "aws", startMs, endMs)
		return e
	})
}

func TestInfrastructure(t *testing.T) {
	ctx, c := context.Background(), conn(t)
	nr := infrarepo.NewRepository(c)
	run(t, "QueryInfrastructureNodes", func() error {
		_, e := nr.QueryInfrastructureNodes(ctx, tenantID, startMs, endMs)
		return e
	})
	run(t, "QueryInfrastructureNodeSummary", func() error {
		_, e := nr.QueryInfrastructureNodeSummary(ctx, tenantID, startMs, endMs)
		return e
	})
	run(t, "QueryInfrastructureNodeServices", func() error {
		_, e := nr.QueryInfrastructureNodeServices(ctx, tenantID, "some-host", startMs, endMs)
		return e
	})
	fr := nr
	run(t, "QueryFleetPods", func() error { _, e := fr.QueryFleetPods(ctx, tenantID, startMs, endMs, ""); return e })
	cd := nr
	run(t, "QueryPodMeta", func() error { _, e := cd.QueryPodMeta(ctx, tenantID, "some-pod", startMs, endMs); return e })
	run(t, "QueryPodRED", func() error { _, e := cd.QueryPodRED(ctx, tenantID, "some-pod", startMs, endMs); return e })
}

func TestQueryDetail(t *testing.T) {
	ctx, r := context.Background(), dbrepo.NewRepository(conn(t))
	// Unfiltered, then with the dbSystem filter that made any(db_system) fail.
	for _, f := range []dbfilter.Filters{{}, {DBSystem: []string{dbSystem}}} {
		run(t, "GetSummary", func() error {
			_, e := r.GetSummary(ctx, tenantID, startMs, endMs, "somehash", f)
			return e
		})
	}
}

// APM alerting has no dashboard that would surface a failure, so it is the
// one path where only a direct call proves the query runs.
func TestAPMAlerting(t *testing.T) {
	ctx, b := context.Background(), apmquery.NewAPMBackend(conn(t))
	mon := models.MonitorRow{TenantID: tenantID}
	q := models.MonitorQuery{APM: &models.APMQuery{
		Service: service, Track: "error_rate", WindowSec: 300,
	}}
	now := time.Now().UTC()
	for _, track := range []string{"error_rate", "latency_p99", "throughput"} {
		q.APM.Track = track
		run(t, "apm.Scalar track="+track, func() error {
			_, e := b.Scalar(ctx, mon, q, models.Scope{}, models.Conditions{}, now)
			return e
		})
		run(t, "apm.Series track="+track, func() error {
			_, e := b.Series(ctx, mon, q, models.Scope{}, models.Conditions{}, 3600_000, now)
			return e
		})
	}
}
