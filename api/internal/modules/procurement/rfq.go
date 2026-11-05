package procurement

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/opeshealthsystems/heckate/api/internal/modules/documents"
	"github.com/opeshealthsystems/heckate/api/internal/platform"
)

type rfqLine struct {
	Position    int     `json:"position"`
	Description string  `json:"description"`
	Quantity    float64 `json:"quantity"`
}

type RFQ struct {
	ID        string     `json:"id"`
	Ref       *string    `json:"ref"`
	Title     string     `json:"title"`
	Status    string     `json:"status"`
	ClosesAt  *time.Time `json:"closes_at"`
	CreatedAt time.Time  `json:"created_at"`
	Lines     []rfqLine  `json:"lines,omitempty"`
	Invited   []string   `json:"invited,omitempty"`
	Responses []response `json:"responses,omitempty"`
}

type response struct {
	Supplier     string          `json:"supplier"`
	SupplierName string          `json:"supplier_name"`
	UnitPrices   json.RawMessage `json:"unit_prices"`
	Total        int64           `json:"total"`
	Note         string          `json:"note"`
	SubmittedAt  time.Time       `json:"submitted_at"`
}

const rfqCols = `id, ref, title, status, closes_at, created_at`

func scanRFQ(row pgx.Row) (*RFQ, error) {
	var x RFQ
	err := row.Scan(&x.ID, &x.Ref, &x.Title, &x.Status, &x.ClosesAt, &x.CreatedAt)
	if isNF(err) {
		return nil, nf("RFQ")
	}
	return &x, err
}

