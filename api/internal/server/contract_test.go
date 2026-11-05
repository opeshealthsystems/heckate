package server

import (
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The OpenAPI file is the contract. This test fails when a route exists in code but not in the
// contract, or the other way round, so SDKs generated from it cannot silently drift.
func TestRoutesMatchOpenAPIContract(t *testing.T) {
	raw, err := os.ReadFile("../../openapi/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	pathRe := regexp.MustCompile(`^  (/[^\s:]+):\s*$`)
	methodRe := regexp.MustCompile(`^    (get|post|put|patch|delete):\s*$`)
	doc := map[string]bool{}
	cur := ""
	inPaths := false
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(line, "paths:") {
			inPaths = true
			continue
		}
		if inPaths && strings.HasPrefix(line, "components:") {
			break
		}
		if m := pathRe.FindStringSubmatch(line); m != nil {
			cur = m[1]
		} else if m := methodRe.FindStringSubmatch(line); m != nil && cur != "" {
			doc[strings.ToUpper(m[1])+" "+cur] = true
		}
	}
	code := map[string]bool{}
	for _, p := range Patterns() {
		code[p] = true
	}
	var missingInDoc, missingInCode []string
	for p := range code {
		if !doc[p] {
			missingInDoc = append(missingInDoc, p)
		}
	}
	for p := range doc {
		if !code[p] {
			missingInCode = append(missingInCode, p)
		}
	}
	sort.Strings(missingInDoc)
	sort.Strings(missingInCode)
	if len(missingInDoc) > 0 {
		t.Errorf("routes in code but not in openapi.yaml:\n  %s", strings.Join(missingInDoc, "\n  "))
	}
	if len(missingInCode) > 0 {
		t.Errorf("operations in openapi.yaml with no route in code:\n  %s", strings.Join(missingInCode, "\n  "))
	}
	if len(doc) < 100 {
		t.Errorf("parsed only %d operations from the contract; the parser may be broken", len(doc))
	}
}
