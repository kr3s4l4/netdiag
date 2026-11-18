package update

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kr3s4l4/netdiag/internal/config"
)

func testKey() (seed, keyID []byte, publicKey string) {
	seed = bytes.Repeat([]byte{0x11}, 32)
	keyID = []byte("netdiag1")
	publicKey, err := PublicKeyFromSeed(seed, keyID)
	if err != nil {
		panic(err)
	}
	return seed, keyID, publicKey
}

func elfFixture(tag string) []byte {
	return append([]byte{0x7f, 'E', 'L', 'F'}, []byte(tag)...)
}

func signedManifestBytes(t *testing.T, seed, keyID []byte, serverURL, version string, artifact []byte) ([]byte, string) {
	t.Helper()
	sum := sha256.Sum256(artifact)
	rel := &Release{
		Schema:     ManifestSchema,
		Version:    version,
		URL:        serverURL + "/netdiag",
		Checksum:   hex.EncodeToString(sum[:]),
		ReleasedAt: time.Now().UTC().Truncate(time.Second),
		Notes:      "versión de pruebas",
	}
	if err := SignRelease(rel, seed, keyID); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(rel)
	if err != nil {
		t.Fatal(err)
	}
	return data, rel.Checksum
}

// dynamicServer serves mutable manifest/artifact bytes so tests can simulate
// new releases without restarting the listener.
func dynamicServer(manifest, artifact *[]byte) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/manifest.json":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(*manifest)
		case "/netdiag":
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write(*artifact)
		default:
			http.NotFound(w, r)
		}
	}))
}

