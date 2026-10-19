package platform

import "fmt"

// TenantFilter returns an extra SQL condition and its argument that narrows a query to the caller's own
// company when the caller is a client; for staff and everyone else it returns nothing.
//
// Row-level security already guarantees a client sees only their own rows, so this never changes what a
// query returns. It exists for speed: the policies read "staff OR tenant_id = mine", and PostgreSQL cannot
// use an index through that OR, so without this predicate every client list scans the whole table and
// gets slower as the data grows. With it, the planner uses the tenant index.
//
// next is the number of the next free placeholder: pass len(args)+1.
func (s *Scope) TenantFilter(col string, next int) (cond string, args []any) {
	if s.Claims != nil && s.Claims.Scope() == "client" && s.Claims.TenantID != "" {
		return fmt.Sprintf(" AND %s = $%d::uuid", col, next), []any{s.Claims.TenantID}
	}
	return "", nil
}
