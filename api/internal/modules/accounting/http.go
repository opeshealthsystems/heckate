package accounting

import (
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/opeshealthsystems/heckate/api/internal/platform"
)

var readers = []string{"ceo", "manager", "finance", "auditor"}
var writers = []string{"ceo", "manager", "finance"}

func Register(a *platform.App) {
	a.Route("GET /v1/accounting/accounts", readers, accounts)
	a.Route("POST /v1/accounting/accounts", writers, platform.Idempotent(addAccount))
	a.Route("GET /v1/accounting/entries", readers, entries)
	a.Route("POST /v1/accounting/entries", writers, platform.Idempotent(manualEntry))
	a.Route("GET /v1/accounting/trial-balance", readers, trialBalance)
	a.Route("GET /v1/accounting/ledger/{account}", readers, ledger)
	a.Route("GET /v1/accounting/verify", readers, verify)
}

var accountRe = regexp.MustCompile(`^[1-9][0-9]{0,7}$`)

func accounts(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	rows, err := s.Tx.Query(r.Context(), `SELECT code, name, class, note FROM accounting.accounts ORDER BY code`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type acc struct {
		Code  string `json:"code"`
		Name  string `json:"name"`
		Class int    `json:"class"`
		Note  string `json:"note"`
	}
	out := []acc{}
	for rows.Next() {
		var x acc
		if err := rows.Scan(&x.Code, &x.Name, &x.Class, &x.Note); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return &platform.Result{Status: http.StatusOK, Body: map[string]any{"data": out}}, rows.Err()
}

func addAccount(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		Code string `json:"code"`
		Name string `json:"name"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	if !accountRe.MatchString(in.Code) || strings.TrimSpace(in.Name) == "" {
		return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid account", "code must be 1-8 digits starting 1-9, and a name is required")
	}
	if _, err := s.Tx.Exec(r.Context(), `INSERT INTO accounting.accounts (code, name, class) VALUES ($1,$2,$3)`, in.Code, strings.TrimSpace(in.Name), int(in.Code[0]-'0')); err != nil {
		if strings.Contains(err.Error(), "duplicate") {
			return nil, platform.Err(http.StatusConflict, "Exists", "account already exists")
		}
		return nil, err
	}
	if err := s.Audit(r.Context(), "create", "account", in.Code); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusCreated, Body: map[string]string{"code": in.Code, "name": in.Name}}, nil
}

func manualEntry(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		Journal   string `json:"journal"`
		Date      string `json:"entry_date"`
		Narration string `json:"narration"`
		Lines     []Line `json:"lines"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	d, err := time.Parse("2006-01-02", in.Date)
	if err != nil {
		return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid entry", "entry_date must be YYYY-MM-DD")
	}
	if d.After(time.Now().UTC().AddDate(0, 0, 1)) {
		return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid entry", "entry_date cannot be in the future")
	}
	e, err := Post(r.Context(), s, Entry{Journal: in.Journal, Date: d, Narration: in.Narration, Source: "manual", Lines: in.Lines})
	if err != nil {
		return nil, err
	}
	if err := s.Audit(r.Context(), "post", "ledger_entry", strconv.FormatInt(e.Seq, 10)); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusCreated, Body: e}, nil
}

func entries(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	rows, err := s.Tx.Query(r.Context(), `SELECT e.seq, e.journal, e.entry_date, e.ref, e.narration, e.source, e.source_ref, e.created_by, e.hash,
		l.account, l.debit, l.credit, coalesce(l.party::text,'')
		FROM accounting.entries e JOIN accounting.entry_lines l ON l.entry_seq = e.seq
		WHERE e.seq IN (SELECT seq FROM accounting.entries ORDER BY seq DESC LIMIT 100)
		ORDER BY e.seq DESC, l.position`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Entry{}
	idx := map[int64]*Entry{}
	for rows.Next() {
		var e Entry
		var l Line
		if err := rows.Scan(&e.Seq, &e.Journal, &e.Date, &e.Ref, &e.Narration, &e.Source, &e.SourceRef, &e.CreatedBy, &e.Hash, &l.Account, &l.Debit, &l.Credit, &l.Party); err != nil {
			return nil, err
		}
		if x, ok := idx[e.Seq]; ok {
			x.Lines = append(x.Lines, l)
			continue
		}
		e.Lines = []Line{l}
		idx[e.Seq] = &e
		out = append(out, &e)
	}
	return &platform.Result{Status: http.StatusOK, Body: map[string]any{"data": out}}, rows.Err()
}

type tbRow struct {
	Account string `json:"account"`
	Name    string `json:"name"`
	Debit   int64  `json:"debit"`
	Credit  int64  `json:"credit"`
	Balance int64  `json:"balance"` // debit minus credit
}

func period(r *http.Request) (any, any, error) {
	var from, to any
	for k, dst := range map[string]*any{"from": &from, "to": &to} {
		if v := r.URL.Query().Get(k); v != "" {
			d, err := time.Parse("2006-01-02", v)
			if err != nil {
				return nil, nil, platform.Err(http.StatusBadRequest, "Bad date", k+" must be YYYY-MM-DD")
			}
			*dst = d
		}
	}
	return from, to, nil
}

func trialBalance(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	from, to, err := period(r)
	if err != nil {
		return nil, err
	}
	rows, err := s.Tx.Query(r.Context(), `SELECT a.code, a.name, coalesce(sum(l.debit),0), coalesce(sum(l.credit),0)
		FROM accounting.accounts a
		LEFT JOIN (accounting.entry_lines l JOIN accounting.entries e ON e.seq = l.entry_seq
		           AND ($1::date IS NULL OR e.entry_date >= $1) AND ($2::date IS NULL OR e.entry_date <= $2)) ON l.account = a.code
		GROUP BY a.code, a.name ORDER BY a.code`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []tbRow{}
	var td, tc int64
	for rows.Next() {
		var x tbRow
		if err := rows.Scan(&x.Account, &x.Name, &x.Debit, &x.Credit); err != nil {
			return nil, err
		}
		x.Balance = x.Debit - x.Credit
		if x.Debit != 0 || x.Credit != 0 {
			out = append(out, x)
			td += x.Debit
			tc += x.Credit
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Account < out[j].Account })
	return &platform.Result{Status: http.StatusOK, Body: map[string]any{"rows": out, "total_debit": td, "total_credit": tc, "balanced": td == tc}}, rows.Err()
}

func ledger(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	rows, err := s.Tx.Query(r.Context(), `SELECT e.seq, e.entry_date, e.journal, e.ref, e.narration, l.debit, l.credit, coalesce(l.party::text,'')
		FROM accounting.entry_lines l JOIN accounting.entries e ON e.seq = l.entry_seq WHERE l.account = $1 ORDER BY e.seq`, r.PathValue("account"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type row struct {
		Seq       int64     `json:"seq"`
		Date      time.Time `json:"entry_date"`
		Journal   string    `json:"journal"`
		Ref       string    `json:"ref"`
		Narration string    `json:"narration"`
		Debit     int64     `json:"debit"`
		Credit    int64     `json:"credit"`
		Party     string    `json:"party,omitempty"`
		Balance   int64     `json:"balance"`
	}
	out := []row{}
	var bal int64
	for rows.Next() {
		var x row
		if err := rows.Scan(&x.Seq, &x.Date, &x.Journal, &x.Ref, &x.Narration, &x.Debit, &x.Credit, &x.Party); err != nil {
			return nil, err
		}
		bal += x.Debit - x.Credit
		x.Balance = bal
		out = append(out, x)
	}
	return &platform.Result{Status: http.StatusOK, Body: map[string]any{"account": r.PathValue("account"), "rows": out, "balance": bal}}, rows.Err()
}

// verify recomputes the whole hash chain and reports the first entry that does not match.
func verify(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	rows, err := s.Tx.Query(r.Context(), `SELECT e.seq, e.journal, e.entry_date, e.ref, e.narration, e.source, e.source_ref, e.created_by, e.prev_hash, e.hash,
		l.account, l.debit, l.credit, coalesce(l.party::text,'')
		FROM accounting.entries e JOIN accounting.entry_lines l ON l.entry_seq = e.seq ORDER BY e.seq, l.position`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var cur *Entry
	prev := genesis
	count := 0
	var brokenAt int64
	check := func() {
		if cur == nil || brokenAt != 0 {
			return
		}
		if cur.PrevHash != prev || ChainHash(prev, *cur) != cur.Hash {
			brokenAt = cur.Seq
			return
		}
		prev = cur.Hash
		count++
	}
	for rows.Next() {
		var e Entry
		var l Line
		if err := rows.Scan(&e.Seq, &e.Journal, &e.Date, &e.Ref, &e.Narration, &e.Source, &e.SourceRef, &e.CreatedBy, &e.PrevHash, &e.Hash, &l.Account, &l.Debit, &l.Credit, &l.Party); err != nil {
			return nil, err
		}
		if cur != nil && cur.Seq == e.Seq {
			cur.Lines = append(cur.Lines, l)
			continue
		}
		check()
		e.Lines = []Line{l}
		cur = &e
	}
	check()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	body := map[string]any{"entries": count, "intact": brokenAt == 0}
	if brokenAt != 0 {
		body["first_broken_seq"] = brokenAt
	}
	return &platform.Result{Status: http.StatusOK, Body: body}, nil
}
