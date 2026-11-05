package projects

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func png(n int) string {
	b := append(append([]byte{}, pngMagic...), make([]byte, n)...)
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(b)
}

func TestValidSignature(t *testing.T) {
	if !ValidSignature(png(100)) {
		t.Error("small PNG should be accepted")
	}
	for name, s := range map[string]string{
		"not a data url": "hello",
		"svg":            "data:image/svg+xml;base64,PHN2Zy8+",
		"not png bytes":  "data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte("<script>alert(1)</script>")),
		"bad base64":     "data:image/png;base64,@@@",
		"too big":        png(300 * 1024),
		"empty":          "",
	} {
		if ValidSignature(s) {
			t.Errorf("%s must be rejected", name)
		}
	}
	_ = strings.Repeat
}

func TestDeviceTime(t *testing.T) {
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	if got, err := DeviceTime("", now); err != nil || !got.Equal(now) {
		t.Errorf("empty should default to now: %v %v", got, err)
	}
	if _, err := DeviceTime("2026-10-18T08:00:00Z", now); err != nil {
		t.Errorf("yesterday (offline work) must be accepted: %v", err)
	}
	if _, err := DeviceTime("2026-10-19T12:03:00Z", now); err != nil {
		t.Errorf("3 minutes of clock skew must be tolerated: %v", err)
	}
	for name, v := range map[string]string{"future": "2026-10-19T13:00:00Z", "too old": "2026-09-01T00:00:00Z", "garbage": "yesterday"} {
		if _, err := DeviceTime(v, now); err == nil {
			t.Errorf("%s must be rejected", name)
		}
	}
}

func TestValidCoord(t *testing.T) {
	if !ValidCoord(4.05, 9.7) || ValidCoord(91, 0) || ValidCoord(0, 181) {
		t.Error("coordinate validation wrong")
	}
}
