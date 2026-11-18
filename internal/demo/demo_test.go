package demo

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kr3s4l4/netdiag/internal/cryptoutil"
	"github.com/kr3s4l4/netdiag/internal/model"
	"github.com/kr3s4l4/netdiag/internal/store"
)

func fixedNow() time.Time {
	return time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
}

func opts(dir string) Options {
	return Options{Seed: 7, Sites: 2, Hosts: 40, DataDir: dir, Now: fixedNow()}
}

func openDemo(t *testing.T, dir string) *store.Store {
	t.Helper()
	box, err := cryptoutil.LoadOrCreate(filepath.Join(dir, "secret.key"))
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(filepath.Join(dir, "netdiag.db"), box)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func TestGenerateCreatesMarkerAndSyntheticData(t *testing.T) {
	dir := t.TempDir()
	if Active(dir) {
		t.Fatal("el directorio vacío se marcó como demo")
	}
	res, err := Generate(opts(dir))
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if Active(dir) {
		t.Fatal("el directorio base se marcó como demo; la BD real debe quedar sin marca")
	}
	if !Active(filepath.Join(dir, "demo")) {
		t.Fatal("no se escribió el marcador demo")
	}
	if res.Sites != 2 || res.Hosts != 40 || res.DBPath != filepath.Join(dir, "demo", "netdiag.db") {
		t.Fatalf("resultado inesperado: %+v", res)
	}
	if res.ManagedHosts >= res.Hosts {
		t.Fatalf("los tags guest/excluded no redujeron los hosts gestionados: %d/%d", res.ManagedHosts, res.Hosts)
	}

	st := openDemo(t, filepath.Join(dir, "demo"))
	sites, err := st.ListSites()
	if err != nil || len(sites) != 2 {
		t.Fatalf("sedes: %d, %v", len(sites), err)
	}
	hosts, err := st.ListAllHosts(0)
	if err != nil || len(hosts) != 40 {
		t.Fatalf("hosts: %d, %v", len(hosts), err)
	}
	marked := 0
	for _, h := range hosts {
		if h.ID == 0 || h.Identity == "" {
			t.Fatalf("host sin identidad: %+v", h)
		}
		if len(h.Ports) == 0 {
			t.Fatalf("host sin puertos: %s", h.IP)
		}
		for _, tag := range h.Tags {
			if tag == "demo" {
				marked++
			}
		}
	}
	if marked != len(hosts) {
		t.Fatalf("solo %d/%d hosts llevan la etiqueta demo", marked, len(hosts))
	}
	alerts, err := st.ListAlerts(model.AlertFilter{Page: 1, Limit: 500})
	if err != nil || len(alerts.Items) == 0 {
		t.Fatalf("alertas: %d, %v", len(alerts.Items), err)
	}

	raw, err := os.ReadFile(filepath.Join(dir, "demo", MarkerName))
	if err != nil {
		t.Fatal(err)
	}
	var m marker
	if err := json.Unmarshal(raw, &m); err != nil || !m.Demo || m.Seed != 7 {
		t.Fatalf("marcador inesperado: %s (%v)", raw, err)
	}
}

func TestGenerateIsDeterministic(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	ra, err := Generate(opts(a))
	if err != nil {
		t.Fatal(err)
	}
	rb, err := Generate(opts(b))
	if err != nil {
		t.Fatal(err)
	}
	ra.GeneratedAt, rb.GeneratedAt = time.Time{}, time.Time{}
	ra.DBPath, rb.DBPath = "", ""
	ra.DemoDir, rb.DemoDir = "", ""
	left, _ := json.Marshal(ra)
	right, _ := json.Marshal(rb)
	if string(left) != string(right) {
		t.Fatalf("resultados distintos con la misma semilla:\n%s\n%s", left, right)
	}
	ma, _ := os.ReadFile(filepath.Join(a, "demo", MarkerName))
	mb, _ := os.ReadFile(filepath.Join(b, "demo", MarkerName))
	if string(ma) != string(mb) {
		t.Fatal("los marcadores difieren")
	}
}

func TestGenerateDoesNotTouchRealDatabase(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "netdiag.db")
	if err := os.WriteFile(real, []byte("base real"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Generate(opts(dir)); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(real)
	if err != nil || string(got) != "base real" {
		t.Fatalf("la base real se modificó: %q (%v)", got, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "demo", dbName)); err != nil {
		t.Fatalf("no se creó la base demo: %v", err)
	}
}
