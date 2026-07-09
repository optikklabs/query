package ingestion

import "testing"

func TestProjectMonthEnd(t *testing.T) {
	tests := []struct {
		name      string
		daily     []uint64
		totalDays int
		want      uint64
	}{
		{"empty", nil, 30, 0},
		{"flat week extrapolates to month", []uint64{10, 10, 10, 10, 10, 10, 10}, 30, 300},
		{"uses only trailing 7 days", []uint64{999, 999, 10, 10, 10, 10, 10, 10, 10}, 30, 300},
		{"short history averages what exists", []uint64{5, 10, 15}, 30, 300},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := projectMonthEnd(tt.daily, tt.totalDays); got != tt.want {
				t.Errorf("projectMonthEnd(%v, %d) = %d, want %d", tt.daily, tt.totalDays, got, tt.want)
			}
		})
	}
}

func TestAggregateServices(t *testing.T) {
	logs := []svcCountRow{{Service: "checkout", Env: "prod", Count: 100, Bytes: 1000}}
	spans := []svcCountRow{{Service: "checkout", Env: "prod", Count: 40, Bytes: 400}}
	ts := []svcCountRow{{Service: "checkout", Count: 7}}

	got := aggregateServices(logs, spans, ts)
	a := got["checkout"]
	if a == nil {
		t.Fatal("checkout missing from aggregate")
	}
	if a.records() != 140 {
		t.Errorf("records: want 140, got %d", a.records())
	}
	if a.bytes() != 1400 {
		t.Errorf("bytes: want 1400, got %d", a.bytes())
	}
	if a.timeseries != 7 {
		t.Errorf("timeseries: want 7, got %d", a.timeseries)
	}
	if a.env != "prod" {
		t.Errorf("env: want prod, got %q", a.env)
	}
}

func TestBuildServicesResponseRanksAndShares(t *testing.T) {
	services := map[string]*serviceAgg{
		"big":   {logs: 800, spans: 0, logsBytes: 8000},
		"small": {logs: 200, spans: 0, logsBytes: 2000},
	}
	resp := buildServicesResponse(services, map[string]uint64{"big": 400}, map[string]*svcSeries{}, 3)

	if resp.TotalServices != 2 {
		t.Fatalf("total services: want 2, got %d", resp.TotalServices)
	}
	if resp.Services[0].Name != "big" {
		t.Errorf("ranking: want big first, got %q", resp.Services[0].Name)
	}
	if resp.Services[0].Pct != 80 {
		t.Errorf("big record share: want 80, got %v", resp.Services[0].Pct)
	}
	if resp.Services[0].BytesPct != 80 {
		t.Errorf("big byte share: want 80, got %v", resp.Services[0].BytesPct)
	}
	// prior 400 → 800 is +100%.
	if resp.Services[0].DeltaPct != 100 {
		t.Errorf("big delta: want 100, got %v", resp.Services[0].DeltaPct)
	}
	// Sparks must be zero-filled, never nil, for services without daily rows.
	if len(resp.Services[0].Spark) != 3 || len(resp.Services[0].ByteSpark) != 3 {
		t.Errorf("sparks should be zero-filled to length 3")
	}
}

func TestBuildServicesResponseSkipsUnattributed(t *testing.T) {
	// Empty-name entry (e.g. servicegraph edge metrics) must not surface as a
	// blank service row nor inflate the count.
	services := map[string]*serviceAgg{
		"real": {logs: 100, logsBytes: 1000},
		"":     {timeseries: 3},
	}
	resp := buildServicesResponse(services, map[string]uint64{}, map[string]*svcSeries{}, 3)

	if resp.TotalServices != 1 {
		t.Fatalf("total services: want 1 (empty skipped), got %d", resp.TotalServices)
	}
	for _, s := range resp.Services {
		if s.Name == "" {
			t.Errorf("unattributed empty-name service should not appear in rows")
		}
	}
}
