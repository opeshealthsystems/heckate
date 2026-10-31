package reports

import (
	"strings"
	"testing"

	"github.com/opeshealthsystems/heckate/api/internal/platform"
)

func TestSafeNeutralisesFormulas(t *testing.T) {
	for in, want := range map[string]string{"=1+1": "'=1+1", "+237": "'+237", "-5": "'-5", "@SUM(A1)": "'@SUM(A1)", "\tcmd": "'\tcmd", "normal": "normal", "": ""} {
		if got := Safe(in); got != want {
			t.Errorf("Safe(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCSVIsExcelFriendlyAndSafe(t *testing.T) {
	res, err := CSV("test", []Col{{"Ref", "ref"}, {"Amount", "amount"}, {"Date", "at"}, {"Paid", "paid"}, {"Note", "note"}},
		[]map[string]any{{"ref": "HS-INV-2026-000001", "amount": float64(1250000), "at": "2026-10-19T08:00:00Z", "paid": true, "note": `=HYPERLINK("http://evil")`}})
	if err != nil {
		t.Fatal(err)
	}
	s := string(res.Bytes)
	if !strings.HasPrefix(s, "\xEF\xBB\xBFRef,Amount,Date,Paid,Note\n") {
		t.Errorf("byte-order mark and heading expected: %q", s)
	}
	if !strings.Contains(s, "HS-INV-2026-000001,1250000,2026-10-19,yes,\"'=HYPERLINK(") {
		t.Errorf("whole-number amounts, plain dates and a neutralised formula expected: %q", s)
	}
	if res.ContentType != "text/csv; charset=utf-8" || !strings.Contains(res.Header["Content-Disposition"], "test-") {
		t.Errorf("download headers: %v %s", res.Header, res.ContentType)
	}
}

func TestRowsReadsAnArrayFromAResultBody(t *testing.T) {
	type r struct {
		A int `json:"a"`
	}
	rows, err := Rows(&platform.Result{Body: map[string]any{"rows": []r{{1}, {2}}}}, "rows")
	if err != nil || len(rows) != 2 || rows[1]["a"] != float64(2) {
		t.Errorf("%v %v", rows, err)
	}
}
