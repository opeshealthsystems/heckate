package accounting

import (
	"encoding/csv"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/opeshealthsystems/heckate/api/internal/platform"
)

func registerBank(a *platform.App) {
	a.Route("POST /v1/bank/statements", writers, platform.Idempotent(importStatement))
	a.Route("GET /v1/bank/statements", readers, listStatements)
	a.Route("GET /v1/bank/statements/{id}", readers, getStatement)
	a.Route("POST /v1/bank/statements/{id}/auto-match", writers, platform.Idempotent(autoMatch))
	a.Route("POST /v1/bank/lines/{id}/match", writers, platform.Idempotent(matchLine))
	a.Route("DELETE /v1/bank/lines/{id}/match", writers, unmatchLine)
}

type BankLine struct {
	ID          string  `json:"id"`
	Position    int     `json:"position"`
	Date        string  `json:"line_date"`
	Description string  `json:"description"`
	Reference   string  `json:"reference"`
	Amount      int64   `json:"amount"`
	EntryLineID *int64  `json:"entry_line_id"`
	MatchedBy   *string `json:"matched_by"`
}

type Statement struct {
	ID         string `json:"id"`
	Account    string `json:"account"`
	PeriodFrom string `json:"period_from"`
	PeriodTo   string `json:"period_to"`
	Opening    int64  `json:"opening"`
	Closing    int64  `json:"closing"`
	ImportedBy string `json:"imported_by"`
	Lines      int    `json:"lines"`
	Matched    int    `json:"matched"`
}

// ParseStatementCSV reads "date,description,amount[,reference]" rows (a header row is optional).
// Amounts are whole XAF, signed from the bank's side: positive is money in. Dates are YYYY-MM-DD or DD/MM/YYYY.
func ParseStatementCSV(text string) ([]struct {
	Date                   time.Time
	Description, Reference string
	Amount                 int64
}, error) {
	type row = struct {
		Date                   time.Time
		Description, Reference string
		Amount                 int64
	}
	rd := csv.NewReader(strings.NewReader(text))
	rd.FieldsPerRecord = -1
	rd.TrimLeadingSpace = true
	var out []row
	for n := 1; ; n++ {
		rec, err := rd.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, errors.New("not a valid CSV: " + err.Error())
		}
		if len(rec) < 3 {
			if len(out) == 0 && n == 1 {
				continue
			}
			return nil, errors.New("line " + strconv.Itoa(n) + ": need date, description and amount")
		}
		d, derr := time.Parse("2006-01-02", strings.TrimSpace(rec[0]))
		if derr != nil {
			d, derr = time.Parse("02/01/2006", strings.TrimSpace(rec[0]))
		}
		if derr != nil {
			if n == 1 { // header
				continue
			}
			return nil, errors.New("line " + strconv.Itoa(n) + ": date must be YYYY-MM-DD or DD/MM/YYYY")
		}
		amt, aerr := strconv.ParseInt(strings.NewReplacer(" ", "", " ", "", ",", "").Replace(rec[2]), 10, 64)
		if aerr != nil || amt == 0 {
			return nil, errors.New("line " + strconv.Itoa(n) + ": amount must be a non-zero whole number of XAF")
		}
		x := row{Date: d, Description: strings.TrimSpace(rec[1]), Amount: amt}
		if len(rec) > 3 {
			x.Reference = strings.TrimSpace(rec[3])
		}
		if x.Description == "" {
			return nil, errors.New("line " + strconv.Itoa(n) + ": description is empty")
		}
		out = append(out, x)
	}
	if len(out) == 0 {
		return nil, errors.New("the statement has no lines")
	}
	return out, nil
}

