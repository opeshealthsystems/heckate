package sales

import "testing"

func TestTotals(t *testing.T) {
	// 19.25% = 17.5% TVA + CAC of 10% of the TVA
	net, tva, cac, total := Totals([]Line{{"Pentest", 1, 1_000_000}}, 1750, 10)
	if net != 1_000_000 || tva != 175_000 || cac != 17_500 || total != 1_192_500 {
		t.Fatalf("got %d %d %d %d", net, tva, cac, total)
	}
}

func TestRoundingHalfUp(t *testing.T) {
	// 2.5 x 333 = 832.5 -> 833
	if got := LineAmount(Line{Quantity: 2.5, UnitPrice: 333}); got != 833 {
		t.Errorf("line amount %d", got)
	}
	// TVA on 1 XAF at 17.5% = 0.175 -> 0; on 3 XAF = 0.525 -> 1
	if tva, _ := Taxes(1, 1750, 10); tva != 0 {
		t.Errorf("tva(1)=%d", tva)
	}
	if tva, _ := Taxes(3, 1750, 10); tva != 1 {
		t.Errorf("tva(3)=%d", tva)
	}
}

func TestFractionalQuantityIsExact(t *testing.T) {
	// 0.1 x 3 in floats is 0.30000000000000004; hundredths avoid that.
	if qty100(0.1)*3 != 30 {
		t.Error("quantity must be handled in hundredths")
	}
	net, _, _, _ := Totals([]Line{{"x", 0.1, 100}, {"y", 0.2, 100}}, 0, 0)
	if net != 30 {
		t.Errorf("net=%d", net)
	}
}

func TestTotalsMultipleLinesRoundOnce(t *testing.T) {
	_, tva, _, _ := Totals([]Line{{"a", 1, 1}, {"b", 1, 1}, {"c", 1, 1}}, 1750, 10)
	// net 3 -> 0.525 -> 1 (rounding per line would give 0)
	if tva != 1 {
		t.Errorf("tva=%d", tva)
	}
}

func TestReminderStage(t *testing.T) {
	for days, want := range map[int]int{-5: 0, 0: 0, 1: 1, 6: 1, 7: 2, 29: 2, 30: 3, 400: 3} {
		if got := reminderStage(days); got != want {
			t.Errorf("%d days late: stage %d, want %d", days, got, want)
		}
	}
}
