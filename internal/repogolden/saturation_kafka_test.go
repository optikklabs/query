package repogolden

import (
	"context"
	"fmt"
	"strings"
	"testing"

	kafkarepo "github.com/optikklabs/query/internal/modules/saturation/kafka/repository"
	"github.com/optikklabs/query/internal/shared/chtest"
)

// TestSaturationKafkaRepoSQL pins the SQL of every read in the Kafka
// saturation domain. The two explorer queries are assembled from a shared
// seriesCTE builder, so both the filtered and unfiltered shapes are recorded.
func TestSaturationKafkaRepoSQL(t *testing.T) {
	ctx := context.Background()
	rec := &chtest.Recorder{}
	var b strings.Builder

	record := func(name string, call func()) {
		rec.Reset()
		call()
		fmt.Fprintf(&b, "=== %s\n%s\n", name, rec.Render())
	}

	repo := kafkarepo.NewRepository(rec)

	record("explorer.QueryTopicThroughput", func() {
		_, _ = repo.QueryTopicThroughput(ctx, tenantID, startMs, endMs, "")
	})
	record("explorer.QueryTopicThroughput/topic", func() {
		_, _ = repo.QueryTopicThroughput(ctx, tenantID, startMs, endMs, "orders")
	})
	record("explorer.QueryGroupPartitions", func() {
		_, _ = repo.QueryGroupPartitions(ctx, tenantID, startMs, endMs, "")
	})
	record("explorer.QueryGroupPartitions/group", func() {
		_, _ = repo.QueryGroupPartitions(ctx, tenantID, startMs, endMs, "checkout-workers")
	})

	record("topology.QueryClients", func() {
		_, _ = repo.QueryClients(ctx, tenantID, startMs, endMs)
	})
	record("topology.QueryEdges", func() {
		_, _ = repo.QueryEdges(ctx, tenantID, startMs, endMs, []string{"checkout", "orders"})
	})

	compareGolden(t, "saturation_kafka.golden.txt", b.String())
}
