package consumer

import (
	"context"
	"testing"
	"time"
)

type fakeRepo struct {
	rateRows []TopicCounterRow
	lagRows  []GroupTopicGaugeRow
	err      error
}

func (f fakeRepo) QueryConsumeRateByTopic(context.Context, int64, int64, int64) ([]TopicCounterRow, error) {
	return f.rateRows, f.err
}

func (f fakeRepo) QueryConsumerLagByGroupTopic(context.Context, int64, int64, int64) ([]GroupTopicGaugeRow, error) {
	return f.lagRows, f.err
}

// Flow: delta counter rows in one 1m bucket -> rate is sum / 60s.
func TestGetConsumeRateByTopic_RatePerSec(t *testing.T) {
	base := time.Date(2026, 6, 24, 10, 0, 0, 0, time.UTC)
	rows := []TopicCounterRow{
		{Timestamp: base.Add(5 * time.Second), Topic: "orders", Value: 30},
		{Timestamp: base.Add(40 * time.Second), Topic: "orders", Value: 30},
	}
	endMs := base.Add(20 * time.Minute).UnixMilli()

	out, err := NewService(fakeRepo{rateRows: rows}).GetConsumeRateByTopic(context.Background(), 1, base.UnixMilli(), endMs)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 {
		t.Fatalf("got %d points, want 1: %+v", len(out), out)
	}
	if got, want := out[0].RatePerSec, 60.0/60.0; got != want {
		t.Errorf("rate = %v, want %v", got, want)
	}
}

func TestGetConsumerLagByGroup_Averages(t *testing.T) {
	base := time.Date(2026, 6, 24, 10, 0, 0, 0, time.UTC)
	rows := []GroupTopicGaugeRow{
		{Timestamp: base.Add(1 * time.Second), ConsumerGroup: "g1", Topic: "orders", Value: 10},
		{Timestamp: base.Add(2 * time.Second), ConsumerGroup: "g1", Topic: "orders", Value: 20},
		{Timestamp: base.Add(3 * time.Second), ConsumerGroup: "g1", Topic: "orders", Value: 30},
	}
	endMs := base.Add(20 * time.Minute).UnixMilli()

	out, err := NewService(fakeRepo{lagRows: rows}).GetConsumerLagByGroup(context.Background(), 1, base.UnixMilli(), endMs)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 {
		t.Fatalf("got %d points, want 1: %+v", len(out), out)
	}
	if out[0].ConsumerGroup != "g1" || out[0].Topic != "orders" {
		t.Errorf("dims = %+v, want g1/orders", out[0])
	}
	if got, want := out[0].Lag, (10.0+20.0+30.0)/3.0; got != want {
		t.Errorf("lag = %v, want %v (avg, not sum)", got, want)
	}
}

func TestGetConsumerLagByGroup_PerGroupSorted(t *testing.T) {
	base := time.Date(2026, 6, 24, 10, 0, 0, 0, time.UTC)
	rows := []GroupTopicGaugeRow{
		{Timestamp: base, ConsumerGroup: "g2", Topic: "orders", Value: 5},
		{Timestamp: base, ConsumerGroup: "g1", Topic: "orders", Value: 7},
	}
	endMs := base.Add(20 * time.Minute).UnixMilli()

	out, err := NewService(fakeRepo{lagRows: rows}).GetConsumerLagByGroup(context.Background(), 1, base.UnixMilli(), endMs)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Fatalf("got %d points, want 2: %+v", len(out), out)
	}
	if out[0].ConsumerGroup != "g1" || out[1].ConsumerGroup != "g2" {
		t.Errorf("groups not sorted: %+v", out)
	}
}
