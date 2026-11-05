package accounting

import (
	"errors"
	"github.com/jackc/pgx/v5"
	"net/http"
	"strings"
	"time"

	"github.com/opeshealthsystems/heckate/api/internal/platform"
	"github.com/opeshealthsystems/heckate/api/internal/reports"
)

func registerBankAccounts(a *platform.App) {
	a.Route("POST /v1/bank/accounts", writers, platform.Idempotent(createBankAccount))
	a.Route("GET /v1/bank/accounts", readers, listBankAccounts)
	a.Route("PATCH /v1/bank/accounts/{id}", writers, updateBankAccount)
	a.Route("GET /v1/exports/bank-accounts", readers, exportBankAccounts)
}

// BankAccount is one account the company holds at a bank, tied to its ledger account so the books and the bank
// are read side by side.
type BankAccount struct {
	ID             string     `json:"id"`
	LedgerAccount  string     `json:"ledger_account"`
	BankName       string     `json:"bank_name"`
	AccountName    string     `json:"account_name"`
	AccountNumber  string     `json:"account_number"`
	IBAN           string     `json:"iban"`
	SWIFT          string     `json:"swift"`
	Branch         string     `json:"branch"`
	Currency       string     `json:"currency"`
	Signatories    string     `json:"signatories"`
	Status         string     `json:"status"`
	OpenedOn       *time.Time `json:"opened_on"`
	ClosedOn       *time.Time `json:"closed_on"`
	Note           string     `json:"note"`
	LedgerBalance  int64      `json:"ledger_balance"`
	LastStatement  *time.Time `json:"last_statement_to"`
	LastBankClose  *int64     `json:"last_statement_closing"`
	UnmatchedLines int        `json:"unmatched_lines"`
}

const baCols = `b.id::text, b.ledger_account, b.bank_name, b.account_name, b.account_number, b.iban, b.swift, b.branch, b.currency, b.signatories, b.status, b.opened_on, b.closed_on, b.note,
	COALESCE((SELECT sum(l.debit - l.credit) FROM accounting.entry_lines l WHERE l.account = b.ledger_account),0)::bigint,
	(SELECT max(st.period_to) FROM accounting.bank_statements st WHERE st.account = b.ledger_account),
	(SELECT st.closing FROM accounting.bank_statements st WHERE st.account = b.ledger_account ORDER BY st.period_to DESC, st.created_at DESC LIMIT 1),
	(SELECT count(*) FROM accounting.bank_lines bl JOIN accounting.bank_statements st ON st.id = bl.statement_id WHERE st.account = b.ledger_account AND bl.entry_line_id IS NULL)::int`

