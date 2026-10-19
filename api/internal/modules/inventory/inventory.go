// Package inventory is the Inventory & Assets context: stock ledger, serial numbers,
// equipment issued to staff and client assets under contract.
package inventory

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/opeshealthsystems/heckate/api/internal/docs"
	"github.com/opeshealthsystems/heckate/api/internal/modules/documents"
	"github.com/opeshealthsystems/heckate/api/internal/platform"
)

var (
	keepers  = []string{"ceo", "manager", "logistics"}
	approver = []string{"ceo", "manager"}
	staff    = []string{"ceo", "manager", "logistics", "engineer", "support", "sales", "finance", "hr", "employee"}
)

func Register(a *platform.App, o docs.Options) {
	m := &module{docs: o}
	a.Route("POST /v1/items", keepers, platform.Idempotent(createItem))
	a.Route("GET /v1/items", platform.Portal, listItems)
	a.Route("POST /v1/warehouses", keepers, platform.Idempotent(createWarehouse))
	a.Route("GET /v1/warehouses", staff, listWarehouses)
	a.Route("POST /v1/stock/receipts", keepers, platform.Idempotent(receipt))
	a.Route("POST /v1/stock/issues", keepers, platform.Idempotent(issue))
	a.Route("POST /v1/stock/transfers", keepers, platform.Idempotent(transfer))
	a.Route("POST /v1/stock/adjustments", approver, platform.Idempotent(adjust))
	a.Route("GET /v1/stock", staff, levels)
	registerExports(a)
	a.Route("GET /v1/stock/reorder", staff, reorder)
	a.Route("POST /v1/stock/counts", keepers, platform.Idempotent(createCount))
	a.Route("GET /v1/stock/counts", staff, listCounts)
	a.Route("GET /v1/stock/counts/{id}", staff, getCount)
	a.Route("POST /v1/stock/counts/{id}/approve", approver, platform.Idempotent(approveCount))
	a.Route("POST /v1/assets/issue", keepers, platform.Idempotent(issueAsset))
	a.Route("POST /v1/assets/return", keepers, platform.Idempotent(returnAsset))
	a.Route("POST /v1/assets/install", keepers, platform.Idempotent(installAsset))
	a.Route("GET /v1/assets", platform.Portal, listAssets)
	a.Route("GET /v1/assets/forms/{ref}/pdf", staff, m.assetFormPDF)
}

func nf(what string) error { return platform.Err(http.StatusNotFound, "Not found", what+" not found") }
func bad(msg string) error {
	return platform.Err(http.StatusUnprocessableEntity, "Invalid request", msg)
}
func isNF(err error) bool {
	return errors.Is(err, pgx.ErrNoRows) || (err != nil && strings.Contains(err.Error(), "invalid input syntax"))
}

type Item struct {
	ID         string `json:"id"`
	SKU        string `json:"sku"`
	Name       string `json:"name"`
	Unit       string `json:"unit"`
	Serialised bool   `json:"serialised"`
	MinStock   int    `json:"min_stock"`
	OnHand     int64  `json:"on_hand"`
	Low        bool   `json:"low"`
}

