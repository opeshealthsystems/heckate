package files

import "testing"

func TestDetectAllowsOnlyRealImagesAndPDFs(t *testing.T) {
	png := append([]byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}, make([]byte, 20)...)
	jpg := append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, make([]byte, 20)...)
	pdf := []byte("%PDF-1.7\n1 0 obj")
	for name, b := range map[string][]byte{"png": png, "jpeg": jpg, "pdf": pdf} {
		if _, ok := Detect(b); !ok {
			t.Errorf("%s should be allowed", name)
		}
	}
	for name, b := range map[string][]byte{
		"svg":        []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`),
		"html":       []byte("<!DOCTYPE html><html><script>alert(1)</script></html>"),
		"html tag":   []byte("<html><body>hi</body></html>"),
		"javascript": []byte("alert(document.cookie)"),
		"exe":        append([]byte("MZ"), make([]byte, 40)...),
		"zip":        append([]byte("PK\x03\x04"), make([]byte, 40)...),
		"empty":      {},
	} {
		if _, ok := Detect(b); ok {
			t.Errorf("%s must be rejected", name)
		}
	}
}

func TestSafeName(t *testing.T) {
	cases := map[string]string{
		"photo.jpg":                "photo.jpg",
		"../../etc/passwd":         "passwd",
		`C:\Users\me\receipt.pdf`:  "receipt.pdf",
		"a\r\nSet-Cookie: x=1.png": "a__Set-Cookie_ x_1.png", // CR and LF can never reach a header
		`evil".png`:                "evil_.png",
		"":                         "file",
		"...":                      "file",
		"résumé.pdf":               "r_sum_.pdf",
	}
	for in, want := range cases {
		if got := SafeName(in); got != want {
			t.Errorf("SafeName(%q) = %q, want %q", in, got, want)
		}
	}
	if len(SafeName(string(make([]byte, 500)))) > 100 {
		t.Error("names must be capped")
	}
}
