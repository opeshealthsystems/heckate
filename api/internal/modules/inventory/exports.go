package inventory

import (
	"net/http"

	"github.com/opeshealthsystems/heckate/api/internal/platform"
	"github.com/opeshealthsystems/heckate/api/internal/reports"
)

func registerExports(a *platform.App) {
	a.Route("GET /v1/exports/stock", staff, exportStock)
	a.Route("GET /v1/exports/assets", staff, exportAssets)
}

func exportStock(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	res, err := levels(r, s)
	if err != nil {
		return nil, err
	}
	rows, err := reports.Rows(res, "data")
	if err != nil {
		return nil, err
	}
	return reports.CSV("stock", []reports.Col{reports.C("SKU", "sku"), reports.C("Item", "item"), reports.C("Warehouse", "warehouse"), reports.C("Quantity", "qty")}, rows)
}

func exportAssets(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	q, err := s.Tx.Query(r.Context(), `SELECT `+assetCols+` FROM inventory.serials s JOIN inventory.items i ON i.id = s.item_id ORDER BY i.name, s.serial LIMIT 50000`)
	if err != nil {
		return nil, err
	}
	defer q.Close()
	out := []*Asset{}
	for q.Next() {
		a, err := scanAsset(q)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	if err := q.Err(); err != nil {
		return nil, err
	}
	rows, err := reports.Rows(&platform.Result{Body: map[string]any{"data": out}}, "data")
	if err != nil {
		return nil, err
	}
	return reports.CSV("assets", []reports.Col{reports.C("Serial", "serial"), reports.C("Item", "item"), reports.C("Status", "status"), reports.C("Held by", "holder"), reports.C("Client id (installed at)", "tenant_id")}, rows)
}
