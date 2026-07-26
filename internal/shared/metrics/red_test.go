package metrics

import "testing"

// REDDerivations guards zero requests and computes error% + mean latency.
// Cases merged from the three modules that each carried their own copy:
// infrastructure (nodes, fleet) and cloud.
func TestREDDerivations(t *testing.T) {
	if er, al := REDDerivations(0, 5, 100); er != 0 || al != 0 {
		t.Errorf("zero requests = (%v,%v), want (0,0)", er, al)
	}
	if er, al := REDDerivations(4, 1, 200); er != 25 || al != 50 {
		t.Errorf("REDDerivations(4,1,200) = (%v,%v), want (25,50)", er, al)
	}
	if er, al := REDDerivations(8, 2, 400); er != 25 || al != 50 {
		t.Errorf("REDDerivations(8,2,400) = (%v,%v), want (25,50)", er, al)
	}
}
