package accounting

import (
	"testing"
	"time"
)

func TestValidate(t *testing.T) {
	ok := Entry{Journal: "OD", Narration: "x", Lines: []Line{{Account: "411", Debit: 100}, {Account: "706", Credit: 100}}}
	if err := Validate(ok); err != nil {
		t.Fatal(err)
	}
	cases := map[string]Entry{
		"unbalanced":    {Journal: "OD", Narration: "x", Lines: []Line{{Account: "411", Debit: 100}, {Account: "706", Credit: 99}}},
		"one line":      {Journal: "OD", Narration: "x", Lines: []Line{{Account: "411", Debit: 100}}},
		"both sides":    {Journal: "OD", Narration: "x", Lines: []Line{{Account: "411", Debit: 100, Credit: 100}, {Account: "706", Credit: 0, Debit: 0}}},
		"negative":      {Journal: "OD", Narration: "x", Lines: []Line{{Account: "411", Debit: -5}, {Account: "706", Credit: -5}}},
		"no narration":  {Journal: "OD", Lines: ok.Lines},
		"no journal":    {Narration: "x", Lines: ok.Lines},
		"empty account": {Journal: "OD", Narration: "x", Lines: []Line{{Debit: 1}, {Account: "706", Credit: 1}}},
	}
	for name, e := range cases {
		if Validate(e) == nil {
			t.Errorf("%s must be rejected", name)
		}
	}
}

func TestChainHashDetectsAnyChange(t *testing.T) {
	e := Entry{Journal: "VT", Date: time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC), Ref: "R", Narration: "n", Source: "invoice", SourceRef: "R", CreatedBy: "u",
		Lines: []Line{{Account: "411", Debit: 100}, {Account: "706", Credit: 100}}}
	base := ChainHash(genesis, e)
	if base != ChainHash(genesis, e) {
		t.Fatal("hash must be deterministic")
	}
	mut := map[string]func(Entry) Entry{
		"amount": func(x Entry) Entry {
			x.Lines = []Line{{Account: "411", Debit: 101}, {Account: "706", Credit: 101}}
			return x
		},
		"account": func(x Entry) Entry {
			x.Lines = []Line{{Account: "411", Debit: 100}, {Account: "701", Credit: 100}}
			return x
		},
		"narration": func(x Entry) Entry { x.Narration = "m"; return x },
		"date":      func(x Entry) Entry { x.Date = x.Date.AddDate(0, 0, 1); return x },
		"author":    func(x Entry) Entry { x.CreatedBy = "v"; return x },
	}
	for name, f := range mut {
		if ChainHash(genesis, f(e)) == base {
			t.Errorf("changing %s must change the hash", name)
		}
	}
	if ChainHash("other-prev", e) == base {
		t.Error("changing the previous hash must change the hash")
	}
}

func TestNonZeroLinesOnly(t *testing.T) {
	got := nz(Line{Account: "a", Debit: 1}, Line{Account: "b"}, Line{Account: "c", Credit: 1})
	if len(got) != 2 {
		t.Errorf("zero lines must be dropped, got %v", got)
	}
}

func TestSettlementAccounts(t *testing.T) {
	for m, want := range map[string]string{"cash": "571", "momo": "5211", "orange_money": "5211", "bank": "521", "card": "521"} {
		if _, a := settlementAccount(m); a != want {
			t.Errorf("%s -> %s, want %s", m, a, want)
		}
	}
}
