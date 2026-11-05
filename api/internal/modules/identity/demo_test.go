package identity

import "testing"

func TestParseDemoLogins(t *testing.T) {
	got := ParseDemoLogins(" Admin|a@x.cm|pw1 ; broken ; Client|c@x.cm|p|w ;Engineer|e@x.cm|pw3;NoMail|nomail|pw;")
	if len(got) != 2 || got[0] != (DemoLogin{"Admin", "a@x.cm", "pw1"}) || got[1] != (DemoLogin{"Engineer", "e@x.cm", "pw3"}) {
		t.Fatalf("unexpected: %+v", got)
	}
	if l := ParseDemoLogins(""); l == nil || len(l) != 0 {
		t.Fatalf("empty must give an empty list, got %+v", l)
	}
}
