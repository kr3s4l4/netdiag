package security

import (
	"crypto/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kr3s4l4/netdiag/internal/update"
)

func signedEvidence(t *testing.T, dir string) {
	t.Helper()
	seed := make([]byte, 32)
	if _, err := rand.Read(seed); err != nil {
		t.Fatal(err)
	}
	keyID := []byte("NETDIAG1")
	pub, err := update.PublicKeyFromSeed(seed, keyID)
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"manifest.json":           `{"schema":"netdiag/update/v1","version":"9.9.9-dryrun"}`,
		"sbom.cdx.json":           `{"bomFormat":"CycloneDX","components":[{"name":"golang.org/x/net","version":"1.0.0"},{"name":"golang.org/x/net","version":"1.0.0"},{"name":"golang.org/x/text","version":"2.0.0"}]}`,
		"provenance.intoto.jsonl": `{"buildType":"https://slsa.dev/provenance/v1"}`,
	}
	for name, body := range files {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		sig, err := update.SignMinisign([]byte(body), seed, keyID, "untrusted comment: prueba")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path+".minisig", []byte(sig), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "release.pub"), []byte("untrusted comment: prueba\n"+pub+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestBuildDryRunIsDeterministicAndOffline(t *testing.T) {
	dir := t.TempDir()
	signedEvidence(t, dir)

	first, err := BuildDryRun(RunbookOptions{Dir: dir, Out: filepath.Join(dir, "out1")})
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildDryRun(RunbookOptions{Dir: dir, Out: filepath.Join(dir, "out2")})
	if err != nil {
		t.Fatal(err)
	}
	if first.Version != "9.9.9-dryrun" || first.Version != second.Version {
		t.Fatalf("versión inesperada: %#v / %#v", first.Version, second.Version)
	}
	if first.SBOMComponents != 2 {
		t.Errorf("componentes=%d, se esperaban 2", first.SBOMComponents)
	}
	if first.T0 != DryRunT0 || first.Deadline24h != "2026-01-02T00:00:00Z" || first.Deadline72h != "2026-01-04T00:00:00Z" {
		t.Fatalf("plazos inesperados: %#v", first)
	}
	if len(first.Files) != 4 || len(second.Files) != 4 {
		t.Fatalf("faltan ficheros: %#v", first.Files)
	}
	for i := range first.Files {
		a, err := os.ReadFile(first.Files[i])
		if err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(second.Files[i])
		if err != nil {
			t.Fatal(err)
		}
		if string(a) != string(b) {
			t.Errorf("salida no determinista: %s", first.Files[i])
		}
	}

	draft, err := os.ReadFile(filepath.Join(dir, "out1", "aviso-24h-dryrun.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"NO ENVIADO", "F41 real sigue bloqueado", "9.9.9-dryrun", "sha256"} {
		if !strings.Contains(string(draft), want) {
			t.Errorf("falta %q en el borrador 24 h", want)
		}
	}
	marker, err := os.ReadFile(filepath.Join(dir, "out1", "NO-ENVIADO.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(marker), "NO enviar a ENISA") {
		t.Errorf("marcador inesperado: %s", marker)
	}
}

func TestBuildDryRunRejectsTamperedEvidence(t *testing.T) {
	dir := t.TempDir()
	signedEvidence(t, dir)
	path := filepath.Join(dir, "sbom.cdx.json")
	if err := os.WriteFile(path, []byte(`{"components":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := BuildDryRun(RunbookOptions{Dir: dir, Out: filepath.Join(dir, "out")}); err == nil {
		t.Fatal("se aceptó un SBOM modificado")
	}
}

func TestBuildDryRunRequiresOutput(t *testing.T) {
	if _, err := BuildDryRun(RunbookOptions{}); err == nil {
		t.Fatal("se aceptó un ensayo sin directorios")
	}
}