func createItem(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		SKU        string `json:"sku"`
		Name       string `json:"name"`
		Unit       string `json:"unit"`
		Serialised bool   `json:"serialised"`
		MinStock   int    `json:"min_stock"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.SKU) == "" || strings.TrimSpace(in.Name) == "" || in.MinStock < 0 {
		return nil, bad("sku and name are required and min_stock cannot be negative")
	}
	if in.Unit == "" {
		in.Unit = "pcs"
	}
	var it Item
	err := s.Tx.QueryRow(r.Context(), `INSERT INTO inventory.items (sku, name, unit, serialised, min_stock) VALUES ($1,$2,$3,$4,$5)
		RETURNING id, sku, name, unit, serialised, min_stock`, strings.TrimSpace(in.SKU), strings.TrimSpace(in.Name), in.Unit, in.Serialised, in.MinStock).
		Scan(&it.ID, &it.SKU, &it.Name, &it.Unit, &it.Serialised, &it.MinStock)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") {
			return nil, platform.Err(http.StatusConflict, "Exists", "an item with this SKU already exists")
		}
		return nil, err
	}
	if err := s.Audit(r.Context(), "create", "item", it.ID); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusCreated, Body: it}, nil
}

func listItems(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	rows, err := s.Tx.Query(r.Context(), `SELECT i.id, i.sku, i.name, i.unit, i.serialised, i.min_stock,
		coalesce((SELECT sum(qty) FROM inventory.moves m WHERE m.item_id = i.id),0)
		FROM inventory.items i ORDER BY i.sku LIMIT 500`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Item{}
	for rows.Next() {
		var it Item
		if err := rows.Scan(&it.ID, &it.SKU, &it.Name, &it.Unit, &it.Serialised, &it.MinStock, &it.OnHand); err != nil {
			return nil, err
		}
		it.Low = it.OnHand < int64(it.MinStock)
		out = append(out, it)
	}
	return &platform.Result{Status: http.StatusOK, Body: map[string]any{"data": out}}, rows.Err()
}

func createWarehouse(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		Name string `json:"name"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.Name) == "" {
		return nil, bad("name is required")
	}
	var id string
	if err := s.Tx.QueryRow(r.Context(), `INSERT INTO inventory.warehouses (name) VALUES ($1) RETURNING id`, strings.TrimSpace(in.Name)).Scan(&id); err != nil {
		if strings.Contains(err.Error(), "duplicate") {
			return nil, platform.Err(http.StatusConflict, "Exists", "a warehouse with this name already exists")
		}
		return nil, err
	}
	return &platform.Result{Status: http.StatusCreated, Body: map[string]string{"id": id, "name": in.Name}}, nil
}

func listWarehouses(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	rows, err := s.Tx.Query(r.Context(), `SELECT id, name FROM inventory.warehouses ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type w struct{ ID, Name string }
	out := []map[string]string{}
	for rows.Next() {
		var x w
		if err := rows.Scan(&x.ID, &x.Name); err != nil {
			return nil, err
		}
		out = append(out, map[string]string{"id": x.ID, "name": x.Name})
	}
	return &platform.Result{Status: http.StatusOK, Body: map[string]any{"data": out}}, rows.Err()
}

// balance returns on-hand stock for an item in a warehouse, after taking a per-pair lock so two
// concurrent issues cannot both pass the check.
func balance(ctx context.Context, s *platform.Scope, item, wh string) (int64, error) {
	if _, err := s.Tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, "stock:"+item+":"+wh); err != nil {
		return 0, err
	}
	var n int64
	err := s.Tx.QueryRow(ctx, `SELECT coalesce(sum(qty),0) FROM inventory.moves WHERE item_id::text=$1 AND warehouse_id::text=$2`, item, wh).Scan(&n)
	return n, err
}

func move(ctx context.Context, s *platform.Scope, item, wh string, qty int, kind, ref string, serial any) error {
	_, err := s.Tx.Exec(ctx, `INSERT INTO inventory.moves (item_id, warehouse_id, qty, kind, ref, serial, actor) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		item, wh, qty, kind, ref, serial, s.Claims.Sub)
	if err != nil && (strings.Contains(err.Error(), "foreign key") || strings.Contains(err.Error(), "invalid input syntax")) {
		return bad("unknown item or warehouse")
	}
	return err
}

type itemInfo struct {
	serialised bool
}

func lookupItem(ctx context.Context, s *platform.Scope, id string) (*itemInfo, error) {
	var it itemInfo
	if err := s.Tx.QueryRow(ctx, `SELECT serialised FROM inventory.items WHERE id::text=$1`, id).Scan(&it.serialised); err != nil {
		if isNF(err) {
			return nil, bad("unknown item")
		}
		return nil, err
	}
	return &it, nil
}

type qtyReq struct {
	ItemID      string   `json:"item_id"`
	WarehouseID string   `json:"warehouse_id"`
	Qty         int      `json:"qty"`
	Ref         string   `json:"ref"`
	Serials     []string `json:"serials"`
}

