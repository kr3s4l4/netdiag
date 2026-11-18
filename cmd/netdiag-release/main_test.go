package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kr3s4l4/netdiag/internal/update"
)

func testKeys(t *testing.T) (dir, signing, pub string) {
	t.Helper()
	dir = t.TempDir()
	if err := keygenCmd([]string{"--out", dir}); err != nil {
		t.Fatalf("keygen: %v", err)
	}
	signing = filepath.Join(dir, "signing.json")
	pub = filepath.Join(dir, "release.pub")
	info, err := os.Stat(signing)
	if err != nil {
		t.Fatalf("stat signing: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("permisos de signing.json: %o, quería 600", info.Mode().Perm())
	}
	return dir, signing, pub
}

func TestLoadMaterialRechazaDatosInvalidos(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(bad, []byte(`{"seed":"corta","key_id":"00"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := loadMaterial(bad); err == nil {
		t.Fatal("se aceptó una semilla inválida")
	}
}

func TestSignVerifyCmd(t *testing.T) {
	dir, signing, pub := testKeys(t)
	in := filepath.Join(dir, "SHA256SUMS")
	sig := in + ".minisig"
	data := "abc  netdiag-linux-amd64\n"
	if err := os.WriteFile(in, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := signCmd([]string{"--in", in, "--out", sig, "--seed-file", signing,
		"--comment", "version:1.5.1 sha256:abc"}); err != nil {
		t.Fatalf("sign: %v", err)
	}
	if err := verifyCmd([]string{"--in", in, "--sig", sig, "--key", pub}); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if err := os.WriteFile(in, []byte(data+"x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := verifyCmd([]string{"--in", in, "--sig", sig, "--key", pub}); err == nil {
		t.Fatal("se verificó un archivo manipulado")
	}
}

func TestManifestCmdYVerify(t *testing.T) {
	dir, signing, pub := testKeys(t)
	out := filepath.Join(dir, "manifest.json")
	checksum := strings.Repeat("a", 64)
	if err := manifestCmd([]string{
		"--version", "1.5.1",
		"--url", "https://updates.example/releases/1.5.1/netdiag-linux-amd64",
		"--checksum", checksum,
		"--notes", "NetDiag 1.5.1",
		"--released-at", "2026-11-18T00:00:00Z",
		"--seed-file", signing,
		"--out", out,
	}); err != nil {
		t.Fatalf("manifest: %v", err)
	}
	if err := verifyManifestCmd([]string{"--in", out, "--key", pub}); err != nil {
		t.Fatalf("verify-manifest: %v", err)
	}
	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	rel, err := update.ParseManifest(raw, strings.TrimSpace(readKey(t, pub)))
	if err != nil {
		t.Fatalf("ParseManifest: %v", err)
	}
	if rel.Schema != update.ManifestSchema || rel.Version != "1.5.1" || rel.Checksum != checksum {
		t.Fatalf("manifiesto inesperado: %+v", rel)
	}
}

func readKey(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) < 2 {
		t.Fatalf("clave pública sin cuerpo: %q", raw)
	}
	return lines[len(lines)-1]
}
