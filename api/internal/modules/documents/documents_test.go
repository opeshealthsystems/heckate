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

// The check character follows ISO/IEC 7064 MOD 37,36 exactly as the brand pack's certificate generators compute it.
func TestCheckCharMatchesThePackGenerator(t *testing.T) {
	for body, want := range map[string]byte{"HS-ASM-2026-000148": 'P', "HS-ASM-2026-000001": 'S', "HS-CRT-A-2026-000012": 'Z'} {
		if got := CheckChar(body); got != want {
			t.Errorf("CheckChar(%s) = %c, want %c", body, got, want)
		}
		if !ValidRef(body + "-" + string(want)) {
			t.Errorf("%s-%c must be a valid reference", body, want)
		}
	}
	if ValidRef("HS-ASM-2026-000148-Q") {
		t.Error("a wrong check character must be refused")
	}
}

func TestListLimit(t *testing.T) {
	for in, want := range map[string]int{"": 50, "abc": 50, "0": 50, "-3": 50, "10": 10, "200": 200, "9999": 200} {
		if got := listLimit(in); got != want {
			t.Errorf("listLimit(%q) = %d, want %d", in, got, want)
		}
	}
}
