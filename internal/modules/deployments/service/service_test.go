package service

import (
	"testing"
	"time"

	"github.com/optikklabs/query/internal/modules/deployments/models"
)

var (
	t0 = time.Date(2026, 7, 30, 22, 0, 0, 0, time.UTC)
	t1 = t0.Add(1 * time.Hour)  // next deployment at T0+1h
	t2 = t0.Add(2 * time.Hour)  // picker end or third deploy
)

func ms(t time.Time) int64 { return t.UnixMilli() }

func makeRows(service, env string, versions ...struct {
	version   string
	firstSeen time.Time
}) []models.RawDeploymentRow {
	rows := make([]models.RawDeploymentRow, len(versions))
	for i, v := range versions {
		rows[i] = models.RawDeploymentRow{
			Service:     service,
			Environment: env,
			Version:     v.version,
			FirstSeen:   v.firstSeen,
		}
	}
	return rows
}

func ver(version string, firstSeen time.Time) struct {
	version   string
	firstSeen time.Time
} {
	return struct {
		version   string
		firstSeen time.Time
	}{version, firstSeen}
}

func TestFindContext_PickerClampsWindowStart(t *testing.T) {
	// Deployment v1.0 at T0, v1.1 at T1. Picker range starts at T0+30m.
	// The current window should be [T0+30m, T1], not [T0, T1].
	rows := makeRows("svc", "prod", ver("1.0.0", t0), ver("1.1.0", t1))
	pickerStart := t0.Add(30 * time.Minute)

	req := models.DetailRequest{
		ListRequest: models.ListRequest{
			TenantID: 1,
			StartMs:  ms(pickerStart),
			EndMs:    ms(t2),
		},
		Service:        "svc",
		Version:        "1.0.0",
		Environment:    "prod",
		EnvironmentSet: true,
	}

	ctx, err := findContext(rows, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Current window should be clamped to [pickerStart, T1]
	if !ctx.Window.CurrentStart.Equal(pickerStart) {
		t.Errorf("CurrentStart = %v, want %v", ctx.Window.CurrentStart, pickerStart)
	}
	if !ctx.Window.CurrentEnd.Equal(t1) {
		t.Errorf("CurrentEnd = %v, want %v", ctx.Window.CurrentEnd, t1)
	}

	// Baseline should mirror: 30 min before pickerStart
	expectedDuration := t1.Sub(pickerStart) // 30 min
	expectedBaselineStart := pickerStart.Add(-expectedDuration)
	if !ctx.Window.BaselineStart.Equal(expectedBaselineStart) {
		t.Errorf("BaselineStart = %v, want %v", ctx.Window.BaselineStart, expectedBaselineStart)
	}
	if !ctx.Window.BaselineEnd.Equal(pickerStart) {
		t.Errorf("BaselineEnd = %v, want %v", ctx.Window.BaselineEnd, pickerStart)
	}
}

func TestFindContext_PickerBeforeFirstSeen(t *testing.T) {
	// Picker starts before the deployment — window should start at FirstSeen.
	rows := makeRows("svc", "prod", ver("1.0.0", t0), ver("1.1.0", t1))
	pickerStart := t0.Add(-1 * time.Hour) // 1h before first seen

	req := models.DetailRequest{
		ListRequest: models.ListRequest{
			TenantID: 1,
			StartMs:  ms(pickerStart),
			EndMs:    ms(t2),
		},
		Service:        "svc",
		Version:        "1.0.0",
		Environment:    "prod",
		EnvironmentSet: true,
	}

	ctx, err := findContext(rows, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Window start should be FirstSeen (not picker start which is earlier)
	if !ctx.Window.CurrentStart.Equal(t0) {
		t.Errorf("CurrentStart = %v, want %v", ctx.Window.CurrentStart, t0)
	}
	if !ctx.Window.CurrentEnd.Equal(t1) {
		t.Errorf("CurrentEnd = %v, want %v", ctx.Window.CurrentEnd, t1)
	}
}

func TestFindContext_LastDeploymentUsesPickerEnd(t *testing.T) {
	// Only one deployment, no next deployment — window end should be pickerEnd.
	rows := makeRows("svc", "prod", ver("1.0.0", t0))
	pickerEnd := t0.Add(15 * time.Minute)

	req := models.DetailRequest{
		ListRequest: models.ListRequest{
			TenantID: 1,
			StartMs:  ms(t0),
			EndMs:    ms(pickerEnd),
		},
		Service:        "svc",
		Version:        "1.0.0",
		Environment:    "prod",
		EnvironmentSet: true,
	}

	ctx, err := findContext(rows, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !ctx.Window.CurrentStart.Equal(t0) {
		t.Errorf("CurrentStart = %v, want %v", ctx.Window.CurrentStart, t0)
	}
	if !ctx.Window.CurrentEnd.Equal(pickerEnd) {
		t.Errorf("CurrentEnd = %v, want %v", ctx.Window.CurrentEnd, pickerEnd)
	}

	// Baseline: 15 min before T0
	expectedBaselineStart := t0.Add(-15 * time.Minute)
	if !ctx.Window.BaselineStart.Equal(expectedBaselineStart) {
		t.Errorf("BaselineStart = %v, want %v", ctx.Window.BaselineStart, expectedBaselineStart)
	}
}

func TestFindContext_NotFound(t *testing.T) {
	rows := makeRows("svc", "prod", ver("1.0.0", t0))

	req := models.DetailRequest{
		ListRequest:    models.ListRequest{TenantID: 1, StartMs: ms(t0), EndMs: ms(t2)},
		Service:        "svc",
		Version:        "2.0.0", // doesn't exist
		Environment:    "prod",
		EnvironmentSet: true,
	}

	_, err := findContext(rows, req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestFindContext_NoTrafficWindow(t *testing.T) {
	// Picker range ends at or before FirstSeen — no valid window.
	rows := makeRows("svc", "prod", ver("1.0.0", t0))

	req := models.DetailRequest{
		ListRequest: models.ListRequest{
			TenantID: 1,
			StartMs:  ms(t0.Add(-1 * time.Hour)),
			EndMs:    ms(t0), // ends exactly at FirstSeen
		},
		Service:        "svc",
		Version:        "1.0.0",
		Environment:    "prod",
		EnvironmentSet: true,
	}

	_, err := findContext(rows, req)
	if err == nil {
		t.Fatal("expected validation error for zero-duration window, got nil")
	}
}

func TestFindContext_BaselineVersion(t *testing.T) {
	rows := makeRows("svc", "prod",
		ver("1.0.0", t0),
		ver("1.1.0", t1),
	)

	req := models.DetailRequest{
		ListRequest:    models.ListRequest{TenantID: 1, StartMs: ms(t0), EndMs: ms(t2)},
		Service:        "svc",
		Version:        "1.1.0",
		Environment:    "prod",
		EnvironmentSet: true,
	}

	ctx, err := findContext(rows, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ctx.BaselineVersion == nil {
		t.Fatal("expected BaselineVersion to be set")
	}
	if *ctx.BaselineVersion != "1.0.0" {
		t.Errorf("BaselineVersion = %q, want %q", *ctx.BaselineVersion, "1.0.0")
	}
}

func TestFindContext_NoBaseline(t *testing.T) {
	// First deployment has no baseline.
	rows := makeRows("svc", "prod", ver("1.0.0", t0))

	req := models.DetailRequest{
		ListRequest:    models.ListRequest{TenantID: 1, StartMs: ms(t0), EndMs: ms(t2)},
		Service:        "svc",
		Version:        "1.0.0",
		Environment:    "prod",
		EnvironmentSet: true,
	}

	ctx, err := findContext(rows, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ctx.BaselineVersion != nil {
		t.Errorf("expected nil BaselineVersion, got %q", *ctx.BaselineVersion)
	}
}

func TestComputeWindow_ClampsBothEnds(t *testing.T) {
	firstSeen := t0
	next := t0.Add(2 * time.Hour)

	// Picker is [T0+30m, T0+90m] — narrower than [firstSeen, next]
	pickerStart := t0.Add(30 * time.Minute)
	pickerEnd := t0.Add(90 * time.Minute)

	w, err := computeWindow(firstSeen, &next, pickerStart, pickerEnd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !w.CurrentStart.Equal(pickerStart) {
		t.Errorf("CurrentStart = %v, want %v", w.CurrentStart, pickerStart)
	}
	if !w.CurrentEnd.Equal(pickerEnd) {
		t.Errorf("CurrentEnd = %v, want %v", w.CurrentEnd, pickerEnd)
	}

	duration := pickerEnd.Sub(pickerStart) // 60 min
	if !w.BaselineStart.Equal(pickerStart.Add(-duration)) {
		t.Errorf("BaselineStart = %v, want %v", w.BaselineStart, pickerStart.Add(-duration))
	}
	if !w.BaselineEnd.Equal(pickerStart) {
		t.Errorf("BaselineEnd = %v, want %v", w.BaselineEnd, pickerStart)
	}
}

func TestComputeWindow_NilNextFirstSeen(t *testing.T) {
	w, err := computeWindow(t0, nil, t0, t0.Add(30*time.Minute))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !w.CurrentEnd.Equal(t0.Add(30 * time.Minute)) {
		t.Errorf("CurrentEnd = %v, want %v", w.CurrentEnd, t0.Add(30*time.Minute))
	}
}

func TestComputeWindow_ZeroDuration(t *testing.T) {
	// pickerEnd == firstSeen → zero duration → error
	_, err := computeWindow(t0, nil, t0, t0)
	if err == nil {
		t.Fatal("expected error for zero-duration window")
	}
}
