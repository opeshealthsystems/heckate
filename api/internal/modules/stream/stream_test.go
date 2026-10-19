package stream

import (
	"testing"

	"github.com/opeshealthsystems/heckate/api/internal/platform"
)

func TestAllowed(t *testing.T) {
	staff := &platform.Claims{Sub: "s", Roles: []string{"support"}}
	client := &platform.Claims{Sub: "c", TenantID: "ten-a", Roles: []string{"client_user"}}
	noTenant := &platform.Claims{Sub: "c2", Roles: []string{"client_user"}}
	supplier := &platform.Claims{Sub: "s2", TenantID: "ten-a", Roles: []string{"supplier"}}
	mine := map[string]any{"tenant_id": "ten-a"}
	theirs := map[string]any{"tenant_id": "ten-b"}
	cases := []struct {
		name    string
		c       *platform.Claims
		subject string
		p       map[string]any
		want    bool
	}{
		{"staff ticket", staff, "ticket.commented", theirs, true},
		{"staff visit", staff, "intervention.completed", theirs, true},
		{"client own ticket", client, "ticket.status_changed", mine, true},
		{"client other tenant", client, "ticket.status_changed", theirs, false},
		{"client event without tenant", client, "ticket.commented", map[string]any{}, false},
		{"client with no tenant claim", noTenant, "ticket.commented", map[string]any{}, false},
		{"supplier gets nothing", supplier, "ticket.commented", mine, false},
		{"finance events are not streamed", staff, "invoice.issued", mine, false},
		{"ledger events are not streamed", staff, "payment.recorded", mine, false},
	}
	for _, tc := range cases {
		if got := Allowed(tc.c, tc.subject, tc.p); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}