func importStatement(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		Account string `json:"account"`
		Opening int64  `json:"opening"`
		Closing int64  `json:"closing"`
		CSV     string `json:"csv"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	bad := func(m string) error { return platform.Err(http.StatusUnprocessableEntity, "Invalid statement", m) }
	var class int
	if err := s.Tx.QueryRow(r.Context(), `SELECT class FROM accounting.accounts WHERE code=$1`, in.Account).Scan(&class); err != nil || class != 5 {
		return nil, bad("account must be a treasury account (class 5) from the chart of accounts")
	}
	rows, err := ParseStatementCSV(in.CSV)
	if err != nil {
		return nil, bad(err.Error())
	}
	from, to := rows[0].Date, rows[0].Date
	var sum int64
	for _, x := range rows {
		if x.Date.Before(from) {
			from = x.Date
		}
		if x.Date.After(to) {
			to = x.Date
		}
		sum += x.Amount
	}
	if in.Opening+sum != in.Closing {
		return nil, bad("opening balance plus the lines does not give the closing balance; check the file")
	}
	var id string
	if err := s.Tx.QueryRow(r.Context(), `INSERT INTO accounting.bank_statements (account, period_from, period_to, opening, closing, imported_by)
		VALUES ($1,$2,$3,$4,$5,$6) RETURNING id::text`, in.Account, from, to, in.Opening, in.Closing, s.Claims.Sub).Scan(&id); err != nil {
		if strings.Contains(err.Error(), "row-level security") {
			return nil, platform.Err(http.StatusForbidden, "Forbidden", "your role cannot import statements")
		}
		return nil, err
	}
	for i, x := range rows {
		if _, err := s.Tx.Exec(r.Context(), `INSERT INTO accounting.bank_lines (statement_id, position, line_date, description, reference, amount) VALUES ($1,$2,$3,$4,$5,$6)`,
			id, i+1, x.Date, x.Description, x.Reference, x.Amount); err != nil {
			return nil, err
		}
	}
	if err := s.Audit(r.Context(), "import", "bank_statement", id); err != nil {
		return nil, err
	}
	return getStatement2(r, s, id, http.StatusCreated)
}

func listStatements(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	rows, err := s.Tx.Query(r.Context(), `SELECT st.id::text, st.account, st.period_from, st.period_to, st.opening, st.closing, st.imported_by,
		count(l.id), count(l.entry_line_id) FROM accounting.bank_statements st LEFT JOIN accounting.bank_lines l ON l.statement_id = st.id
		GROUP BY st.id ORDER BY st.period_to DESC, st.created_at DESC LIMIT 100`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Statement{}
	for rows.Next() {
		var x Statement
		var f, t time.Time
		if err := rows.Scan(&x.ID, &x.Account, &f, &t, &x.Opening, &x.Closing, &x.ImportedBy, &x.Lines, &x.Matched); err != nil {
			return nil, err
		}
		x.PeriodFrom, x.PeriodTo = f.Format("2006-01-02"), t.Format("2006-01-02")
		out = append(out, x)
	}
	return &platform.Result{Status: http.StatusOK, Body: map[string]any{"data": out}}, rows.Err()
}

func getStatement(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	return getStatement2(r, s, r.PathValue("id"), http.StatusOK)
}

// Candidate is an unmatched ledger line that could be the other side of a bank line.
type Candidate struct {
	EntryLineID int64  `json:"entry_line_id"`
	Seq         int64  `json:"seq"`
	Date        string `json:"entry_date"`
	Ref         string `json:"ref"`
	Narration   string `json:"narration"`
	Amount      int64  `json:"amount"` // debit minus credit
}

// LineView is a bank line with the ledger lines it could be matched to.
type LineView struct {
	BankLine
	Suggestions []Candidate `json:"suggestions"`
}

type stmtView struct {
	Statement       Statement
	Lines           []LineView
	UnmatchedLedger []Candidate
	Summary         map[string]any
}

func getStatement2(r *http.Request, s *platform.Scope, id string, status int) (*platform.Result, error) {
	v, err := loadStatement(r, s, id)
	if err != nil {
		return nil, err
	}
	return &platform.Result{Status: status, Body: v.body()}, nil
}

func (v *stmtView) body() map[string]any {
	return map[string]any{"statement": v.Statement, "lines": v.Lines, "unmatched_ledger": v.UnmatchedLedger, "summary": v.Summary}
}

func loadStatement(r *http.Request, s *platform.Scope, id string) (*stmtView, error) {
	ctx := r.Context()
	var st Statement
	var f, t time.Time
	err := s.Tx.QueryRow(ctx, `SELECT id::text, account, period_from, period_to, opening, closing, imported_by FROM accounting.bank_statements WHERE id=$1`, id).
		Scan(&st.ID, &st.Account, &f, &t, &st.Opening, &st.Closing, &st.ImportedBy)
	if errors.Is(err, pgx.ErrNoRows) || (err != nil && strings.Contains(err.Error(), "invalid input syntax")) {
		return nil, platform.Err(http.StatusNotFound, "Not found", "bank statement not found")
	}
	if err != nil {
		return nil, err
	}
	st.PeriodFrom, st.PeriodTo = f.Format("2006-01-02"), t.Format("2006-01-02")

	rows, err := s.Tx.Query(ctx, `SELECT id::text, position, line_date, description, reference, amount, entry_line_id, matched_by FROM accounting.bank_lines WHERE statement_id=$1 ORDER BY position`, id)
	if err != nil {
		return nil, err
	}
	lines := []BankLine{}
	for rows.Next() {
		var l BankLine
		var d time.Time
		if err := rows.Scan(&l.ID, &l.Position, &d, &l.Description, &l.Reference, &l.Amount, &l.EntryLineID, &l.MatchedBy); err != nil {
			rows.Close()
			return nil, err
		}
		l.Date = d.Format("2006-01-02")
		lines = append(lines, l)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	st.Lines = len(lines)
	var unmatchedBank int64
	for _, l := range lines {
		if l.EntryLineID != nil {
			st.Matched++
		} else {
			unmatchedBank += l.Amount
		}
	}

	// Ledger lines on this account not yet matched to any bank line, up to the end of the statement.
	crows, err := s.Tx.Query(ctx, `SELECT l.id, e.seq, e.entry_date, e.ref, e.narration, l.debit - l.credit
		FROM accounting.entry_lines l JOIN accounting.entries e ON e.seq = l.entry_seq
		WHERE l.account = $1 AND e.entry_date <= $2 AND NOT EXISTS (SELECT 1 FROM accounting.bank_lines b WHERE b.entry_line_id = l.id)
		ORDER BY e.entry_date, e.seq`, st.Account, t)
	if err != nil {
		return nil, err
	}
	defer crows.Close()
	open := []Candidate{}
	var unmatchedLedger int64
	for crows.Next() {
		var c Candidate
		var d time.Time
		if err := crows.Scan(&c.EntryLineID, &c.Seq, &d, &c.Ref, &c.Narration, &c.Amount); err != nil {
			return nil, err
		}
		c.Date = d.Format("2006-01-02")
		open = append(open, c)
		unmatchedLedger += c.Amount
	}
	if err := crows.Err(); err != nil {
		return nil, err
	}
	var ledgerBalance int64
	if err := s.Tx.QueryRow(ctx, `SELECT COALESCE(sum(l.debit - l.credit),0) FROM accounting.entry_lines l JOIN accounting.entries e ON e.seq = l.entry_seq WHERE l.account=$1 AND e.entry_date <= $2`, st.Account, t).Scan(&ledgerBalance); err != nil {
		return nil, err
	}
	// Each bank line's possible partners: same amount, within five days.
	out := make([]LineView, 0, len(lines))
	for _, l := range lines {
		w := LineView{BankLine: l, Suggestions: []Candidate{}}
		if l.EntryLineID == nil {
			ld, _ := time.Parse("2006-01-02", l.Date)
			for _, c := range open {
				cd, _ := time.Parse("2006-01-02", c.Date)
				if c.Amount == l.Amount && cd.Sub(ld).Abs() <= 5*24*time.Hour {
					w.Suggestions = append(w.Suggestions, c)
				}
			}
		}
		out = append(out, w)
	}
	return &stmtView{Statement: st, Lines: out, UnmatchedLedger: open, Summary: map[string]any{
		"ledger_balance": ledgerBalance, "statement_closing": st.Closing, "difference": ledgerBalance - st.Closing,
		"unmatched_bank_total": unmatchedBank, "unmatched_ledger_total": unmatchedLedger,
		"reconciled": st.Matched == st.Lines && len(open) == 0 && ledgerBalance == st.Closing,
	}}, nil
}

// autoMatch pairs each bank line with a ledger line only when the choice is unambiguous: exactly one
// candidate with that amount in the window, and that candidate wanted by no other bank line.
func autoMatch(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	ctx := r.Context()
	v, err := loadStatement(r, s, r.PathValue("id"))
	if err != nil {
		return nil, err
	}
	wanted := map[int64]int{}
	for _, l := range v.Lines {
		if len(l.Suggestions) == 1 {
			wanted[l.Suggestions[0].EntryLineID]++
		}
	}
	n := 0
	for _, l := range v.Lines {
		if len(l.Suggestions) != 1 || wanted[l.Suggestions[0].EntryLineID] != 1 {
			continue
		}
		if _, err := s.Tx.Exec(ctx, `UPDATE accounting.bank_lines SET entry_line_id=$2, matched_by=$3, matched_at=now() WHERE id=$1 AND entry_line_id IS NULL`, l.ID, l.Suggestions[0].EntryLineID, s.Claims.Sub); err != nil {
			return nil, err
		}
		n++
	}
	if err := s.Audit(ctx, "auto_match", "bank_statement", r.PathValue("id")); err != nil {
		return nil, err
	}
	v, err = loadStatement(r, s, r.PathValue("id"))
	if err != nil {
		return nil, err
	}
	b := v.body()
	b["auto_matched"] = n
	return &platform.Result{Status: http.StatusOK, Body: b}, nil
}

func matchLine(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		EntryLineID int64 `json:"entry_line_id"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	tag, err := s.Tx.Exec(r.Context(), `UPDATE accounting.bank_lines SET entry_line_id=$2, matched_by=$3, matched_at=now() WHERE id=$1 AND entry_line_id IS NULL`, r.PathValue("id"), in.EntryLineID, s.Claims.Sub)
	if err != nil {
		switch m := err.Error(); {
		case strings.Contains(m, "same account and amount"):
			return nil, platform.Err(http.StatusUnprocessableEntity, "No match", "that ledger line is not on this account with this amount")
		case strings.Contains(m, "duplicate key"):
			return nil, platform.Err(http.StatusConflict, "Already matched", "that ledger line is matched to another bank line")
		case strings.Contains(m, "invalid input syntax"):
			return nil, platform.Err(http.StatusNotFound, "Not found", "bank line not found")
		}
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, platform.Err(http.StatusConflict, "Cannot match", "bank line not found or already matched")
	}
	if err := s.Audit(r.Context(), "match", "bank_line", r.PathValue("id")); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusNoContent}, nil
}

func unmatchLine(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	tag, err := s.Tx.Exec(r.Context(), `UPDATE accounting.bank_lines SET entry_line_id=NULL, matched_by=NULL, matched_at=NULL WHERE id=$1 AND entry_line_id IS NOT NULL`, r.PathValue("id"))
	if err != nil {
		if strings.Contains(err.Error(), "invalid input syntax") {
			return nil, platform.Err(http.StatusNotFound, "Not found", "bank line not found")
		}
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, platform.Err(http.StatusConflict, "Not matched", "bank line not found or not matched")
	}
	if err := s.Audit(r.Context(), "unmatch", "bank_line", r.PathValue("id")); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusNoContent}, nil
}