func createRFQ(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		Title     string    `json:"title"`
		ClosesAt  string    `json:"closes_at"`
		Lines     []rfqLine `json:"lines"`
		Suppliers []string  `json:"suppliers"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.Title) == "" || len(in.Lines) == 0 || len(in.Lines) > 100 || len(in.Suppliers) == 0 {
		return nil, bad("title, at least one line and at least one supplier are required")
	}
	for _, l := range in.Lines {
		if strings.TrimSpace(l.Description) == "" || l.Quantity <= 0 {
			return nil, bad("each line needs a description and a positive quantity")
		}
	}
	var closes any
	if in.ClosesAt != "" {
		t, err := time.Parse(time.RFC3339, in.ClosesAt)
		if err != nil || t.Before(time.Now()) {
			return nil, bad("closes_at must be a future RFC 3339 timestamp")
		}
		closes = t
	}
	ctx := r.Context()
	x, err := scanRFQ(s.Tx.QueryRow(ctx, `INSERT INTO procurement.rfqs (title, closes_at, created_by) VALUES ($1,$2,$3) RETURNING `+rfqCols, strings.TrimSpace(in.Title), closes, s.Claims.Sub))
	if err != nil {
		return nil, err
	}
	for i, l := range in.Lines {
		if _, err := s.Tx.Exec(ctx, `INSERT INTO procurement.rfq_lines (rfq_id, position, description, quantity) VALUES ($1,$2,$3,$4)`, x.ID, i+1, strings.TrimSpace(l.Description), l.Quantity); err != nil {
			return nil, err
		}
	}
	for _, sup := range in.Suppliers {
		if _, err := s.Tx.Exec(ctx, `INSERT INTO procurement.rfq_invites (rfq_id, supplier) VALUES ($1,$2) ON CONFLICT DO NOTHING`, x.ID, sup); err != nil {
			if strings.Contains(err.Error(), "foreign key") || strings.Contains(err.Error(), "invalid input syntax") {
				return nil, bad("unknown supplier " + sup)
			}
			return nil, err
		}
	}
	if err := s.Audit(ctx, "create", "rfq", x.ID); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusCreated, Body: x}, nil
}

func listRFQs(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	rows, err := s.Tx.Query(r.Context(), `SELECT `+rfqCols+` FROM procurement.rfqs ORDER BY created_at DESC LIMIT 200`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*RFQ{}
	for rows.Next() {
		x, err := scanRFQ(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return &platform.Result{Status: http.StatusOK, Body: map[string]any{"data": out}}, rows.Err()
}

func getRFQ(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	ctx := r.Context()
	x, err := scanRFQ(s.Tx.QueryRow(ctx, `SELECT `+rfqCols+` FROM procurement.rfqs WHERE id::text=$1`, r.PathValue("id")))
	if err != nil {
		return nil, err
	}
	rows, err := s.Tx.Query(ctx, `SELECT position, description, quantity::float8 FROM procurement.rfq_lines WHERE rfq_id=$1 ORDER BY position`, x.ID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var l rfqLine
		if err := rows.Scan(&l.Position, &l.Description, &l.Quantity); err != nil {
			rows.Close()
			return nil, err
		}
		x.Lines = append(x.Lines, l)
	}
	rows.Close()
	// Competing bids are visible to buyers only; RLS limits a supplier to their own response.
	rows, err = s.Tx.Query(ctx, `SELECT p.supplier, su.legal_name, p.unit_prices, p.total, p.note, p.submitted_at
		FROM procurement.rfq_responses p JOIN procurement.suppliers su ON su.tenant_id = p.supplier WHERE p.rfq_id=$1 ORDER BY p.total`, x.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var p response
		if err := rows.Scan(&p.Supplier, &p.SupplierName, &p.UnitPrices, &p.Total, &p.Note, &p.SubmittedAt); err != nil {
			return nil, err
		}
		x.Responses = append(x.Responses, p)
	}
	return &platform.Result{Status: http.StatusOK, Body: x}, rows.Err()
}

func lockRFQ(r *http.Request, s *platform.Scope) (*RFQ, error) {
	return scanRFQ(s.Tx.QueryRow(r.Context(), `SELECT `+rfqCols+` FROM procurement.rfqs WHERE id::text=$1 FOR UPDATE`, r.PathValue("id")))
}

// readRFQ is for callers without update rights (suppliers): FOR UPDATE would need an UPDATE policy.
func readRFQ(r *http.Request, s *platform.Scope) (*RFQ, error) {
	return scanRFQ(s.Tx.QueryRow(r.Context(), `SELECT `+rfqCols+` FROM procurement.rfqs WHERE id::text=$1`, r.PathValue("id")))
}

func openRFQ(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	ctx := r.Context()
	x, err := lockRFQ(r, s)
	if err != nil {
		return nil, err
	}
	if x.Status != "draft" {
		return nil, platform.Err(http.StatusConflict, "Cannot open", "only a draft RFQ can be opened")
	}
	doc, err := documents.Issue(ctx, s, "RFQ", "Request for quotation: "+x.Title, "", "")
	if err != nil {
		return nil, err
	}
	x, err = scanRFQ(s.Tx.QueryRow(ctx, `UPDATE procurement.rfqs SET status='open', ref=$2 WHERE id=$1 RETURNING `+rfqCols, x.ID, doc.Ref))
	if err != nil {
		return nil, err
	}
	if err := s.Emit(ctx, "rfq.opened", map[string]any{"id": x.ID, "ref": doc.Ref}); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: x}, nil
}

func respondRFQ(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		UnitPrices []struct {
			Position  int   `json:"position"`
			UnitPrice int64 `json:"unit_price"`
		} `json:"unit_prices"`
		Note string `json:"note"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	ctx := r.Context()
	x, err := readRFQ(r, s) // RLS: a supplier only finds RFQs they are invited to and that are open
	if err != nil {
		return nil, err
	}
	if x.Status != "open" || (x.ClosesAt != nil && time.Now().After(*x.ClosesAt)) {
		return nil, platform.Err(http.StatusConflict, "Closed", "this RFQ is no longer accepting responses")
	}
	rows, err := s.Tx.Query(ctx, `SELECT position, quantity::float8 FROM procurement.rfq_lines WHERE rfq_id=$1`, x.ID)
	if err != nil {
		return nil, err
	}
	qty := map[int]float64{}
	for rows.Next() {
		var p int
		var q float64
		if err := rows.Scan(&p, &q); err != nil {
			rows.Close()
			return nil, err
		}
		qty[p] = q
	}
	rows.Close()
	if len(in.UnitPrices) != len(qty) {
		return nil, bad("a price is required for every line")
	}
	var total int64
	seen := map[int]bool{}
	for _, p := range in.UnitPrices {
		q, ok := qty[p.Position]
		if !ok || seen[p.Position] || p.UnitPrice < 0 {
			return nil, bad("prices must match the RFQ lines, once each, and cannot be negative")
		}
		seen[p.Position] = true
		total += (int64(q*100)*p.UnitPrice + 50) / 100
	}
	prices, _ := json.Marshal(in.UnitPrices)
	if _, err := s.Tx.Exec(ctx, `INSERT INTO procurement.rfq_responses (rfq_id, supplier, unit_prices, total, note) VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (rfq_id, supplier) DO UPDATE SET unit_prices=EXCLUDED.unit_prices, total=EXCLUDED.total, note=EXCLUDED.note, submitted_at=now()`,
		x.ID, s.Claims.TenantID, prices, total, in.Note); err != nil {
		return nil, err
	}
	if err := s.Emit(ctx, "rfq.responded", map[string]any{"id": x.ID, "supplier": s.Claims.TenantID, "total": total}); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusCreated, Body: map[string]any{"rfq_id": x.ID, "total": total}}, nil
}

