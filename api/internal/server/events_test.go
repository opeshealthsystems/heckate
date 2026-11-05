package server

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/opeshealthsystems/heckate/api/internal/platform"
)

// Every event the code emits must belong to a domain the JetStream stream captures; otherwise the
// event would be published by the relay and silently dropped by the broker.
func TestEveryEmittedEventHasAStreamDomain(t *testing.T) {
	domains := map[string]bool{}
	for _, d := range platform.EventDomains {
		domains[d] = true
	}
	emit := regexp.MustCompile(`\.Emit\(\s*[a-zA-Z.()]+,\s*"([a-z_]+)\.([a-z0-9_.]+)"`)
	dynamic := regexp.MustCompile(`\.Emit\(\s*[a-zA-Z.()]+,\s*"([a-z_]+)\."\s*\+`)
	call := regexp.MustCompile(`\.Emit\(`)
	found, total := 0, 0
	err := filepath.Walk("..", func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		b, _ := os.ReadFile(path)
		total += len(call.FindAllString(string(b), -1))
		for _, m := range emit.FindAllStringSubmatch(string(b), -1) {
			found++
			if !domains[m[1]] {
				t.Errorf("%s emits %q but %q is not in platform.EventDomains", path, m[1]+"."+m[2], m[1])
			}
		}
		for _, m := range dynamic.FindAllStringSubmatch(string(b), -1) {
			found++
			if !domains[m[1]] {
				t.Errorf("%s emits %q.* but %q is not in platform.EventDomains", path, m[1], m[1])
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// Every call must have been understood: an Emit with a computed subject would escape the check.
	if found != total || total < 20 {
		t.Errorf("recognised %d of %d Emit calls; use a literal \"domain.verb\" subject (or \"domain.\"+suffix)", found, total)
	}
}
