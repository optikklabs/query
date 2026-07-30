package service

import (
	"math"
	"testing"
	"time"

	"github.com/optikklabs/query/internal/modules/deployments/models"
)

func TestBuildListResponseDerivesBaselineTimelineAndTrafficShare(t *testing.T) {
	start := time.Date(2026, 7, 30, 8, 0, 0, 0, time.UTC)
	rows := []models.RawDeploymentRow{
		{
			Service: "checkout", Environment: "prod", Version: "v2",
			FirstSeen: start.Add(30 * time.Minute), Requests: 300, Errors: 6, QS: []float64{240},
		},
		{
			Service: "checkout", Environment: "prod", Version: "v1",
			FirstSeen: start, Requests: 100, Errors: 1, QS: []float64{200},
		},
	}

	got := buildListResponse(rows, start.Add(time.Hour).UnixMilli())
	if got.Summary.DeploymentCount != 2 || got.Summary.ServiceCount != 1 || got.Summary.EnvironmentCount != 1 {
		t.Fatalf("unexpected summary: %+v", got.Summary)
	}
	if len(got.Results) != 2 {
		t.Fatalf("got %d results, want 2", len(got.Results))
	}

	latest := got.Results[0]
	if latest.Version != "v2" {
		t.Fatalf("latest version = %q, want v2", latest.Version)
	}
	if latest.PreviousVersion == nil || *latest.PreviousVersion != "v1" {
		t.Fatalf("previous version = %v, want v1", latest.PreviousVersion)
	}
	if latest.ErrorRateDelta == nil || math.Abs(*latest.ErrorRateDelta-1) > 0.0001 {
		t.Fatalf("error-rate delta = %v, want 1pp", latest.ErrorRateDelta)
	}
	if latest.P95DeltaMs == nil || *latest.P95DeltaMs != 40 {
		t.Fatalf("p95 delta = %v, want 40ms", latest.P95DeltaMs)
	}

	var share float64
	for _, deployment := range got.Results {
		share += deployment.TrafficShare
	}
	if math.Abs(share-100) > 0.0001 {
		t.Fatalf("traffic shares sum to %v, want 100", share)
	}

	earliest := got.Results[1]
	if !earliest.TimelineEnd.Equal(latest.FirstSeen) {
		t.Fatalf("v1 timeline end = %v, want %v", earliest.TimelineEnd, latest.FirstSeen)
	}
}

func TestFindContextUsesEqualWindowsAndPrecedingFirstSeen(t *testing.T) {
	start := time.Date(2026, 7, 30, 8, 0, 0, 0, time.UTC)
	rows := []models.RawDeploymentRow{
		{Service: "checkout", Environment: "prod", Version: "v3", FirstSeen: start.Add(50 * time.Minute)},
		{Service: "checkout", Environment: "prod", Version: "v1", FirstSeen: start},
		{Service: "checkout", Environment: "prod", Version: "v2", FirstSeen: start.Add(20 * time.Minute)},
		{Service: "checkout", Environment: "staging", Version: "v9", FirstSeen: start.Add(10 * time.Minute)},
	}
	req := models.DetailRequest{
		ListRequest:    models.ListRequest{StartMs: start.UnixMilli(), EndMs: start.Add(time.Hour).UnixMilli()},
		Service:        "checkout",
		Version:        "v2",
		Environment:    "prod",
		EnvironmentSet: true,
	}

	got, err := findContext(rows, req)
	if err != nil {
		t.Fatalf("findContext returned error: %v", err)
	}
	if got.BaselineVersion == nil || *got.BaselineVersion != "v1" {
		t.Fatalf("baseline = %v, want v1", got.BaselineVersion)
	}
	if !got.Window.CurrentEnd.Equal(start.Add(50 * time.Minute)) {
		t.Fatalf("current end = %v, want next first-seen", got.Window.CurrentEnd)
	}
	currentLength := got.Window.CurrentEnd.Sub(got.Window.CurrentStart)
	baselineLength := got.Window.BaselineEnd.Sub(got.Window.BaselineStart)
	if currentLength != baselineLength {
		t.Fatalf("window lengths differ: current=%v baseline=%v", currentLength, baselineLength)
	}
	if !got.Window.BaselineStart.Equal(start.Add(-10 * time.Minute)) {
		t.Fatalf("baseline start = %v, want %v", got.Window.BaselineStart, start.Add(-10*time.Minute))
	}
}

func TestComparisonMetricsWithoutBaselineLeavesDeltasNull(t *testing.T) {
	got := comparisonMetrics(models.RawComparisonRow{
		CurrentRequests: 10,
		CurrentErrors:   1,
		CurrentQS:       []float64{1, 2, 3, 4, 5},
	}, false)

	if got.Requests.Current != 10 || got.ErrorRate.Current != 10 {
		t.Fatalf("unexpected current metrics: %+v", got)
	}
	if got.Requests.Baseline != nil || got.Requests.Delta != nil || got.Requests.DeltaPercent != nil {
		t.Fatalf("baseline fields must be null without a baseline: %+v", got.Requests)
	}
}