func receipt(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in qtyReq
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	if in.Qty <= 0 || in.Qty > 1_000_000 {
		return nil, bad("qty must be a positive number")
	}
	ctx := r.Context()
	it, err := lookupItem(ctx, s, in.ItemID)
	if err != nil {
		return nil, err
	}
	if it.serialised {
		if len(in.Serials) != in.Qty {
			return nil, bad("a serialised item needs exactly one serial number per unit received")
		}
	} else if len(in.Serials) > 0 {
		return nil, bad("serial numbers are only for serialised items")
	}
	if err := move(ctx, s, in.ItemID, in.WarehouseID, in.Qty, "receipt", in.Ref, nil); err != nil {
		return nil, err
	}
	for _, sn := range in.Serials {
		sn = strings.TrimSpace(sn)
		if sn == "" {
			return nil, bad("serial numbers cannot be empty")
		}
		if _, err := s.Tx.Exec(ctx, `INSERT INTO inventory.serials (item_id, serial, warehouse_id) VALUES ($1,$2,$3)`, in.ItemID, sn, in.WarehouseID); err != nil {
			if strings.Contains(err.Error(), "duplicate") {
				return nil, platform.Err(http.StatusConflict, "Duplicate serial", "serial "+sn+" already exists")
			}
			return nil, err
		}
	}
	if err := s.Emit(ctx, "stock.received", map[string]any{"item_id": in.ItemID, "qty": in.Qty, "ref": in.Ref}); err != nil {
		return nil, err
	}
	if err := s.Audit(ctx, "receipt", "stock", in.ItemID); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusCreated, Body: map[string]any{"item_id": in.ItemID, "received": in.Qty}}, nil
}

// Issue removes non-serialised stock. Other modules call it inside their own transaction
// (for example an intervention using spare parts).
func Issue(ctx context.Context, s *platform.Scope, item, wh string, qty int, ref string) error {
	if qty <= 0 {
		return bad("qty must be positive")
	}
	it, err := lookupItem(ctx, s, item)
	if err != nil {
		return err
	}
	if it.serialised {
		return bad("serialised items are issued by serial number through /v1/assets")
	}
	have, err := balance(ctx, s, item, wh)
	if err != nil {
		return err
	}
	if have < int64(qty) {
		return platform.Err(http.StatusConflict, "Insufficient stock", fmt.Sprintf("only %d on hand", have))
	}
	return move(ctx, s, item, wh, -qty, "issue", ref, nil)
}

// Receive adds non-serialised stock. The procurement module calls it when goods arrive against a purchase order.
func Receive(ctx context.Context, s *platform.Scope, item, wh string, qty int, ref string) error {
	if qty <= 0 {
		return bad("qty must be positive")
	}
	it, err := lookupItem(ctx, s, item)
	if err != nil {
		return err
	}
	if it.serialised {
		return bad("serialised items are received with their serial numbers through /v1/stock/receipts")
	}
	return move(ctx, s, item, wh, qty, "receipt", ref, nil)
}

func issue(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in qtyReq
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	if err := Issue(r.Context(), s, in.ItemID, in.WarehouseID, in.Qty, in.Ref); err != nil {
		return nil, err
	}
	if err := s.Audit(r.Context(), "issue", "stock", in.ItemID); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusCreated, Body: map[string]any{"item_id": in.ItemID, "issued": in.Qty}}, nil
}

func transfer(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		ItemID string `json:"item_id"`
		From   string `json:"from_warehouse_id"`
		To     string `json:"to_warehouse_id"`
		Qty    int    `json:"qty"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	if in.Qty <= 0 || in.From == in.To {
		return nil, bad("qty must be positive and the warehouses must differ")
	}
	ctx := r.Context()
	it, err := lookupItem(ctx, s, in.ItemID)
	if err != nil {
		return nil, err
	}
	if it.serialised {
		return nil, bad("serialised items cannot be transferred in bulk")
	}
	have, err := balance(ctx, s, in.ItemID, in.From)
	if err != nil {
		return nil, err
	}
	if have < int64(in.Qty) {
		return nil, platform.Err(http.StatusConflict, "Insufficient stock", fmt.Sprintf("only %d on hand", have))
	}
	if err := move(ctx, s, in.ItemID, in.From, -in.Qty, "transfer_out", "", nil); err != nil {
		return nil, err
	}
	if err := move(ctx, s, in.ItemID, in.To, in.Qty, "transfer_in", "", nil); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusCreated, Body: map[string]any{"item_id": in.ItemID, "moved": in.Qty}}, nil
}

func adjust(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		ItemID      string `json:"item_id"`
		WarehouseID string `json:"warehouse_id"`
		Qty         int    `json:"qty"`
		Reason      string `json:"reason"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	if in.Qty == 0 || strings.TrimSpace(in.Reason) == "" {
		return nil, bad("a non-zero qty and a reason are required")
	}
	ctx := r.Context()
	it, err := lookupItem(ctx, s, in.ItemID)
	if err != nil {
		return nil, err
	}
	if it.serialised {
		return nil, bad("serialised items are corrected by serial number")
	}
	if in.Qty < 0 {
		have, err := balance(ctx, s, in.ItemID, in.WarehouseID)
		if err != nil {
			return nil, err
		}
		if have < int64(-in.Qty) {
			return nil, platform.Err(http.StatusConflict, "Insufficient stock", fmt.Sprintf("only %d on hand", have))
		}
	}
	if err := move(ctx, s, in.ItemID, in.WarehouseID, in.Qty, "adjustment", strings.TrimSpace(in.Reason), nil); err != nil {
		return nil, err
	}
	if err := s.Audit(ctx, "adjust", "stock", in.ItemID); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusCreated, Body: map[string]any{"item_id": in.ItemID, "adjusted": in.Qty}}, nil
}

