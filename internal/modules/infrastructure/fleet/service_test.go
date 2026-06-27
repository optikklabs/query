package fleet

import "testing"

// redDerivations computes error% and mean latency, guarding zero requests.
func TestRedDerivations(t *testing.T) {
	if er, al := redDerivations(0, 5, 100); er != 0 || al != 0 {
		t.Errorf("zero requests = (%v,%v), want (0,0)", er, al)
	}
	er, al := redDerivations(4, 1, 200)
	if er != 25 {
		t.Errorf("errorRate = %v, want 25", er)
	}
	if al != 50 {
		t.Errorf("avgLatency = %v, want 50", al)
	}
}
