package producer

import (
	"context"
	"testing"
	"time"
)

type fakeRepo struct {
	rows []TopicCounterRow
	err  error
}

func (f fakeRepo) QueryPublishRateByTopic(context.Context, int64, int64, int64) ([]TopicCounterRow, error) {
	return f.rows, f.err
}

// Flow: delta counter rows in one 1m bucket -> rate is sum / 60s (the verified
// 74 records/min = 1.2333/s ground truth). Guards the delta+grain math.
func TestGetProduceRateByTopic_RatePerSec(t *testing.T) {
	base := time.Date(2026, 6, 24, 10, 0, 0, 0, time.UTC)
	rows := []TopicCounterRow{
		{Timestamp: base.Add(10 * time.Second), Topic: "orders", Value: 40},
		{Timestamp: base.Add(30 * time.Second), Topic: "orders", Value: 34},
	}
	startMs := base.UnixMilli()
	endMs := base.Add(20 * time.Minute).UnixMilli()

	out, err := NewService(fakeRepo{rows: rows}).GetProduceRateByTopic(context.Background(), 1, startMs, endMs)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 {
		t.Fatalf("got %d points, want 1: %+v", len(out), out)
	}
	if out[0].Topic != "orders" {
		t.Errorf("topic = %q, want orders", out[0].Topic)
	}
	if got, want := out[0].RatePerSec, (40.0+34.0)/60.0; got != want {
		t.Errorf("rate = %v, want %v", got, want)
	}
}

func TestGetProduceRateByTopic_PerTopic(t *testing.T) {
	base := time.Date(2026, 6, 24, 10, 0, 0, 0, time.UTC)
	rows := []TopicCounterRow{
		{Timestamp: base, Topic: "orders", Value: 60},
		{Timestamp: base, Topic: "payments", Value: 120},
	}
	endMs := base.Add(20 * time.Minute).UnixMilli()

	out, err := NewService(fakeRepo{rows: rows}).GetProduceRateByTopic(context.Background(), 1, base.UnixMilli(), endMs)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Fatalf("got %d points, want 2: %+v", len(out), out)
	}

	if out[0].Topic != "orders" || out[0].RatePerSec != 1 {
		t.Errorf("orders point = %+v, want rate 1", out[0])
	}
	if out[1].Topic != "payments" || out[1].RatePerSec != 2 {
		t.Errorf("payments point = %+v, want rate 2", out[1])
	}
}

func TestGetProduceRateByTopic_Empty(t *testing.T) {
	out, err := NewService(fakeRepo{}).GetProduceRateByTopic(context.Background(), 1, 0, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 0 {
		t.Errorf("want no points, got %+v", out)
	}
}
