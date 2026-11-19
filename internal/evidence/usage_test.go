package evidence

import (
	"archive/zip"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kr3s4l4/netdiag/internal/container"
	"github.com/kr3s4l4/netdiag/internal/cryptoutil"
	"github.com/kr3s4l4/netdiag/internal/model"
	"github.com/kr3s4l4/netdiag/internal/store"
	"github.com/kr3s4l4/netdiag/internal/update"
)

func testStore(t *testing.T, dir string) *store.Store {
	t.Helper()
	box, err := cryptoutil.LoadOrCreate(filepath.Join(dir, "secret.key"))
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(filepath.Join(dir, "test.db"), box)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func testSeedFile(t *testing.T, dir string) (string, string) {
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
	material := map[string]string{
		"seed":   base64.StdEncoding.EncodeToString(seed),
		"key_id": hex.EncodeToString(keyID),
	}
	raw, err := json.Marshal(material)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "signing.json")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	return path, pub
}

func addHost(t *testing.T, st *store.Store, ip string, siteID int64, now time.Time) {
	t.Helper()
	_, _, _, _, err := st.UpsertHost(&model.Host{
		SiteID:     siteID,
		IP:         ip,
		DeviceType: model.DevUnknown,
		Status:     model.StatusUp,
		FirstSeen:  now.Add(-time.Hour),
		LastSeen:   now,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestBuildUsageIsSignedAndAnonymous(t *testing.T) {
	dir := t.TempDir()
	st := testStore(t, dir)
	now := time.Now().UTC()

	if _, err := st.CreateUser("admin", "hash", "admin"); err != nil {
		t.Fatal(err)
	}
	initialSites, err := st.ListSites()
	if err != nil {
		t.Fatal(err)
	}
	siteA, err := st.CreateSite("Sede Secreta Norte", "no debe aparecer")
	if err != nil {
		t.Fatal(err)
	}
	siteB, err := st.CreateSite("Planta Privada", "no debe aparecer")
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 6; i++ {
		addHost(t, st, "192.168.7."+string(rune('0'+i)), siteA.ID, now)
	}
	for i := 1; i <= 2; i++ {
		addHost(t, st, "10.9.8."+string(rune('0'+i)), siteB.ID, now)
	}

	seedFile, publicKey := testSeedFile(t, dir)
	out := filepath.Join(dir, "usage.ndc")
	entry, err := BuildUsage(st, UsageOptions{
		SeedFile: seedFile, Out: out, DataDir: dir,
		Version: "1.5.2-test", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !entry.Valid || entry.Path != out {
		t.Fatalf("entrada inesperada: %#v", entry)
	}
	report, err := container.Verify(out, publicKey)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Valid {
		t.Fatalf("contenedor no válido: %#v", report.Signature)
	}

	payload := readUsagePayload(t, out)
	for _, leak := range []string{"Sede Secreta Norte", "Planta Privada", "192.168.7.", "10.9.8.", "admin", "hash"} {
		if strings.Contains(payload, leak) {
			t.Errorf("la evidencia filtra %q", leak)
		}
	}
	var doc UsageReport
	if err := json.Unmarshal([]byte(payload), &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Schema != UsageSchema || doc.Version != "1.5.2-test" {
		t.Fatalf("cabecera inesperada: %#v", doc)
	}
	if doc.Sites != len(initialSites)+2 {
		t.Errorf("sites=%d, se esperaban %d", doc.Sites, len(initialSites)+2)
	}
	if doc.ManagedHostBands["6-10"] != 1 || doc.ManagedHostBands["1-5"] != 1 {
		t.Errorf("bandas inesperadas: %#v", doc.ManagedHostBands)
	}
}

func TestBuildUsageRequiresSeed(t *testing.T) {
	dir := t.TempDir()
	st := testStore(t, dir)
	out := filepath.Join(dir, "usage.ndc")
	if _, err := BuildUsage(st, UsageOptions{Out: out}); err == nil {
		t.Fatal("se aceptó una exportación sin clave de firma")
	}
}

func readUsagePayload(t *testing.T, path string) string {
	t.Helper()
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	for _, f := range zr.File {
		if f.Name != container.PayloadPrefix+"usage.json" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		defer rc.Close()
		raw, err := io.ReadAll(rc)
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	t.Fatal("usage.json no encontrado en el contenedor")
	return ""
}
