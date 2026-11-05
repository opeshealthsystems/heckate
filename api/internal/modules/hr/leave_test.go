package hr

import (
	"testing"
	"time"
)

func d(s string) time.Time { t, _ := time.Parse("2006-01-02", s); return t }

func TestWorkingDays(t *testing.T) {
	// Mon 2026-10-19 to Sun 2026-10-25: five working days
	if got := WorkingDays(d("2026-10-19"), d("2026-10-25"), nil); got != 5 {
		t.Errorf("week = %d", got)
	}
	if got := WorkingDays(d("2026-10-24"), d("2026-10-25"), nil); got != 0 {
		t.Errorf("weekend = %d", got)
	}
	if got := WorkingDays(d("2026-10-19"), d("2026-10-23"), map[string]bool{"2026-10-21": true}); got != 4 {
		t.Errorf("with holiday = %d", got)
	}
	if got := WorkingDays(d("2026-10-19"), d("2026-10-19"), nil); got != 1 {
		t.Errorf("single day = %d", got)
	}
}

func TestEntitlement(t *testing.T) {
	if got := Entitlement(18, d("2020-03-15"), 2026); got != 18 {
		t.Errorf("full year = %d", got)
	}
	if got := Entitlement(18, d("2026-01-01"), 2026); got != 18 {
		t.Errorf("hired 1 Jan = %d", got)
	}
	if got := Entitlement(18, d("2026-07-01"), 2026); got != 9 {
		t.Errorf("hired 1 Jul = %d", got)
	}
	if got := Entitlement(18, d("2026-07-15"), 2026); got != 7 { // 5 whole months -> 18*5/12 = 7
		t.Errorf("hired mid Jul = %d", got)
	}
	if got := Entitlement(18, d("2027-01-01"), 2026); got != 0 {
		t.Errorf("not yet hired = %d", got)
	}
}

func TestOverlaps(t *testing.T) {
	if !Overlaps(d("2026-10-01"), d("2026-10-05"), d("2026-10-05"), d("2026-10-09")) {
		t.Error("sharing a boundary day overlaps")
	}
	if Overlaps(d("2026-10-01"), d("2026-10-04"), d("2026-10-05"), d("2026-10-09")) {
		t.Error("adjacent ranges do not overlap")
	}
}
