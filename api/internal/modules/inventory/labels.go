package inventory

import (
	"net/http"
	"strings"
	"time"

	"github.com/opeshealthsystems/heckate/api/internal/docs"
	"github.com/opeshealthsystems/heckate/api/internal/platform"
	"github.com/opeshealthsystems/heckate/api/internal/reports"
)

func registerLabels(a *platform.App, m *module) {
	a.Route("GET /v1/assets/labels/pdf", staff, m.assetLabels)
	a.Route("GET /v1/items/labels/pdf", staff, m.itemLabels)
	a.Route("GET /v1/assets/{id}", platform.Portal, getAsset)
	a.Route("GET /v1/items/{id}", staff, getItem)
}

func ids(r *http.Request) ([]string, error) {
	var out []string
	for _, p := range strings.Split(r.URL.Query().Get("ids"), ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil, bad("ids is required: a comma-separated list of ids")
	}
	if len(out) > 480 {
		return nil, bad("at most 480 labels at once")
	}
	return out, nil
}

func (m *module) base() string { return strings.TrimRight(m.docs.PortalURL, "/") }

// assetLabels prints a sheet of QR stickers, one per serial-numbered unit. The QR opens the unit in the platform,
// which asks the person to sign in, so a photo of a sticker reveals nothing.
func (m *module) assetLabels(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	list, err := ids(r)
	if err != nil {
		return nil, err
	}
	labels := make([]docs.Label, 0, len(list))
	for _, id := range list {
		a, err := lockSerialShared(r, s, id)
		if err != nil {
			return nil, err
		}
		extra := ""
		switch {
		case a.Holder != nil:
			extra = "Held by " + *a.Holder
		case a.TenantID != nil:
			extra = "Installed at client"
		}
		labels = append(labels, docs.Label{Kind: "Asset", Name: a.Item, Code: a.Serial, Extra: extra, URL: m.base() + "/asset?id=" + a.ID})
	}
	pdf, err := docs.RenderLabels(m.docs, labels, time.Now().UTC())
	if err != nil {
		return nil, bad(err.Error())
	}
	return reports.PDF("asset-labels", pdf), nil
}

func (m *module) itemLabels(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	list, err := ids(r)
	if err != nil {
		return nil, err
	}
	labels := make([]docs.Label, 0, len(list))
	for _, id := range list {
		var name, sku string
		var uid string
		if err := s.Tx.QueryRow(r.Context(), `SELECT id::text, name, sku FROM inventory.items WHERE id::text=$1`, id).Scan(&uid, &name, &sku); err != nil {
			if isNF(err) {
				return nil, nf("item")
			}
			return nil, err
		}
		labels = append(labels, docs.Label{Kind: "Stock item", Name: name, Code: sku, URL: m.base() + "/asset?item=" + uid})
	}
	pdf, err := docs.RenderLabels(m.docs, labels, time.Now().UTC())
	if err != nil {
		return nil, bad(err.Error())
	}
	return reports.PDF("item-labels", pdf), nil
}

func lockSerialShared(r *http.Request, s *platform.Scope, id string) (*Asset, error) {
	return scanAsset(s.Tx.QueryRow(r.Context(), `SELECT `+assetCols+` FROM inventory.serials s JOIN inventory.items i ON i.id = s.item_id WHERE s.id::text=$1`, id))
}

// getAsset is where a scanned sticker lands: the unit, who holds it, and its movement history.
func getAsset(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	a, err := lockSerialShared(r, s, r.PathValue("id"))
	if err != nil {
		return nil, err
	}
	type hist struct {
		Assignee   string     `json:"assignee"`
		IssuedBy   string     `json:"issued_by"`
		IssuedAt   time.Time  `json:"issued_at"`
		ReturnedAt *time.Time `json:"returned_at"`
		Ref        *string    `json:"document_ref"`
	}
	out := []hist{}
	if s.Claims.Scope() == "staff" {
		rows, err := s.Tx.Query(r.Context(), `SELECT assignee, issued_by, issued_at, returned_at, document_ref FROM inventory.assignments WHERE serial_id=$1 ORDER BY issued_at DESC`, a.ID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var h hist
			if err := rows.Scan(&h.Assignee, &h.IssuedBy, &h.IssuedAt, &h.ReturnedAt, &h.Ref); err != nil {
				return nil, err
			}
			out = append(out, h)
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	return &platform.Result{Status: http.StatusOK, Body: map[string]any{"asset": a, "history": out}}, nil
}

func getItem(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var it Item
	err := s.Tx.QueryRow(r.Context(), `SELECT i.id::text, i.sku, i.name, i.unit, i.serialised, i.min_stock, COALESCE((SELECT sum(qty) FROM inventory.moves m WHERE m.item_id=i.id),0)
		FROM inventory.items i WHERE i.id::text=$1`, r.PathValue("id")).Scan(&it.ID, &it.SKU, &it.Name, &it.Unit, &it.Serialised, &it.MinStock, &it.OnHand)
	if isNF(err) {
		return nil, nf("item")
	}
	if err != nil {
		return nil, err
	}
	it.Low = it.OnHand < int64(it.MinStock)
	return &platform.Result{Status: http.StatusOK, Body: it}, nil
}
