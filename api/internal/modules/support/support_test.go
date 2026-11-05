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

func TestTransitions(t *testing.T) {
	ok := [][2]string{{"open", "in_progress"}, {"in_progress", "resolved"}, {"resolved", "closed"}, {"resolved", "in_progress"}}
	for _, m := range ok {
		if !canMove(m[0], m[1]) {
			t.Errorf("%s -> %s should be allowed", m[0], m[1])
		}
	}
	bad := [][2]string{{"closed", "open"}, {"open", "closed"}, {"resolved", "open"}}
	for _, m := range bad {
		if canMove(m[0], m[1]) {
			t.Errorf("%s -> %s should be rejected", m[0], m[1])
		}
	}
}
