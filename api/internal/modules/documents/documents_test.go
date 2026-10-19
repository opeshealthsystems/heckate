package documents

import "testing"

func TestRefs(t *testing.T) {
	inv := FormatRef("INV", 2026, 12)
	if inv != "HS-INV-2026-000012" || !ValidRef(inv) {
		t.Fatalf("bad invoice ref %q", inv)
	}
	cert := FormatRef("CRT-A", 2026, 7)
	if !ValidRef(cert) {
		t.Fatalf("certificate ref %q should validate", cert)
	}
	// tamper: change the sequence, keep the check char
	forged := "HS-CRT-A-2026-000008" + cert[len(cert)-2:]
	if ValidRef(forged) {
		t.Errorf("forged certificate %q must fail the check character", forged)
	}
	if ValidRef("HS-CRT-A-2026-000007") {
		t.Error("certificate without check char must fail")
	}
	if ValidRef("HS-INV-2026-000012-X") || ValidRef("nonsense") {
		t.Error("malformed refs must fail")
	}
}
