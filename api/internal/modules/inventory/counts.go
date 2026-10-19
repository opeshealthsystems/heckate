package inventory

import (
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/opeshealthsystems/heckate/api/internal/platform"
)

type countLine struct {
	ItemID   string `json:"item_id"`
	SKU      string `json:"sku"`
	Item     string `json:"item"`
	Expected int    `json:"expected"`
	Counted  int    `json:"counted"`
	Variance int    `json:"variance"`
}

type Count struct {
	ID          string      `json:"id"`
	WarehouseID string      `json:"warehouse_id"`
	Warehouse   string      `json:"warehouse"`
	Status      string      `json:"status"`
	CountedBy   string      `json:"counted_by"`
	ApprovedBy  *string     `json:"approved_by"`
	CreatedAt   time.Time   `json:"created_at"`
	ApprovedAt  *time.Time  `json:"approved_at"`
	Lines       []countLine `json:"lines,omitempty"`
}

const countCols = `c.id::text, c.warehouse_id::text, w.name, c.status, c.counted_by, c.approved_by, c.created_at, c.approved_at`
const countFrom = ` FROM inventory.counts c JOIN inventory.warehouses w ON w.id = c.warehouse_id`

func scanCount(row pgx.Row) (*Count, error) {
	var c Count
	err := row.Scan(&c.ID, &c.WarehouseID, &c.Warehouse, &c.Status, &c.CountedBy, &c.ApprovedBy, &c.CreatedAt, &c.ApprovedAt)
	if isNF(err) {
		return nil, nf("stock count")
	}
	return &c, err
}

func loadLines(r *http.Request, s *platform.Scope, c *Count) error {
	rows, err := s.Tx.Query(r.Context(), `SELECT l.item_id::text, i.sku, i.name, l.expected, l.counted
		FROM inventory.count_lines l JOIN inventory.items i ON i.id = l.item_id WHERE l.count_id = $1 ORDER BY i.sku`, c.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var l countLine
		if err := rows.Scan(&l.ItemID, &l.SKU, &l.Item, &l.Expected, &l.Counted); err != nil {
			return err
		}
		l.Variance = l.Counted - l.Expected
		c.Lines = append(c.Lines, l)
	}
	return rows.Err()
}