func testManager(t *testing.T, serverURL, publicKey, exe, dataDir string) *Manager {
	t.Helper()
	cfg := config.Update{
		Enabled:            true,
		ManifestURL:        serverURL + "/manifest.json",
		PublicKey:          publicKey,
		CheckIntervalHours: 24,
		DownloadTimeout:    config.Duration(5 * time.Second),
		MaxDownloadMB:      16,
	}
	m, err := New(Options{
		Config:         cfg,
		CurrentVersion: "1.5.0",
		Executable:     exe,
		DataDir:        dataDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func testPaths(t *testing.T) (exe, dataDir string, original []byte) {
	t.Helper()
	dir := t.TempDir()
	exe = filepath.Join(dir, "netdiag")
	original = elfFixture("original")
	if err := os.WriteFile(exe, original, 0o755); err != nil {
		t.Fatal(err)
	}
	dataDir = filepath.Join(dir, "data")
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		t.Fatal(err)
	}
	return exe, dataDir, original
}

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.5.1", "1.5.0", 1},
		{"v1.6.0", "1.5.9", 1},
		{"1.5.0", "1.5.0", 0},
		{"1.5.0", "1.5.0-rc.1", 1},
		{"1.5.0-rc.2", "1.5.0-rc.10", -1},
		{"dev", "1.0.0", -1},
	}
	for _, tc := range cases {
		got := CompareVersions(tc.a, tc.b)
		if (got > 0) != (tc.want > 0) || (got == 0) != (tc.want == 0) {
			t.Errorf("CompareVersions(%q,%q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestParseManifestRejectsTampering(t *testing.T) {
	seed, keyID, publicKey := testKey()
	artifact := elfFixture("v160")
	manifest, _ := signedManifestBytes(t, seed, keyID, "https://updates.example.net", "1.6.0", artifact)
	if rel, err := ParseManifest(manifest, publicKey); err != nil || rel.Version != "1.6.0" {
		t.Fatalf("manifiesto válido rechazado: %+v %v", rel, err)
	}

	var raw map[string]any
	if err := json.Unmarshal(manifest, &raw); err != nil {
		t.Fatal(err)
	}
	raw["version"] = "9.9.9"
	tampered, _ := json.Marshal(raw)
	if _, err := ParseManifest(tampered, publicKey); err == nil {
		t.Fatal("se aceptó un manifiesto modificado")
	}
	otherSeed := bytes.Repeat([]byte{0x22}, 32)
	otherPublic, err := PublicKeyFromSeed(otherSeed, keyID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseManifest(manifest, otherPublic); err == nil {
		t.Fatal("se aceptó un manifiesto firmado por otra clave")
	}
}

func TestCheckApplyCommitAndRollback(t *testing.T) {
	seed, keyID, publicKey := testKey()
	exe, dataDir, _ := testPaths(t)
	var manifest, artifact = []byte("{}"), elfFixture("placeholder")
	srv := dynamicServer(&manifest, &artifact)
	defer srv.Close()

	artifact = elfFixture("v160")
	manifest, checksum := signedManifestBytes(t, seed, keyID, srv.URL, "1.6.0", artifact)
	m := testManager(t, srv.URL, publicKey, exe, dataDir)

	check, err := m.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !check.Available || check.LatestVersion != "1.6.0" {
		t.Fatalf("check inesperado: %+v", check)
	}
	result, err := m.Apply(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if result.ToVersion != "1.6.0" || result.FromVersion != "1.5.0" {
		t.Fatalf("apply inesperado: %+v", result)
	}
	installed, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	installedSum := sha256.Sum256(installed)
	if got := hex.EncodeToString(installedSum[:]); got != checksum {
		t.Fatalf("el binario instalado no coincide con el checksum: %s != %s", got, checksum)
	}
	if m.Pending() == nil {
		t.Fatal("no se escribió el marcador de rollback")
	}
	commit, err := m.CommitPending()
	if err != nil || !commit.Confirmed {
		t.Fatalf("confirmación fallida: %+v %v", commit, err)
	}
	if m.Pending() != nil {
		t.Fatal("el marcador no se eliminó al confirmar")
	}
	if _, err := os.Stat(result.BackupPath); !os.IsNotExist(err) {
		t.Fatal("la copia de seguridad no se limpió al confirmar")
	}

	newArtifact := elfFixture("v170")
	artifact = newArtifact
	manifest, _ = signedManifestBytes(t, seed, keyID, srv.URL, "1.7.0", newArtifact)
	if _, err := m.Apply(context.Background(), "1.7.0"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Rollback(); err != nil {
		t.Fatal(err)
	}
	restored, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(restored, installed) {
		t.Fatal("el rollback no restauró la versión anterior")
	}
	if m.Pending() != nil {
		t.Fatal("el rollback no eliminó el marcador")
	}
}

func TestApplyRejectsChecksumMismatch(t *testing.T) {
	seed, keyID, publicKey := testKey()
	exe, dataDir, original := testPaths(t)
	expected := elfFixture("expected")
	var manifest = []byte("{}")
	var artifact = elfFixture("tampered")
	srv := dynamicServer(&manifest, &artifact)
	defer srv.Close()
	manifest, _ = signedManifestBytes(t, seed, keyID, srv.URL, "1.6.0", expected)
	m := testManager(t, srv.URL, publicKey, exe, dataDir)

	if _, err := m.Apply(context.Background(), ""); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("se esperaba error de checksum, se obtuvo %v", err)
	}
	content, err := os.ReadFile(exe)
	if err != nil || !bytes.Equal(content, original) {
		t.Fatal("el binario original se modificó pese a fallar la verificación")
	}
}

func TestCommitPendingRollsBackCorruptBinary(t *testing.T) {
	seed, keyID, publicKey := testKey()
	exe, dataDir, original := testPaths(t)
	artifact := elfFixture("v160")
	var manifest = []byte("{}")
	srv := dynamicServer(&manifest, &artifact)
	defer srv.Close()
	manifest, _ = signedManifestBytes(t, seed, keyID, srv.URL, "1.6.0", artifact)
	m := testManager(t, srv.URL, publicKey, exe, dataDir)

	if _, err := m.Apply(context.Background(), ""); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(exe, elfFixture("corrupt"), 0o755); err != nil {
		t.Fatal(err)
	}
	commit, err := m.CommitPending()
	if err != nil || !commit.RolledBack || commit.PreviousVersion != "1.5.0" {
		t.Fatalf("rollback automático inesperado: %+v %v", commit, err)
	}
	content, err := os.ReadFile(exe)
	if err != nil || !bytes.Equal(content, original) {
		t.Fatal("no se restauró el binario original")
	}
}
