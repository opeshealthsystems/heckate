package support

import "testing"

func TestSeverityValid(t *testing.T) {
	for _, s := range []Severity{P1, P2, P3, P4} {
		if !s.Valid() {
			t.Errorf("%s should be valid", s)
		}
	}
	if Severity("P5").Valid() {
		t.Error("P5 should be invalid")
	}
}
