package projects

import (
	"bytes"
	"encoding/base64"
	"net/http"
	"strings"
	"time"

	"github.com/opeshealthsystems/heckate/api/internal/platform"
)

const maxSignature = 200 * 1024

var pngMagic = []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}

// ValidSignature accepts only a PNG data URL of reasonable size, so a signature field cannot be
// used to store arbitrary content.
func ValidSignature(s string) bool {
	rest, ok := strings.CutPrefix(s, "data:image/png;base64,")
	if !ok || len(rest) > maxSignature*4/3+4 {
		return false
	}
	b, err := base64.StdEncoding.DecodeString(rest)
	return err == nil && len(b) <= maxSignature && bytes.HasPrefix(b, pngMagic)
}

// DeviceTime resolves the time a field action happened. Offline devices send the real time of the
// action; it must be plausible: not in the future (beyond clock skew) and not older than 14 days.
func DeviceTime(raw string, now time.Time) (time.Time, error) {
	if raw == "" {
		return now, nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, platform.Err(http.StatusUnprocessableEntity, "Invalid time", "at must be an RFC 3339 timestamp")
	}
	if t.After(now.Add(5 * time.Minute)) {
		return time.Time{}, platform.Err(http.StatusUnprocessableEntity, "Invalid time", "at is in the future")
	}
	if t.Before(now.AddDate(0, 0, -14)) {
		return time.Time{}, platform.Err(http.StatusUnprocessableEntity, "Invalid time", "at is older than 14 days; ask a manager to correct the record")
	}
	return t, nil
}

// ValidCoord checks a GPS coordinate pair.
func ValidCoord(lat, lng float64) bool { return lat >= -90 && lat <= 90 && lng >= -180 && lng <= 180 }
