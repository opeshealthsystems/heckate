package payroll

import (
	"errors"
	"fmt"
)

// Config is the rules set a payroll run is computed with. It is stored as JSON, validated by a
// second person before use, and copied into every run so a past run can always be re-explained.
type Config struct {
	AbatementBP       int64     `json:"abatement_bp"`       // professional-expense allowance on annual gross, in basis points
	PersonalDeduction int64     `json:"personal_deduction"` // annual deduction before tax brackets
	CACBP             int64     `json:"cac_bp"`             // council surcharge on the income tax, in basis points
	Rules             []Rule    `json:"rules"`
	Brackets          []Bracket `json:"brackets"` // annual income tax, in ascending order; the last has no upper bound
}

// Rule is one contribution. Ceiling 0 means the whole gross is the base.
type Rule struct {
	Code          string `json:"code"`
	Label         string `json:"label"`
	Party         string `json:"party"` // employee or employer
	RateBP        int64  `json:"rate_bp"`
	Ceiling       int64  `json:"ceiling"`
	TaxDeductible bool   `json:"tax_deductible"` // an employee contribution that reduces taxable pay
}

type Bracket struct {
	UpTo   *int64 `json:"upto"`
	RateBP int64  `json:"rate_bp"`
}

type Line struct {
	Code   string `json:"code"`
	Label  string `json:"label"`
	Base   int64  `json:"base"`
	RateBP int64  `json:"rate_bp"`
	Amount int64  `json:"amount"`
}

type Slip struct {
	Gross        int64  `json:"gross"`
	Deductions   []Line `json:"deductions"`
	IRPP         int64  `json:"irpp"`
	CAC          int64  `json:"cac"`
	Net          int64  `json:"net"`
	Employer     []Line `json:"employer_lines"`
	EmployerCost int64  `json:"employer_cost"`
}

// Validate rejects a configuration that could not produce a sensible payslip.
func (c Config) Validate() error {
	if c.AbatementBP < 0 || c.AbatementBP > 10000 || c.CACBP < 0 || c.CACBP > 10000 || c.PersonalDeduction < 0 {
		return errors.New("abatement_bp and cac_bp must be 0-10000 and personal_deduction not negative")
	}
	seen := map[string]bool{}
	for _, r := range c.Rules {
		if r.Code == "" || seen[r.Code] {
			return fmt.Errorf("rule codes must be unique and not empty (%q)", r.Code)
		}
		seen[r.Code] = true
		if r.Party != "employee" && r.Party != "employer" {
			return fmt.Errorf("rule %s: party must be employee or employer", r.Code)
		}
		if r.RateBP < 0 || r.RateBP > 10000 || r.Ceiling < 0 {
			return fmt.Errorf("rule %s: rate_bp must be 0-10000 and ceiling not negative", r.Code)
		}
		if r.TaxDeductible && r.Party != "employee" {
			return fmt.Errorf("rule %s: only employee contributions can be tax deductible", r.Code)
		}
	}
	if len(c.Brackets) == 0 {
		return errors.New("at least one tax bracket is required")
	}
	var prev int64
	for i, b := range c.Brackets {
		if b.RateBP < 0 || b.RateBP > 10000 {
			return fmt.Errorf("bracket %d: rate_bp must be 0-10000", i+1)
		}
		last := i == len(c.Brackets)-1
		if last != (b.UpTo == nil) {
			return errors.New("only the last bracket has no upper bound")
		}
		if b.UpTo != nil {
			if *b.UpTo <= prev {
				return errors.New("bracket upper bounds must increase")
			}
			prev = *b.UpTo
		}
	}
	return nil
}

// pct is x * bp / 10000 rounded half up (all amounts are whole XAF and never negative).
func pct(x, bp int64) int64 { return (x*bp + 5000) / 10000 }

// Compute works out one month's payslip from a monthly gross.
func Compute(gross int64, c Config) Slip {
	s := Slip{Gross: gross, Deductions: []Line{}, Employer: []Line{}}
	var empTotal, deductible, erTotal int64
	for _, r := range c.Rules {
		base := gross
		if r.Ceiling > 0 && base > r.Ceiling {
			base = r.Ceiling
		}
		l := Line{Code: r.Code, Label: r.Label, Base: base, RateBP: r.RateBP, Amount: pct(base, r.RateBP)}
		if r.Party == "employee" {
			s.Deductions = append(s.Deductions, l)
			empTotal += l.Amount
			if r.TaxDeductible {
				deductible += l.Amount
			}
		} else {
			s.Employer = append(s.Employer, l)
			erTotal += l.Amount
		}
	}
	taxable := gross*12 - pct(gross*12, c.AbatementBP) - deductible*12 - c.PersonalDeduction
	if taxable < 0 {
		taxable = 0
	}
	annual := bracketTax(taxable, c.Brackets)
	s.IRPP = (annual + 6) / 12
	s.CAC = pct(s.IRPP, c.CACBP)
	s.Net = gross - empTotal - s.IRPP - s.CAC
	s.EmployerCost = gross + erTotal
	return s
}

func bracketTax(taxable int64, brackets []Bracket) int64 {
	var tax, floor int64
	for _, b := range brackets {
		top := taxable
		if b.UpTo != nil && *b.UpTo < top {
			top = *b.UpTo
		}
		if top > floor {
			tax += pct(top-floor, b.RateBP)
		}
		if b.UpTo == nil || taxable <= *b.UpTo {
			break
		}
		floor = *b.UpTo
	}
	return tax
}

// Totals sums a payslip's employer-side and employee-side lines by who they are owed to.
func (s Slip) ContributionsOwed() (employee, employer int64) {
	for _, l := range s.Deductions {
		employee += l.Amount
	}
	for _, l := range s.Employer {
		employer += l.Amount
	}
	return
}
