package platform

// Role groups for route allow-lists. Every route lists the roles that may call it; a route with no list
// is open to any signed-in caller, so only routes that really are open (such as /v1/me) may omit it.
var (
	// StaffAll is every internal role.
	StaffAll = []string{"ceo", "manager", "finance", "sales", "support", "engineer", "hr", "employee", "logistics", "procurement"}
	// ClientAll is every role a client company's people can hold.
	ClientAll = []string{"client_admin", "client_user"}
)

// Join concatenates role groups into one allow-list.
func Join(groups ...[]string) []string {
	var out []string
	for _, g := range groups {
		out = append(out, g...)
	}
	return out
}

// Portal is everyone who signs in to the ERP or a client portal: all staff and all client users. It excludes
// suppliers and the external auditor, who have their own screens and must not reach client-facing routes.
var Portal = Join(StaffAll, ClientAll)