func closeRFQ(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	x, err := lockRFQ(r, s)
	if err != nil {
		return nil, err
	}
	if x.Status != "open" {
		return nil, platform.Err(http.StatusConflict, "Cannot close", "only an open RFQ can be closed")
	}
	x, err = scanRFQ(s.Tx.QueryRow(r.Context(), `UPDATE procurement.rfqs SET status='closed' WHERE id=$1 RETURNING `+rfqCols, x.ID))
	if err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: x}, nil
}

// Awarding turns the winning response into a draft purchase order.
func awardRFQ(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		Supplier string `json:"supplier"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	ctx := r.Context()
	x, err := lockRFQ(r, s)
	if err != nil {
		return nil, err
	}
	if x.Status != "closed" {
		return nil, platform.Err(http.StatusConflict, "Cannot award", "close the RFQ before awarding it")
	}
	var prices json.RawMessage
	if err := s.Tx.QueryRow(ctx, `SELECT unit_prices FROM procurement.rfq_responses WHERE rfq_id=$1 AND supplier::text=$2`, x.ID, in.Supplier).Scan(&prices); err != nil {
		if isNF(err) {
			return nil, bad("that supplier did not respond to this RFQ")
		}
		return nil, err
	}
	var unit []struct {
		Position  int   `json:"position"`
		UnitPrice int64 `json:"unit_price"`
	}
	if err := json.Unmarshal(prices, &unit); err != nil {
		return nil, err
	}
	var po string
	if err := s.Tx.QueryRow(ctx, `INSERT INTO procurement.purchase_orders (supplier, rfq_id, title, created_by) VALUES ($1,$2,$3,$4) RETURNING id`, in.Supplier, x.ID, x.Title, s.Claims.Sub).Scan(&po); err != nil {
		return nil, err
	}
	for _, u := range unit {
		if _, err := s.Tx.Exec(ctx, `INSERT INTO procurement.po_lines (po_id, position, description, quantity, unit_price)
			SELECT $1, position, description, quantity, $3 FROM procurement.rfq_lines WHERE rfq_id=$2 AND position=$4`, po, x.ID, u.UnitPrice, u.Position); err != nil {
			return nil, err
		}
	}
	if err := retotalPO(ctx, s, po); err != nil {
		return nil, err
	}
	if _, err := s.Tx.Exec(ctx, `UPDATE procurement.rfqs SET status='awarded' WHERE id=$1`, x.ID); err != nil {
		return nil, err
	}
	if err := s.Audit(ctx, "award", "rfq", x.ID); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusCreated, Body: map[string]any{"rfq_id": x.ID, "purchase_order_id": po}}, nil
}
