package repogolden

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/optikklabs/query/internal/modules/infrastructure/containerdetail"
	"github.com/optikklabs/query/internal/modules/infrastructure/cpu"
	"github.com/optikklabs/query/internal/modules/infrastructure/fleet"
	"github.com/optikklabs/query/internal/modules/infrastructure/hostdetail"
	"github.com/optikklabs/query/internal/modules/infrastructure/hosts"
	"github.com/optikklabs/query/internal/modules/infrastructure/memory"
	"github.com/optikklabs/query/internal/modules/infrastructure/nodes"
	"github.com/optikklabs/query/internal/modules/infrastructure/seriesgroup"
	"github.com/optikklabs/query/internal/shared/chtest"
)

const (
	testHost = "node-a"
	testPod  = "checkout-7d9f"
)

// gaugeDef and rateDef exercise both seriesgroup value expressions: Gauge
// averages val_sum/val_count, Rate derives a per-second counter increase. The
// two produce materially different SQL from the same builder.
var (
	gaugeDef = seriesgroup.Def{
		ID:          "cpu",
		MetricNames: []string{"system.cpu.utilization"},
		LabelSQL:    "if(attributes['state'] = '', 'cpu', attributes['state'])",
		Agg:         seriesgroup.Gauge,
		Scale:       100,
	}
	rateDef = seriesgroup.Def{
		ID:          "network_io",
		MetricNames: []string{"system.network.io"},
		LabelSQL:    "trim(concat(attributes['device'], ' ', attributes['direction']))",
		Agg:         seriesgroup.Rate,
		Scale:       1,
	}
)

// TestInfrastructureRepoSQL pins the SQL of every repository method across the
// seven infrastructure modules, which are about to be merged into one package.
// hosts, cpu, memory and hostdetail had no repository-level test before this.
func TestInfrastructureRepoSQL(t *testing.T) {
	ctx := context.Background()
	rec := &chtest.Recorder{}
	var b strings.Builder

	record := func(name string, call func()) {
		rec.Reset()
		call()
		fmt.Fprintf(&b, "=== %s\n%s\n", name, rec.Render())
	}

	cpuRepo := cpu.NewRepository(rec)
	record("cpu.QueryCPUUtilizationAgg", func() {
		_, _ = cpuRepo.QueryCPUUtilizationAgg(ctx, tenantID, startMs, endMs)
	})
	record("cpu.QueryCPUUtilizationByInstance", func() {
		_, _ = cpuRepo.QueryCPUUtilizationByInstance(ctx, tenantID, startMs, endMs)
	})

	memRepo := memory.NewRepository(rec)
	record("memory.QueryMemoryUtilizationAgg", func() {
		_, _ = memRepo.QueryMemoryUtilizationAgg(ctx, tenantID, startMs, endMs)
	})
	record("memory.QueryMemoryUtilizationForInstance", func() {
		_, _ = memRepo.QueryMemoryUtilizationForInstance(ctx, tenantID, startMs, endMs, testHost, testPod, "checkout")
	})

	hostsRepo := hosts.NewRepository(rec)
	record("hosts.QueryHostUtilization", func() {
		_, _ = hostsRepo.QueryHostUtilization(ctx, tenantID, startMs, endMs)
	})
	record("hosts.QueryHostSpans", func() {
		_, _ = hostsRepo.QueryHostSpans(ctx, tenantID, startMs, endMs, "checkout")
	})

	// Unfiltered and host-filtered: the host predicate is a bind, so both
	// render the same SQL — recording each still pins the bind set.
	fleetRepo := fleet.NewRepository(rec)
	record("fleet.QueryFleetPods", func() {
		_, _ = fleetRepo.QueryFleetPods(ctx, tenantID, startMs, endMs, "")
	})
	record("fleet.QueryFleetPods/host", func() {
		_, _ = fleetRepo.QueryFleetPods(ctx, tenantID, startMs, endMs, testHost)
	})

	nodesRepo := nodes.NewRepository(rec)
	record("nodes.QueryInfrastructureNodes", func() {
		_, _ = nodesRepo.QueryInfrastructureNodes(ctx, tenantID, startMs, endMs)
	})
	// The summary derives from the node query rather than issuing its own, so
	// this records the same statement — that indirection is the thing pinned.
	record("nodes.QueryInfrastructureNodeSummary", func() {
		_, _ = nodesRepo.QueryInfrastructureNodeSummary(ctx, tenantID, startMs, endMs)
	})
	record("nodes.QueryInfrastructureNodeServices", func() {
		_, _ = nodesRepo.QueryInfrastructureNodeServices(ctx, tenantID, testHost, startMs, endMs)
	})

	hostDetailRepo := hostdetail.NewRepository(rec)
	record("hostdetail.QueryHostMeta", func() {
		_, _ = hostDetailRepo.QueryHostMeta(ctx, tenantID, testHost, startMs, endMs)
	})
	record("hostdetail.QueryKPIs", func() {
		_, _ = hostDetailRepo.QueryKPIs(ctx, tenantID, testHost, startMs, endMs)
	})
	record("hostdetail.QuerySeries/gauge", func() {
		_, _ = hostDetailRepo.QuerySeries(ctx, tenantID, testHost, startMs, endMs, gaugeDef)
	})
	record("hostdetail.QuerySeries/rate", func() {
		_, _ = hostDetailRepo.QuerySeries(ctx, tenantID, testHost, startMs, endMs, rateDef)
	})

	podRepo := containerdetail.NewRepository(rec)
	record("containerdetail.QueryPodMeta", func() {
		_, _ = podRepo.QueryPodMeta(ctx, tenantID, testPod, startMs, endMs)
	})
	record("containerdetail.QueryPodRED", func() {
		_, _ = podRepo.QueryPodRED(ctx, tenantID, testPod, startMs, endMs)
	})
	record("containerdetail.QuerySeries/gauge", func() {
		_, _ = podRepo.QuerySeries(ctx, tenantID, testPod, startMs, endMs, gaugeDef)
	})

	compareGolden(t, "infrastructure.golden.txt", b.String())
}
