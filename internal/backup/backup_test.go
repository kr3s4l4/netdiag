package backup

import (
	"archive/zip"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kr3s4l4/netdiag/internal/config"
	"github.com/kr3s4l4/netdiag/internal/cryptoutil"
	"github.com/kr3s4l4/netdiag/internal/store"
)

func newTestManager(t *testing.T) (*Manager, *store.Store, *config.Config, string) {
	t.Helper()
	dir := t.TempDir()
	cfg := config.Defaults()
	cfg.Storage.DataDir = filepath.Join(dir, "data")
	cfg.Storage.DB = filepath.Join(cfg.Storage.DataDir, "netdiag.db")
	cfg.Storage.BackupDir = filepath.Join(cfg.Storage.DataDir, "backups")
	cfg.Storage.BackupEnabled = true
	cfg.Storage.BackupKeep = 2
	if err := cfg.EnsureDirs(); err != nil {
		t.Fatal(err)
	}
	box, err := cryptoutil.LoadOrCreate(filepath.Join(cfg.Storage.DataDir, "secret.key"))
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(cfg.Storage.DB, box)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if _, err := st.CreateUser("admin", "hash", "admin"); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte("server:\n  listen: 127.0.0.1:8080\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return New(st, cfg, cfgPath, nil), st, cfg, dir
}

func TestCreateToWritesCompleteZip(t *testing.T) {
	mgr, _, cfg, dir := newTestManager(t)
	path := filepath.Join(dir, "manual.zip")
	entry, err := mgr.CreateTo(path)
	if err != nil {
		t.Fatal(err)
	}
	if entry.Name != "manual.zip" || entry.Size <= 0 {
		t.Fatalf("entrada inesperada: %+v", entry)
	}
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	names := map[string]bool{}
	for _, f := range zr.File {
		names[f.Name] = true
	}
	for _, want := range []string{"manifest.json", "netdiag.db", "config.yaml", "secret.key"} {
		if !names[want] {
			t.Errorf("falta %s en el ZIP (contenido: %v)", want, names)
		}
	}

	// The embedded database must be readable.
	var dbFile *zip.File
	for _, f := range zr.File {
		if f.Name == "netdiag.db" {
			dbFile = f
		}
	}
	rc, err := dbFile.Open()
	if err != nil {
		t.Fatal(err)
	}
	extracted := filepath.Join(dir, "extracted.db")
	out, err := os.Create(extracted)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := out.ReadFrom(rc); err != nil {
		t.Fatal(err)
	}
	_ = out.Close()
	_ = rc.Close()
	snap, err := store.Open(extracted, nil)
	if err != nil {
		t.Fatalf("la base de datos del ZIP no abre: %v", err)
	}
	defer snap.Close()
	n, err := snap.CountUsers()
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("usuarios en la copia: %d", n)
	}

	// Secret key must match the original.
	keyData, err := os.ReadFile(filepath.Join(cfg.Storage.DataDir, "secret.key"))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range zr.File {
		if f.Name != "secret.key" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		buf, err := io.ReadAll(rc)
		if err != nil {
			t.Fatal(err)
		}
		_ = rc.Close()
		if string(buf) != string(keyData) {
			t.Fatal("la clave del ZIP no coincide con la original")
		}
	}
}

func TestCreateAppliesRetention(t *testing.T) {
	mgr, _, _, _ := newTestManager(t)
	for i := 0; i < 4; i++ {
		if _, err := mgr.Create(); err != nil {
			t.Fatal(err)
		}
		// Guarantee distinct modification times for the ordering.
		time.Sleep(10 * time.Millisecond)
	}
	items, err := mgr.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("retención: %d copias, se esperaban 2", len(items))
	}
	for i := 1; i < len(items); i++ {
		if items[i].CreatedAt.After(items[i-1].CreatedAt) {
			t.Fatal("las copias no están ordenadas de más reciente a más antigua")
		}
	}
}

func TestPathRejectsTraversal(t *testing.T) {
	mgr, _, _, dir := newTestManager(t)
	if _, err := mgr.Create(); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"../evil.zip", "..", "sub/dir.zip", "x.txt", ""} {
		if _, err := mgr.Path(name); !errors.Is(err, ErrInvalidName) {
			t.Errorf("nombre %q: se esperaba ErrInvalidName, se obtuvo %v", name, err)
		}
	}
	if _, err := mgr.Path(filepath.Base(dir)); !errors.Is(err, ErrInvalidName) {
		t.Errorf("nombre sin .zip aceptado: %v", err)
	}
}

func TestDeleteRemovesBackup(t *testing.T) {
	mgr, _, _, _ := newTestManager(t)
	entry, err := mgr.Create()
	if err != nil {
		t.Fatal(err)
	}
	if err := mgr.Delete(entry.Name); err != nil {
		t.Fatal(err)
	}
	items, err := mgr.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("quedan %d copias tras borrar", len(items))
	}
	if err := mgr.Delete(entry.Name); err == nil {
		t.Fatal("borrar dos veces no devolvió error")
	}
}

func TestListOnMissingDirIsEmpty(t *testing.T) {
	mgr, _, cfg, _ := newTestManager(t)
	cfg.Storage.BackupDir = filepath.Join(cfg.Storage.DataDir, "does-not-exist")
	items, err := mgr.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("se esperaban 0 copias, hay %d", len(items))
	}
}