// createCount records what was counted in one warehouse next to what the ledger expected at that moment.
// Nothing changes in stock until a second person approves it.
func createCount(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		WarehouseID string `json:"warehouse_id"`
		Lines       []struct {
			ItemID  string `json:"item_id"`
			Counted int    `json:"counted"`
		} `json:"lines"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	if in.WarehouseID == "" || len(in.Lines) == 0 || len(in.Lines) > 500 {
		return nil, bad("warehouse_id and between 1 and 500 lines are required")
	}
	seen := map[string]bool{}
	for _, l := range in.Lines {
		if l.ItemID == "" || l.Counted < 0 || seen[l.ItemID] {
			return nil, bad("each line needs a distinct item_id and a counted quantity of zero or more")
		}
		seen[l.ItemID] = true
	}
	ctx := r.Context()
	var id string
	if err := s.Tx.QueryRow(ctx, `INSERT INTO inventory.counts (warehouse_id, counted_by) VALUES ($1,$2) RETURNING id::text`, in.WarehouseID, s.Claims.Sub).Scan(&id); err != nil {
		if strings.Contains(err.Error(), "foreign key") || strings.Contains(err.Error(), "invalid input syntax") {
			return nil, bad("unknown warehouse")
		}
		return nil, err
	}
	for _, l := range in.Lines {
		it, err := lookupItem(ctx, s, l.ItemID)
		if err != nil {
			return nil, err
		}
		if it.serialised {
			return nil, bad("serialised items are counted by serial number, not here")
		}
		have, err := balance(ctx, s, l.ItemID, in.WarehouseID)
		if err != nil {
			return nil, err
		}
		if _, err := s.Tx.Exec(ctx, `INSERT INTO inventory.count_lines (count_id, item_id, expected, counted) VALUES ($1,$2,$3,$4)`, id, l.ItemID, have, l.Counted); err != nil {
			return nil, err
		}
	}
	if err := s.Audit(ctx, "create", "stock_count", id); err != nil {
		return nil, err
	}
	c, err := scanCount(s.Tx.QueryRow(ctx, `SELECT `+countCols+countFrom+` WHERE c.id::text = $1`, id))
	if err != nil {
		return nil, err
	}
	if err := loadLines(r, s, c); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusCreated, Body: c}, nil
}

func listCounts(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	rows, err := s.Tx.Query(r.Context(), `SELECT `+countCols+countFrom+` ORDER BY c.created_at DESC LIMIT 100`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Count{}
	for rows.Next() {
		c, err := scanCount(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return &platform.Result{Status: http.StatusOK, Body: map[string]any{"data": out}}, rows.Err()
}

func getCount(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	c, err := scanCount(s.Tx.QueryRow(r.Context(), `SELECT `+countCols+countFrom+` WHERE c.id::text = $1`, r.PathValue("id")))
	if err != nil {
		return nil, err
	}
	if err := loadLines(r, s, c); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: c}, nil
}

// approveCount turns the differences into adjustment movements. The difference is counted minus the
// quantity expected when the count was taken, so movements made since then are kept, not overwritten.
func approveCount(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	ctx := r.Context()
	c, err := scanCount(s.Tx.QueryRow(ctx, `SELECT `+countCols+countFrom+` WHERE c.id::text = $1 FOR UPDATE OF c`, r.PathValue("id")))
	if err != nil {
		return nil, err
	}
	if c.Status != "draft" {
		return nil, platform.Err(http.StatusConflict, "Already approved", "this stock count has already been approved")
	}
	if c.CountedBy == s.Claims.Sub {
		return nil, platform.Err(http.StatusForbidden, "Dual control", "a stock count must be approved by someone other than the counter")
	}
	if err := loadLines(r, s, c); err != nil {
		return nil, err
	}
	for _, l := range c.Lines {
		if l.Variance == 0 {
			continue
		}
		if _, err := balance(ctx, s, l.ItemID, c.WarehouseID); err != nil { // takes the stock lock so concurrent movements queue behind us
			return nil, err
		}
		if err := move(ctx, s, l.ItemID, c.WarehouseID, l.Variance, "adjustment", "count:"+c.ID, nil); err != nil {
			return nil, err
		}
	}
	if _, err := s.Tx.Exec(ctx, `UPDATE inventory.counts SET status='approved', approved_by=$2, approved_at=now() WHERE id=$1`, c.ID, s.Claims.Sub); err != nil {
		return nil, err
	}
	if err := s.Audit(ctx, "approve", "stock_count", c.ID); err != nil {
		return nil, err
	}
	return getCount(r, s)
}

// reorder lists non-serialised items whose stock across all warehouses is below their minimum.
func reorder(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	rows, err := s.Tx.Query(r.Context(), `SELECT i.id::text, i.sku, i.name, i.min_stock, COALESCE(sum(m.qty),0)::int AS on_hand
		FROM inventory.items i LEFT JOIN inventory.moves m ON m.item_id = i.id
		WHERE i.min_stock > 0 AND NOT i.serialised
		GROUP BY i.id, i.sku, i.name, i.min_stock
		HAVING COALESCE(sum(m.qty),0) < i.min_stock
		ORDER BY (i.min_stock - COALESCE(sum(m.qty),0)) DESC, i.sku`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, sku, name string
		var min, have int
		if err := rows.Scan(&id, &sku, &name, &min, &have); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"item_id": id, "sku": sku, "item": name, "min_stock": min, "on_hand": have, "shortfall": min - have})
	}
	return &platform.Result{Status: http.StatusOK, Body: map[string]any{"data": out}}, rows.Err()
}
