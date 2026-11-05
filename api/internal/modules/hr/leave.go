package hr

import "time"

// WorkingDays counts Monday to Friday days from start to end inclusive, skipping the given holidays.
func WorkingDays(start, end time.Time, holidays map[string]bool) int {
	n := 0
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		if wd := d.Weekday(); wd == time.Saturday || wd == time.Sunday {
			continue
		}
		if holidays[d.Format("2006-01-02")] {
			continue
		}
		n++
	}
	return n
}

// Entitlement is the annual leave earned in a calendar year, pro-rated by whole months worked in the
// hire year (rounded down so the company never over-grants).
func Entitlement(annualDays int, hire time.Time, year int) int {
	switch {
	case hire.Year() > year:
		return 0
	case hire.Year() < year:
		return annualDays
	}
	months := 12 - int(hire.Month()) + 1
	if hire.Day() > 1 {
		months-- // a partial first month does not count
	}
	if months < 0 {
		months = 0
	}
	return annualDays * months / 12
}

// Overlaps reports whether two inclusive date ranges share a day.
func Overlaps(aStart, aEnd, bStart, bEnd time.Time) bool {
	return !aEnd.Before(bStart) && !bEnd.Before(aStart)
}
