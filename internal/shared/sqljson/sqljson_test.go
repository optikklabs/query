package sqljson

import "testing"

func TestStringListRoundTrip(t *testing.T) {
	v, err := StringList(nil).Value()
	if err != nil || string(v.([]byte)) != "[]" {
		t.Fatalf("nil list stored as %q, %v; want []", v, err)
	}

	var l StringList
	for _, src := range []any{[]byte(`["a","b"]`), `["a","b"]`} {
		if err := l.Scan(src); err != nil {
			t.Fatalf("Scan(%T): %v", src, err)
		}
		if len(l) != 2 || l[0] != "a" || l[1] != "b" {
			t.Fatalf("Scan(%T) = %v", src, l)
		}
	}
}

func TestScanNullLeavesDestination(t *testing.T) {
	l := StringList{"keep"}
	if err := l.Scan(nil); err != nil || len(l) != 1 {
		t.Fatalf("Scan(nil) = %v, %v", l, err)
	}
}

func TestScanRejectsOtherTypes(t *testing.T) {
	var l StringList
	if err := l.Scan(42); err == nil {
		t.Fatal("Scan(int) succeeded")
	}
}
