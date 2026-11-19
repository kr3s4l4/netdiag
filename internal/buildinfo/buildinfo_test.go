package buildinfo

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// TestDefaultVersionMatchesMakefile keeps a single product version source.
// The Makefile is the value used by make build and release-secure; the default
// here is what plain "go build" and "go test" report when no -ldflags are set.
func TestDefaultVersionMatchesMakefile(t *testing.T) {
	data, err := os.ReadFile("../../Makefile")
	if err != nil {
		t.Fatalf("leer Makefile: %v", err)
	}
	re := regexp.MustCompile(`(?m)^VERSION\s+\?=\s*(\S+)`)
	m := re.FindSubmatch(data)
	if m == nil {
		t.Fatal("VERSION ?= no encontrada en Makefile")
	}
	want := strings.TrimSpace(string(m[1]))
	if Version != want {
		t.Fatalf("buildinfo.Version=%q, Makefile VERSION=%q", Version, want)
	}
}
