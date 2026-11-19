package webui

import (
	"os"
	"strings"
	"testing"
)

// TestLoginFooterUsesAuthStatusVersion guards the value rendered on the
// login/setup card. It must come from AuthStatus.Version, never from a
// hardcoded literal in the embedded JavaScript.
func TestLoginFooterUsesAuthStatusVersion(t *testing.T) {
	data, err := os.ReadFile("dist/app.js")
	if err != nil {
		t.Fatalf("leer app.js: %v", err)
	}
	want := `NetDiag ${esc((st && st.version) || '')}`
	if !strings.Contains(string(data), want) {
		t.Fatalf("el footer del login no usa st.version: %q", want)
	}
}
