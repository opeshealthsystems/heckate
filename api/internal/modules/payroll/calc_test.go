package payroll

import "testing"

func i64(v int64) *int64 { return &v }

func cfg() Config {
	return Config{
		AbatementBP: 3000, PersonalDeduction: 500000, CACBP: 1000,
		Rules: []Rule{
			{Code: "PEN_EMP", Label: "Pension", Party: "employee", RateBP: 280, Ceiling: 750000, TaxDeductible: true},
			{Code: "FUND_EMP", Label: "Fund", Party: "employee", RateBP: 100},
			{Code: "PEN_ER", Label: "Pension", Party: "employer", RateBP: 420, Ceiling: 750000},
		},
		Brackets: []Bracket{{UpTo: i64(2000000), RateBP: 1000}, {UpTo: i64(3000000), RateBP: 1500}, {UpTo: nil, RateBP: 2500}},
	}
}

func TestComputeAddsUp(t *testing.T) {
	for _, gross := range []int64{0, 50_000, 300_000, 750_000, 1_000_000, 2_500_000, 10_000_000} {
		s := Compute(gross, cfg())
		emp, er := s.ContributionsOwed()
		if s.Net != gross-emp-s.IRPP-s.CAC {
			t.Errorf("gross %d: net %d does not equal gross minus deductions", gross, s.Net)
		}
		if s.EmployerCost != gross+er {
			t.Errorf("gross %d: employer cost %d", gross, s.EmployerCost)
		}
		if s.Net < 0 {
			t.Errorf("gross %d: negative net %d", gross, s.Net)
		}
	}
}

func TestComputeKnownFigures(t *testing.T) {
	// Gross 1,000,000: pension on the 750,000 ceiling = 21,000; fund 1% of the whole gross = 10,000.
	s := Compute(1_000_000, cfg())
	if s.Deductions[0].Amount != 21_000 || s.Deductions[1].Amount != 10_000 {
		t.Fatalf("contributions: %+v", s.Deductions)
	}
	// Taxable per year = 12,000,000 - 30% (3,600,000) - 252,000 pension - 500,000 = 7,648,000;
	// tax = 200,000 + 150,000 + 25% of 4,648,000 (1,162,000) = 1,512,000 a year = 126,000 a month.
	if s.IRPP != 126_000 || s.CAC != 12_600 {
		t.Fatalf("income tax: irpp %d cac %d", s.IRPP, s.CAC)
	}
	if s.Net != 1_000_000-31_000-126_000-12_600 {
		t.Fatalf("net %d", s.Net)
	}
	if s.Employer[0].Amount != 31_500 || s.EmployerCost != 1_031_500 {
		t.Fatalf("employer %+v cost %d", s.Employer, s.EmployerCost)
	}
}

func TestLowPayPaysNoIncomeTax(t *testing.T) {
	if s := Compute(40_000, cfg()); s.IRPP != 0 || s.CAC != 0 {
		t.Fatalf("expected no income tax, got %d / %d", s.IRPP, s.CAC)
	}
}

func TestConfigValidation(t *testing.T) {
	if err := cfg().Validate(); err != nil {
		t.Fatal(err)
	}
	bad := cfg()
	bad.Rules[0].Party = "boss"
	if bad.Validate() == nil {
		t.Error("bad party must fail")
	}
	bad = cfg()
	bad.Brackets[2].UpTo = i64(9_000_000)
	if bad.Validate() == nil {
		t.Error("last bracket must be unbounded")
	}
	bad = cfg()
	bad.Brackets[1].UpTo = i64(1_000_000)
	if bad.Validate() == nil {
		t.Error("bounds must increase")
	}
	bad = cfg()
	bad.Rules = append(bad.Rules, bad.Rules[0])
	if bad.Validate() == nil {
		t.Error("duplicate code must fail")
	}
	bad = cfg()
	bad.Rules[2].TaxDeductible = true
	if bad.Validate() == nil {
		t.Error("employer rule cannot be tax deductible")
	}
}
