package procurement

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/opeshealthsystems/heckate/api/internal/modules/documents"
	"github.com/opeshealthsystems/heckate/api/internal/modules/inventory"
	"github.com/opeshealthsystems/heckate/api/internal/platform"
)

type poLine struct {
	Position    int     `json:"position"`
	Description string  `json:"description"`
	Quantity    float64 `json:"quantity"`
	UnitPrice   int64   `json:"unit_price"`
	ItemID      *string `json:"item_id"`
	Received    float64 `json:"received_qty"`
}

type PO struct {
	ID          string    `json:"id"`
	Ref         *string   `json:"ref"`
	Supplier    string    `json:"supplier"`
	RFQID       *string   `json:"rfq_id"`
	Title       string    `json:"title"`
	Status      string    `json:"status"`
	Net         int64     `json:"net"`
	CreatedBy   string    `json:"created_by"`
	RequestedBy *string   `json:"requested_by"`
	ApprovedBy  *string   `json:"approved_by"`
	CreatedAt   time.Time `json:"created_at"`
	Lines       []poLine  `json:"lines,omitempty"`
}

const poCols = `id, ref, supplier, rfq_id, title, status, net, created_by, requested_by, approved_by, created_at`

func scanPO(row pgx.Row) (*PO, error) {
	var x PO
	err := row.Scan(&x.ID, &x.Ref, &x.Supplier, &x.RFQID, &x.Title, &x.Status, &x.Net, &x.CreatedBy, &x.RequestedBy, &x.ApprovedBy, &x.CreatedAt)
	if isNF(err) {
		return nil, nf("purchase order")
	}
	return &x, err
}

func retotalPO(ctx context.Context, s *platform.Scope, id string) error {
	_, err := s.Tx.Exec(ctx, `UPDATE procurement.purchase_orders SET net = coalesce((SELECT sum((round(quantity*100)::bigint * unit_price + 50) / 100) FROM procurement.po_lines WHERE po_id=$1),0) WHERE id=$1`, id)
	return err
}

