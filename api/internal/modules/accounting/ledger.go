// Package accounting is the Accounting context: an append-only, hash-chained double-entry ledger.
package accounting

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/opeshealthsystems/heckate/api/internal/platform"
)

const genesis = "0000000000000000000000000000000000000000000000000000000000000000"

type Line struct {
	Account string `json:"account"`
	Debit   int64  `json:"debit"`
	Credit  int64  `json:"credit"`
	Party   string `json:"party,omitempty"`
}

type Entry struct {
	Seq       int64     `json:"seq,omitempty"`
	Journal   string    `json:"journal"`
	Date      time.Time `json:"entry_date"`
	Ref       string    `json:"ref"`
	Narration string    `json:"narration"`
	Source    string    `json:"source"`
	SourceRef string    `json:"source_ref"`
	CreatedBy string    `json:"created_by,omitempty"`
	Lines     []Line    `json:"lines"`
	PrevHash  string    `json:"prev_hash,omitempty"`
	Hash      string    `json:"hash,omitempty"`
}

// ChainHash binds an entry to its predecessor. Anything that changes the entry changes the hash,
// and every later hash with it, so tampering is detectable by recomputation.
func ChainHash(prev string, e Entry) string {
	canon := struct {
		Journal, Date, Ref, Narration, Source, SourceRef, CreatedBy string
		Lines                                                       []Line
	}{e.Journal, e.Date.Format("2006-01-02"), e.Ref, e.Narration, e.Source, e.SourceRef, e.CreatedBy, e.Lines}
	b, _ := json.Marshal(canon)
	h := sha256.Sum256(append([]byte(prev), b...))
	return hex.EncodeToString(h[:])
}

// Validate checks that an entry is well formed and balanced before it reaches the database.
func Validate(e Entry) error {
	if len(e.Lines) < 2 {
		return platform.Err(http.StatusUnprocessableEntity, "Invalid entry", "an entry needs at least two lines")
	}
	var d, c int64
	for _, l := range e.Lines {
		if l.Account == "" || l.Debit < 0 || l.Credit < 0 || (l.Debit == 0) == (l.Credit == 0) {
			return platform.Err(http.StatusUnprocessableEntity, "Invalid entry", "each line needs an account and exactly one of debit or credit, as a positive amount")
		}
		d += l.Debit
		c += l.Credit
	}
	if d != c {
		return platform.Err(http.StatusUnprocessableEntity, "Entry does not balance", fmt.Sprintf("debits %d and credits %d must be equal", d, c))
	}
	if strings.TrimSpace(e.Narration) == "" || e.Journal == "" {
		return platform.Err(http.StatusUnprocessableEntity, "Invalid entry", "journal and narration are required")
	}
	return nil
}

// Post appends an entry inside the caller's transaction. The advisory lock serialises writers so
// each entry chains to the true previous one.
func Post(ctx context.Context, s *platform.Scope, e Entry) (*Entry, error) {
	if err := Validate(e); err != nil {
		return nil, err
	}
	if _, err := s.Tx.Exec(ctx, `SELECT pg_advisory_xact_lock(727275)`); err != nil {
		return nil, err
	}
	prev := genesis
	if err := s.Tx.QueryRow(ctx, `SELECT hash FROM accounting.entries ORDER BY seq DESC LIMIT 1`).Scan(&prev); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	e.CreatedBy = s.Claims.Sub
	e.PrevHash = prev
	e.Hash = ChainHash(prev, e)
	if err := s.Tx.QueryRow(ctx, `INSERT INTO accounting.entries (journal, entry_date, ref, narration, source, source_ref, created_by, prev_hash, hash)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING seq`,
		e.Journal, e.Date, e.Ref, e.Narration, e.Source, e.SourceRef, e.CreatedBy, e.PrevHash, e.Hash).Scan(&e.Seq); err != nil {
		if strings.Contains(err.Error(), "row-level security") {
			return nil, platform.Err(http.StatusForbidden, "Forbidden", "your role cannot post to the ledger")
		}
		if strings.Contains(err.Error(), "foreign key") {
			return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid entry", "unknown journal")
		}
		return nil, err
	}
	for i, l := range e.Lines {
		var party any
		if l.Party != "" {
			party = l.Party
		}
		if _, err := s.Tx.Exec(ctx, `INSERT INTO accounting.entry_lines (entry_seq, position, account, debit, credit, party) VALUES ($1,$2,$3,$4,$5,$6)`,
			e.Seq, i+1, l.Account, l.Debit, l.Credit, party); err != nil {
			if strings.Contains(err.Error(), "foreign key") {
				return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid entry", "unknown account "+l.Account)
			}
			return nil, err
		}
	}
	return &e, nil
}
