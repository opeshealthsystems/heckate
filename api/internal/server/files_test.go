package server

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"
	"time"
)

var pngBytes = append([]byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}, bytes.Repeat([]byte{7}, 200)...)

func (e *env) upload(tok, entityType, entityID, filename string, data []byte) (int, map[string]any) {
	e.t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("entity_type", entityType)
	_ = w.WriteField("entity_id", entityID)
	fw, _ := w.CreateFormFile("file", filename)
	_, _ = fw.Write(data)
	_ = w.Close()
	req, _ := http.NewRequest("POST", e.srv.URL+"/v1/attachments", &buf)
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = jsonDecode(resp.Body, &out)
	return resp.StatusCode, out
}

func (e *env) download(tok, id string) (*http.Response, []byte) {
	req, _ := http.NewRequest("GET", e.srv.URL+"/v1/attachments/"+id, nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp, b
}

func TestTicketAttachments(t *testing.T) {
	e := setup(t)
	staff := token("att-agent", "", "support")
	ta := e.newClient(staff, "AttA "+t.Name())
	tb := e.newClient(staff, "AttB "+t.Name())
	alice := token("att-alice", ta, "client_user")
	bob := token("att-bob", tb, "client_user")
	_, tk, _ := e.do("POST", "/v1/tickets", alice, map[string]any{"subject": "with photo", "severity": "P3", "channel": "app"})
	id := tk["id"].(string)

	code, meta := e.upload(alice, "ticket", id, "../screen shot.png", pngBytes)
	if code != 201 || meta["content_type"] != "image/png" || meta["filename"] != "screen shot.png" || meta["size"] != float64(len(pngBytes)) {
		t.Fatalf("upload: %d %v", code, meta)
	}
	aid := meta["id"].(string)

	// Another tenant can neither attach to, list, nor read it.
	if c, _ := e.upload(bob, "ticket", id, "x.png", pngBytes); c != 404 {
		t.Errorf("cross-tenant upload: want 404, got %d", c)
	}
	if c, _, _ := e.do("GET", "/v1/attachments?entity_type=ticket&entity_id="+id, bob, nil); c != 404 {
		t.Errorf("cross-tenant list: want 404, got %d", c)
	}
	if resp, _ := e.download(bob, aid); resp.StatusCode != 404 {
		t.Errorf("cross-tenant download: want 404, got %d", resp.StatusCode)
	}
	// Staff and the owner can; the bytes round-trip exactly with safe headers.
	for _, tok := range []string{alice, staff} {
		resp, body := e.download(tok, aid)
		if resp.StatusCode != 200 || !bytes.Equal(body, pngBytes) {
			t.Fatalf("download: %d, %d bytes", resp.StatusCode, len(body))
		}
		if resp.Header.Get("Content-Type") != "image/png" || resp.Header.Get("X-Content-Type-Options") != "nosniff" ||
			!strings.HasPrefix(resp.Header.Get("Content-Disposition"), "attachment;") || !strings.Contains(resp.Header.Get("Content-Security-Policy"), "sandbox") {
			t.Errorf("unsafe download headers: %v", resp.Header)
		}
	}
	_, list, _ := e.do("GET", "/v1/attachments?entity_type=ticket&entity_id="+id, alice, nil)
	if n := len(list["data"].([]any)); n != 1 {
		t.Errorf("list should have 1 attachment, got %d", n)
	}
	// Content is judged by its bytes, never by the name or the declared type.
	for name, data := range map[string][]byte{
		"svg":  []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`),
		"html": []byte("<html><script>alert(1)</script></html>"),
		"exe":  append([]byte("MZ"), make([]byte, 100)...),
	} {
		if c, _ := e.upload(alice, "ticket", id, "innocent.png", data); c != 415 {
			t.Errorf("%s disguised as a PNG: want 415, got %d", name, c)
		}
	}
	if c, _ := e.upload(alice, "ticket", id, "empty.png", nil); c != 422 {
		t.Errorf("empty file: want 422, got %d", c)
	}
	big := append(append([]byte{}, pngBytes...), bytes.Repeat([]byte{1}, MaxSize)...)
	if c, _ := e.upload(alice, "ticket", id, "big.png", big); c != 413 {
		t.Errorf("file over 10 MB: want 413, got %d", c)
	}
	if c, _ := e.upload(alice, "ticket", "not-a-uuid", "x.png", pngBytes); c != 404 {
		t.Errorf("bad entity id: want 404, got %d", c)
	}
	if c, _ := e.upload(alice, "invoice", id, "x.png", pngBytes); c != 422 {
		t.Errorf("unsupported entity: want 422, got %d", c)
	}
	if c, _ := e.upload("", "ticket", id, "x.png", pngBytes); c != 401 {
		t.Errorf("anonymous upload: want 401, got %d", c)
	}
}

func TestExpenseReceiptsArePrivate(t *testing.T) {
	e := setup(t)
	sfx := fmt.Sprint(time.Now().UnixNano())
	emp := token("rcpt-"+sfx, "", "employee")
	peer := token("peer-rcpt-"+sfx, "", "employee")
	mgr := token("rcpt-mgr", "", "manager")
	day := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")
	_, cl, _ := e.do("POST", "/v1/expenses", emp, map[string]any{"expense_date": day, "category": "travel", "description": "taxi", "amount": 8000})
	id := cl["id"].(string)

	code, meta := e.upload(emp, "expense", id, "receipt.png", pngBytes)
	if code != 201 {
		t.Fatalf("receipt upload: %d %v", code, meta)
	}
	aid := meta["id"].(string)
	if c, _ := e.upload(peer, "expense", id, "fake.png", pngBytes); c != 404 {
		t.Errorf("a peer must not attach to someone else's claim, got %d", c)
	}
	if c, _ := e.upload(mgr, "expense", id, "fake.png", pngBytes); c != 403 {
		t.Errorf("a reviewer must not attach to someone else's claim, got %d", c)
	}
	if resp, _ := e.download(peer, aid); resp.StatusCode != 404 {
		t.Errorf("a peer must not read the receipt, got %d", resp.StatusCode)
	}
	for _, tok := range []string{emp, mgr} {
		if resp, body := e.download(tok, aid); resp.StatusCode != 200 || !bytes.Equal(body, pngBytes) {
			t.Errorf("claimant and manager can read it: %d", resp.StatusCode)
		}
	}
}

// MaxSize is re-exported so the test reads naturally.
const MaxSize = 10 << 20