func createPO(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		Supplier string `json:"supplier_id"`
		Title    string `json:"title"`
		Lines    []struct {
			Description string  `json:"description"`
			Quantity    float64 `json:"quantity"`
			UnitPrice   int64   `json:"unit_price"`
			ItemID      string  `json:"item_id"`
		} `json:"lines"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	if in.Supplier == "" || strings.TrimSpace(in.Title) == "" || len(in.Lines) == 0 || len(in.Lines) > 200 {
		return nil, bad("supplier_id, title and between 1 and 200 lines are required")
	}
	for _, l := range in.Lines {
		if strings.TrimSpace(l.Description) == "" || l.Quantity <= 0 || l.UnitPrice < 0 {
			return nil, bad("each line needs a description, a positive quantity and a non-negative unit price")
		}
	}
	ctx := r.Context()
	var id string
	if err := s.Tx.QueryRow(ctx, `INSERT INTO procurement.purchase_orders (supplier, title, created_by) VALUES ($1,$2,$3) RETURNING id`, in.Supplier, strings.TrimSpace(in.Title), s.Claims.Sub).Scan(&id); err != nil {
		if strings.Contains(err.Error(), "foreign key") || strings.Contains(err.Error(), "invalid input syntax") {
			return nil, bad("unknown supplier")
		}
		return nil, err
	}
	for i, l := range in.Lines {
		if _, err := s.Tx.Exec(ctx, `INSERT INTO procurement.po_lines (po_id, position, description, quantity, unit_price, item_id) VALUES ($1,$2,$3,$4,$5,NULLIF($6,'')::uuid)`,
			id, i+1, strings.TrimSpace(l.Description), l.Quantity, l.UnitPrice, l.ItemID); err != nil {
			if strings.Contains(err.Error(), "invalid input syntax") {
				return nil, bad("invalid item_id")
			}
			return nil, err
		}
	}
	if err := retotalPO(ctx, s, id); err != nil {
		return nil, err
	}
	x, err := loadPO(ctx, s, id, false)
	if err != nil {
		return nil, err
	}
	if err := s.Audit(ctx, "create", "purchase_order", id); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusCreated, Body: x}, nil
}

func loadPO(ctx context.Context, s *platform.Scope, id string, lock bool) (*PO, error) {
	q := `SELECT ` + poCols + ` FROM procurement.purchase_orders WHERE id::text=$1`
	if lock {
		q += ` FOR UPDATE`
	}
	x, err := scanPO(s.Tx.QueryRow(ctx, q, id))
	if err != nil {
		return nil, err
	}
	rows, err := s.Tx.Query(ctx, `SELECT position, description, quantity::float8, unit_price, item_id, received_qty::float8 FROM procurement.po_lines WHERE po_id=$1 ORDER BY position`, x.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var l poLine
		if err := rows.Scan(&l.Position, &l.Description, &l.Quantity, &l.UnitPrice, &l.ItemID, &l.Received); err != nil {
			return nil, err
		}
		x.Lines = append(x.Lines, l)
	}
	return x, rows.Err()
}

func listPOs(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	rows, err := s.Tx.Query(r.Context(), `SELECT `+poCols+` FROM procurement.purchase_orders ORDER BY created_at DESC LIMIT 200`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*PO{}
	for rows.Next() {
		x, err := scanPO(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return &platform.Result{Status: http.StatusOK, Body: map[string]any{"data": out}}, rows.Err()
}

func getPO(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	x, err := loadPO(r.Context(), s, r.PathValue("id"), false)
	if err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: x}, nil
}

func finalizePO(ctx context.Context, s *platform.Scope, x *PO, approver *string) (*PO, error) {
	if x.Net <= 0 {
		return nil, bad("a purchase order needs a positive total")
	}
	doc, err := documents.Issue(ctx, s, "PO", "Purchase order: "+x.Title, "", "")
	if err != nil {
		return nil, err
	}
	if _, err := s.Tx.Exec(ctx, `UPDATE procurement.purchase_orders SET status='issued', ref=$2, approved_by=COALESCE($3, approved_by) WHERE id=$1`, x.ID, doc.Ref, approver); err != nil {
		return nil, err
	}
	if err := s.Emit(ctx, "purchase_order.issued", map[string]any{"id": x.ID, "supplier": x.Supplier, "ref": doc.Ref, "net": x.Net}); err != nil {
		return nil, err
	}
	if err := s.Audit(ctx, "issue", "purchase_order", x.ID); err != nil {
		return nil, err
	}
	return loadPO(ctx, s, x.ID, false)
}

func issuePO(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	ctx := r.Context()
	x, err := loadPO(ctx, s, r.PathValue("id"), true)
	if err != nil {
		return nil, err
	}
	if x.Status != "draft" {
		return nil, platform.Err(http.StatusConflict, "Cannot issue", "only a draft purchase order can be issued")
	}
	var threshold int64
	if err := s.Tx.QueryRow(ctx, `SELECT approval_threshold FROM procurement.settings`).Scan(&threshold); err != nil {
		return nil, err
	}
	if x.Net >= threshold {
		if _, err := s.Tx.Exec(ctx, `UPDATE procurement.purchase_orders SET status='pending_approval', requested_by=$2 WHERE id=$1`, x.ID, s.Claims.Sub); err != nil {
			return nil, err
		}
		if err := s.Audit(ctx, "request_approval", "purchase_order", x.ID); err != nil {
			return nil, err
		}
		out, err := loadPO(ctx, s, x.ID, false)
		if err != nil {
			return nil, err
		}
		return &platform.Result{Status: http.StatusAccepted, Body: out}, nil
	}
	out, err := finalizePO(ctx, s, x, nil)
	if err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: out}, nil
}

func approvePO(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	ctx := r.Context()
	x, err := loadPO(ctx, s, r.PathValue("id"), true)
	if err != nil {
		return nil, err
	}
	if x.Status != "pending_approval" {
		return nil, platform.Err(http.StatusConflict, "Cannot approve", "the purchase order is not waiting for approval")
	}
	if x.CreatedBy == s.Claims.Sub || (x.RequestedBy != nil && *x.RequestedBy == s.Claims.Sub) {
		return nil, platform.Err(http.StatusForbidden, "Second approver required", "the person who prepared or requested this order cannot approve it")
	}
	sub := s.Claims.Sub
	out, err := finalizePO(ctx, s, x, &sub)
	if err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: out}, nil
}

// receivePO records goods received. Lines linked to a stock item go into the given warehouse.
func receivePO(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		Lines []struct {
			Position    int     `json:"position"`
			Qty         float64 `json:"qty"`
			WarehouseID string  `json:"warehouse_id"`
		} `json:"lines"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	if len(in.Lines) == 0 {
		return nil, bad("at least one line is required")
	}
	ctx := r.Context()
	x, err := loadPO(ctx, s, r.PathValue("id"), true)
	if err != nil {
		return nil, err
	}
	if x.Status != "issued" {
		return nil, platform.Err(http.StatusConflict, "Cannot receive", "goods can only be received against an issued purchase order")
	}
	byPos := map[int]poLine{}
	for _, l := range x.Lines {
		byPos[l.Position] = l
	}
	for _, rl := range in.Lines {
		l, ok := byPos[rl.Position]
		if !ok || rl.Qty <= 0 {
			return nil, bad(fmt.Sprintf("line %d is unknown or the quantity is not positive", rl.Position))
		}
		if l.Received+rl.Qty > l.Quantity+1e-9 {
			return nil, platform.Err(http.StatusUnprocessableEntity, "Over-receipt", fmt.Sprintf("line %d: only %.2f outstanding", rl.Position, l.Quantity-l.Received))
		}
		if l.ItemID != nil {
			if rl.WarehouseID == "" || rl.Qty != float64(int(rl.Qty)) {
				return nil, bad(fmt.Sprintf("line %d is a stock item: give a warehouse_id and a whole quantity", rl.Position))
			}
			if err := inventory.Receive(ctx, s, *l.ItemID, rl.WarehouseID, int(rl.Qty), "po:"+*x.Ref); err != nil {
				return nil, err
			}
		}
		if _, err := s.Tx.Exec(ctx, `UPDATE procurement.po_lines SET received_qty = received_qty + $3 WHERE po_id=$1 AND position=$2`, x.ID, rl.Position, rl.Qty); err != nil {
			return nil, err
		}
	}
	var open int
	if err := s.Tx.QueryRow(ctx, `SELECT count(*) FROM procurement.po_lines WHERE po_id=$1 AND received_qty < quantity`, x.ID).Scan(&open); err != nil {
		return nil, err
	}
	if open == 0 {
		if _, err := s.Tx.Exec(ctx, `UPDATE procurement.purchase_orders SET status='received' WHERE id=$1`, x.ID); err != nil {
			return nil, err
		}
		if err := s.Emit(ctx, "purchase_order.received", map[string]any{"id": x.ID}); err != nil {
			return nil, err
		}
	}
	if err := s.Audit(ctx, "receive", "purchase_order", x.ID); err != nil {
		return nil, err
	}
	out, err := loadPO(ctx, s, x.ID, false)
	if err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: out}, nil
}
