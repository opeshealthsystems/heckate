package platform

import (
	"testing"
	"time"
)

func TestDurationOr(t *testing.T) {
	cases := map[string]time.Duration{
		"1h": time.Hour, "30m": 30 * time.Minute, "off": 0, "0": 0, " off ": 0,
		"": time.Hour, "banana": time.Hour, "10s": time.Hour, "-5m": time.Hour, // a typo or an unsafe value must never disable or hammer the job
	}
	for in, want := range cases {
		if got := durationOr(in, time.Hour); got != want {
			t.Errorf("durationOr(%q) = %v, want %v", in, got, want)
		}
	}
}