func listBankAccounts(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	rows, err := s.Tx.Query(r.Context(), `SELECT `+baCols+` FROM accounting.bank_accounts b ORDER BY b.status, b.bank_name, b.account_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*BankAccount{}
	for rows.Next() {
		var x BankAccount
		if err := rows.Scan(&x.ID, &x.LedgerAccount, &x.BankName, &x.AccountName, &x.AccountNumber, &x.IBAN, &x.SWIFT, &x.Branch, &x.Currency, &x.Signatories, &x.Status, &x.OpenedOn, &x.ClosedOn, &x.Note,
			&x.LedgerBalance, &x.LastStatement, &x.LastBankClose, &x.UnmatchedLines); err != nil {
			return nil, err
		}
		out = append(out, &x)
	}
	return &platform.Result{Status: http.StatusOK, Body: map[string]any{"data": out}}, rows.Err()
}

func bankErr(m string) error {
	return platform.Err(http.StatusUnprocessableEntity, "Invalid bank account", m)
}

func createBankAccount(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		LedgerAccount string `json:"ledger_account"`
		BankName      string `json:"bank_name"`
		AccountName   string `json:"account_name"`
		AccountNumber string `json:"account_number"`
		IBAN          string `json:"iban"`
		SWIFT         string `json:"swift"`
		Branch        string `json:"branch"`
		Currency      string `json:"currency"`
		Signatories   string `json:"signatories"`
		OpenedOn      string `json:"opened_on"`
		Note          string `json:"note"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	if in.LedgerAccount == "" || strings.TrimSpace(in.BankName) == "" || strings.TrimSpace(in.AccountName) == "" || strings.TrimSpace(in.AccountNumber) == "" {
		return nil, bankErr("ledger_account, bank_name, account_name and account_number are required")
	}
	if in.Currency == "" {
		in.Currency = "XAF"
	}
	var opened any
	if in.OpenedOn != "" {
		d, err := time.Parse("2006-01-02", in.OpenedOn)
		if err != nil {
			return nil, bankErr("opened_on must be YYYY-MM-DD")
		}
		opened = d
	}
	var id string
	err := s.Tx.QueryRow(r.Context(), `INSERT INTO accounting.bank_accounts (ledger_account, bank_name, account_name, account_number, iban, swift, branch, currency, signatories, opened_on, note, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) RETURNING id::text`,
		in.LedgerAccount, strings.TrimSpace(in.BankName), strings.TrimSpace(in.AccountName), strings.TrimSpace(in.AccountNumber), in.IBAN, in.SWIFT, in.Branch, in.Currency, in.Signatories, opened, in.Note, s.Claims.Sub).Scan(&id)
	if err != nil {
		switch {
		case strings.Contains(err.Error(), "bank_accounts_ledger_account_key"):
			return nil, platform.Err(http.StatusConflict, "Exists", "that ledger account already has a bank account")
		case strings.Contains(err.Error(), "foreign key"):
			return nil, bankErr("unknown ledger account")
		}
		return nil, err
	}
	if err := s.Audit(r.Context(), "create", "bank_account", id); err != nil {
		return nil, err
	}
	return oneBankAccount(r, s, id, http.StatusCreated)
}

func oneBankAccount(r *http.Request, s *platform.Scope, id string, status int) (*platform.Result, error) {
	var x BankAccount
	if err := s.Tx.QueryRow(r.Context(), `SELECT `+baCols+` FROM accounting.bank_accounts b WHERE b.id::text=$1`, id).Scan(&x.ID, &x.LedgerAccount, &x.BankName, &x.AccountName, &x.AccountNumber, &x.IBAN, &x.SWIFT, &x.Branch, &x.Currency, &x.Signatories, &x.Status, &x.OpenedOn, &x.ClosedOn, &x.Note,
		&x.LedgerBalance, &x.LastStatement, &x.LastBankClose, &x.UnmatchedLines); err != nil {
		if isNotFound(err) {
			return nil, platform.Err(http.StatusNotFound, "Not found", "bank account not found")
		}
		return nil, err
	}
	return &platform.Result{Status: status, Body: x}, nil
}

func updateBankAccount(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		Signatories *string `json:"signatories"`
		Note        *string `json:"note"`
		Branch      *string `json:"branch"`
		Close       bool    `json:"close"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	tag, err := s.Tx.Exec(r.Context(), `UPDATE accounting.bank_accounts SET signatories=COALESCE($2,signatories), note=COALESCE($3,note), branch=COALESCE($4,branch),
		status=CASE WHEN $5 THEN 'closed' ELSE status END, closed_on=CASE WHEN $5 THEN COALESCE(closed_on, current_date) ELSE closed_on END WHERE id::text=$1`,
		r.PathValue("id"), in.Signatories, in.Note, in.Branch, in.Close)
	if err != nil {
		if isNotFound(err) {
			return nil, platform.Err(http.StatusNotFound, "Not found", "bank account not found")
		}
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, platform.Err(http.StatusNotFound, "Not found", "bank account not found")
	}
	if err := s.Audit(r.Context(), "update", "bank_account", r.PathValue("id")); err != nil {
		return nil, err
	}
	return oneBankAccount(r, s, r.PathValue("id"), http.StatusOK)
}

func exportBankAccounts(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	res, err := listBankAccounts(r, s)
	if err != nil {
		return nil, err
	}
	data, err := reports.Rows(res, "data")
	if err != nil {
		return nil, err
	}
	return reports.CSV("bank-accounts", []reports.Col{reports.C("Bank", "bank_name"), reports.C("Account name", "account_name"), reports.C("Account number", "account_number"), reports.C("IBAN", "iban"), reports.C("SWIFT", "swift"),
		reports.C("Branch", "branch"), reports.C("Currency", "currency"), reports.C("Ledger account", "ledger_account"), reports.C("Ledger balance", "ledger_balance"), reports.C("Last statement to", "last_statement_to"),
		reports.C("Last statement closing", "last_statement_closing"), reports.C("Unmatched bank lines", "unmatched_lines"), reports.C("Status", "status"), reports.C("Signatories", "signatories")}, data)
}

func isNotFound(err error) bool {
	return errors.Is(err, pgx.ErrNoRows) || (err != nil && strings.Contains(err.Error(), "invalid input syntax"))
}