func levels(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	rows, err := s.Tx.Query(r.Context(), `SELECT i.id, i.sku, i.name, w.id, w.name, sum(m.qty)
		FROM inventory.moves m JOIN inventory.items i ON i.id = m.item_id JOIN inventory.warehouses w ON w.id = m.warehouse_id
		GROUP BY i.id, i.sku, i.name, w.id, w.name HAVING sum(m.qty) <> 0 ORDER BY i.sku, w.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var iid, sku, name, wid, wname string
		var qty int64
		if err := rows.Scan(&iid, &sku, &name, &wid, &wname, &qty); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"item_id": iid, "sku": sku, "item": name, "warehouse_id": wid, "warehouse": wname, "qty": qty})
	}
	return &platform.Result{Status: http.StatusOK, Body: map[string]any{"data": out}}, rows.Err()
}

// ---- serial-numbered assets ----

type Asset struct {
	ID       string  `json:"id"`
	ItemID   string  `json:"item_id"`
	Item     string  `json:"item"`
	Serial   string  `json:"serial"`
	Status   string  `json:"status"`
	Holder   *string `json:"holder"`
	TenantID *string `json:"tenant_id"`
}

const assetCols = `s.id, s.item_id, i.name, s.serial, s.status, s.holder, s.tenant_id`

func scanAsset(row pgx.Row) (*Asset, error) {
	var a Asset
	err := row.Scan(&a.ID, &a.ItemID, &a.Item, &a.Serial, &a.Status, &a.Holder, &a.TenantID)
	if isNF(err) {
		return nil, nf("asset")
	}
	return &a, err
}

func lockSerial(ctx context.Context, s *platform.Scope, id string) (*Asset, error) {
	return scanAsset(s.Tx.QueryRow(ctx, `SELECT `+assetCols+` FROM inventory.serials s JOIN inventory.items i ON i.id = s.item_id WHERE s.id::text=$1 FOR UPDATE OF s`, id))
}

func listAssets(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	rows, err := s.Tx.Query(r.Context(), `SELECT `+assetCols+` FROM inventory.serials s JOIN inventory.items i ON i.id = s.item_id ORDER BY i.name, s.serial LIMIT 500`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Asset{}
	for rows.Next() {
		a, err := scanAsset(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return &platform.Result{Status: http.StatusOK, Body: map[string]any{"data": out}}, rows.Err()
}

// Issuing equipment removes the unit from stock and records who holds it (HS-FRM-38).
func issueAsset(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		SerialID string `json:"serial_id"`
		Assignee string `json:"assignee"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.Assignee) == "" {
		return nil, bad("assignee is required")
	}
	ctx := r.Context()
	a, err := lockSerial(ctx, s, in.SerialID)
	if err != nil {
		return nil, err
	}
	if a.Status != "in_stock" {
		return nil, platform.Err(http.StatusConflict, "Not available", "only in-stock equipment can be issued")
	}
	var wh string
	if err := s.Tx.QueryRow(ctx, `SELECT warehouse_id FROM inventory.serials WHERE id=$1`, a.ID).Scan(&wh); err != nil {
		return nil, err
	}
	if err := move(ctx, s, a.ItemID, wh, -1, "issue", "asset:"+a.Serial, a.Serial); err != nil {
		return nil, err
	}
	doc, err := documents.Issue(ctx, s, "FRM-38", "Asset issue: "+a.Item+" "+a.Serial, "", "")
	if err != nil {
		return nil, err
	}
	if _, err := s.Tx.Exec(ctx, `UPDATE inventory.serials SET status='issued', holder=$2, warehouse_id=NULL WHERE id=$1`, a.ID, strings.TrimSpace(in.Assignee)); err != nil {
		return nil, err
	}
	if _, err := s.Tx.Exec(ctx, `INSERT INTO inventory.assignments (serial_id, assignee, issued_by, document_ref) VALUES ($1,$2,$3,$4)`, a.ID, strings.TrimSpace(in.Assignee), s.Claims.Sub, doc.Ref); err != nil {
		return nil, err
	}
	if err := s.Audit(ctx, "issue", "asset", a.ID); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: map[string]any{"serial_id": a.ID, "assignee": in.Assignee, "document_ref": doc.Ref}}, nil
}

