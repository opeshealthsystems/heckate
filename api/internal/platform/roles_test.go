package platform

import "testing"

// StaffAll must stay in step with the roles that Scope() treats as staff.
func TestStaffAllMatchesStaffRoles(t *testing.T) {
	if len(StaffAll) != len(staffRoles) {
		t.Fatalf("StaffAll has %d roles, staffRoles has %d", len(StaffAll), len(staffRoles))
	}
	for _, r := range StaffAll {
		if !staffRoles[r] {
			t.Errorf("%s is in StaffAll but not in staffRoles", r)
		}
	}
	for _, r := range ClientAll {
		if (&Claims{Roles: []string{r}}).Scope() != "client" {
			t.Errorf("%s must have client scope", r)
		}
	}
}
