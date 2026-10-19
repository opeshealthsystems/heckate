// Package reports turns what the API already computes into files people keep: CSV for spreadsheets and PDF
// printouts. A CSV opens correctly in Excel (UTF-8 with a byte-order mark), amounts are plain whole numbers so
// they can be summed, and no cell can run as a formula when the file is opened (see Safe).
package reports

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/opeshealthsystems/heckate/api/internal/platform"
)

// Col is one CSV column: the heading a person reads and the JSON key it comes from.
type Col struct{ Header, Key string }

// C is a column: the heading and the JSON key.
func C(header, key string) Col { return Col{Header: header, Key: key} }

// Safe stops spreadsheet formula injection: a cell that starts with = + - @ or a control character would be run
// as a formula by Excel or Sheets, so it is prefixed with an apostrophe, which shows as plain text.
func Safe(s string) string {
	if s == "" {
		return s
	}
	switch s[0] {
	case '=', '+', '-', '@', '\t', '\r', '\n':
		return "'" + s
	}
	return s
}

func cell(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		if t, err := time.Parse(time.RFC3339Nano, x); err == nil {
			return t.UTC().Format("2006-01-02")
		}
		return Safe(x)
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		if x {
			return "yes"
		}
		return "no"
	default:
		return Safe(fmt.Sprint(x))
	}
}

// Rows converts a handler's result into rows: body[key] must be an array of objects.
func Rows(res *platform.Result, key string) ([]map[string]any, error) {
	b, err := Body(res)
	if err != nil {
		return nil, err
	}
	raw, _ := b[key].([]any)
	out := make([]map[string]any, 0, len(raw))
	for _, x := range raw {
		if m, ok := x.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out, nil
}

// Body converts a handler's result body into a generic map.
func Body(res *platform.Result) (map[string]any, error) {
	j, err := json.Marshal(res.Body)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	return m, json.Unmarshal(j, &m)
}

// CSV builds a download. The file name carries the date it was produced.
func CSV(name string, cols []Col, rows []map[string]any) (*platform.Result, error) {
	var buf bytes.Buffer
	buf.WriteString("\xEF\xBB\xBF")
	w := csv.NewWriter(&buf)
	head := make([]string, len(cols))
	for i, c := range cols {
		head[i] = c.Header
	}
	if err := w.Write(head); err != nil {
		return nil, err
	}
	for _, r := range rows {
		rec := make([]string, len(cols))
		for i, c := range cols {
			rec[i] = cell(r[c.Key])
		}
		if err := w.Write(rec); err != nil {
			return nil, err
		}
	}
	w.Flush()
	return &platform.Result{Status: http.StatusOK, Bytes: buf.Bytes(), ContentType: "text/csv; charset=utf-8", Header: download(name + "-" + time.Now().UTC().Format("2006-01-02") + ".csv")}, w.Error()
}

// PDF builds a download of a rendered report.
func PDF(name string, b []byte) *platform.Result {
	return &platform.Result{Status: http.StatusOK, Bytes: b, ContentType: "application/pdf", Header: download(name + "-" + time.Now().UTC().Format("2006-01-02") + ".pdf")}
}

func download(filename string) map[string]string {
	return map[string]string{
		"Content-Disposition":    `attachment; filename="` + strings.ReplaceAll(filename, `"`, "") + `"`,
		"Cache-Control":          "private, no-store",
		"X-Content-Type-Options": "nosniff",
	}
}