func returnAsset(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		SerialID    string `json:"serial_id"`
		WarehouseID string `json:"warehouse_id"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	ctx := r.Context()
	a, err := lockSerial(ctx, s, in.SerialID)
	if err != nil {
		return nil, err
	}
	if a.Status != "issued" {
		return nil, platform.Err(http.StatusConflict, "Not issued", "only issued equipment can be returned")
	}
	if err := move(ctx, s, a.ItemID, in.WarehouseID, 1, "receipt", "return:"+a.Serial, a.Serial); err != nil {
		return nil, err
	}
	if _, err := s.Tx.Exec(ctx, `UPDATE inventory.serials SET status='in_stock', holder=NULL, warehouse_id=$2 WHERE id=$1`, a.ID, in.WarehouseID); err != nil {
		return nil, err
	}
	if _, err := s.Tx.Exec(ctx, `UPDATE inventory.assignments SET returned_at=now() WHERE serial_id=$1 AND returned_at IS NULL`, a.ID); err != nil {
		return nil, err
	}
	if err := s.Audit(ctx, "return", "asset", a.ID); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: map[string]any{"serial_id": a.ID, "status": "in_stock"}}, nil
}

// Installing equipment at a client under contract takes it out of stock and shows it in their portal.
func installAsset(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		SerialID string `json:"serial_id"`
		TenantID string `json:"tenant_id"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	if in.TenantID == "" {
		return nil, bad("tenant_id is required")
	}
	ctx := r.Context()
	a, err := lockSerial(ctx, s, in.SerialID)
	if err != nil {
		return nil, err
	}
	if a.Status != "in_stock" {
		return nil, platform.Err(http.StatusConflict, "Not available", "only in-stock equipment can be installed at a client")
	}
	var wh string
	if err := s.Tx.QueryRow(ctx, `SELECT warehouse_id FROM inventory.serials WHERE id=$1`, a.ID).Scan(&wh); err != nil {
		return nil, err
	}
	if err := move(ctx, s, a.ItemID, wh, -1, "issue", "client:"+in.TenantID, a.Serial); err != nil {
		return nil, err
	}
	if _, err := s.Tx.Exec(ctx, `UPDATE inventory.serials SET status='client_asset', tenant_id=$2, warehouse_id=NULL WHERE id=$1`, a.ID, in.TenantID); err != nil {
		if strings.Contains(err.Error(), "foreign key") {
			return nil, bad("unknown tenant")
		}
		return nil, err
	}
	if err := s.Audit(ctx, "install", "asset", a.ID); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: map[string]any{"serial_id": a.ID, "status": "client_asset", "tenant_id": in.TenantID, "at": time.Now().UTC()}}, nil
}

type module struct{ docs docs.Options }

// assetFormPDF serves the equipment issue and return form (HS-FRM-38) of one assignment.
func (m *module) assetFormPDF(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	ctx := r.Context()
	ref := r.PathValue("ref")
	var a docs.Asset
	err := s.Tx.QueryRow(ctx, `SELECT i.name, sr.serial, g.assignee, g.issued_by, g.issued_at
		FROM inventory.assignments g JOIN inventory.serials sr ON sr.id = g.serial_id JOIN inventory.items i ON i.id = sr.item_id
		WHERE g.document_ref = $1`, ref).Scan(&a.Item, &a.Serial, &a.Holder, &a.IssuedBy, &a.IssuedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nf("asset form")
	}
	if err != nil {
		return nil, err
	}
	return documents.ServePDF(ctx, s, ref, func() ([]byte, error) { return docs.RenderAssetForm(m.docs, ref, a) })
}
