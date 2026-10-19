package notify

import "fmt"

// Message is what the phone shows. Data lets the app open the right screen.
type Message struct {
	Title string
	Body  string
	Data  map[string]any
}

// Audience says which devices get a message: narrowed to one user, one tenant and/or some roles.
// Except is the person who caused the event, so they are not notified of their own action.
type Audience struct {
	Actor  string
	Tenant string
	Roles  []string
	Except string
}

var (
	clientRoles = []string{"client_user", "client_admin"}
	pageRoles   = []string{"support", "manager", "ceo", "engineer"} // no on-call roster yet: page everyone who could respond
)

func str(p map[string]any, k string) string {
	v, _ := p[k].(string)
	return v
}

// Plan maps a domain event to a push notification, or returns nil when the event is not one that
// should reach a phone. It is pure so the rules can be tested without a database.
func Plan(subject string, p map[string]any) (*Message, *Audience) {
	id := str(p, "id")
	switch subject {
	case "ticket.status_changed":
		return &Message{"Ticket update", fmt.Sprintf("%s is now %s", str(p, "subject"), str(p, "to")), map[string]any{"screen": "ticket", "id": id}},
			&Audience{Tenant: str(p, "tenant_id"), Roles: clientRoles, Except: str(p, "actor")}
	case "ticket.commented":
		if str(p, "by") == "client" {
			return &Message{"Client replied", str(p, "subject"), map[string]any{"screen": "ticket", "id": id}},
				&Audience{Roles: []string{"support", "manager"}, Except: str(p, "actor")}
		}
		return &Message{"New reply from HEKATE", str(p, "subject"), map[string]any{"screen": "ticket", "id": id}},
			&Audience{Tenant: str(p, "tenant_id"), Roles: clientRoles, Except: str(p, "actor")}
	case "ticket.p1.page":
		return &Message{"P1 incident", str(p, "subject"), map[string]any{"screen": "ticket", "id": id}}, &Audience{Roles: pageRoles}
	case "intervention.scheduled":
		return &Message{"New site visit", "A site visit has been assigned to you", map[string]any{"screen": "intervention", "id": id}},
			&Audience{Actor: str(p, "engineer")}
	case "quote.sent":
		return &Message{"New quotation", "A quotation is waiting for your approval", map[string]any{"screen": "quote", "id": id}},
			&Audience{Tenant: str(p, "tenant_id"), Roles: []string{"client_admin"}}
	case "invoice.issued":
		return &Message{"New invoice", fmt.Sprintf("Invoice %s has been issued", str(p, "ref")), map[string]any{"screen": "invoice", "id": id}},
			&Audience{Tenant: str(p, "tenant_id"), Roles: []string{"client_admin"}}
	case "leave.approved", "leave.rejected":
		verdict := "approved"
		if subject == "leave.rejected" {
			verdict = "rejected"
		}
		return &Message{"Leave request " + verdict, "Your leave request was " + verdict, map[string]any{"screen": "account"}}, &Audience{Actor: str(p, "actor")}
	case "expense.paid":
		return &Message{"Expense claim paid", "Your expense claim has been paid", map[string]any{"screen": "account"}}, &Audience{Actor: str(p, "actor")}
	}
	return nil, nil
}
