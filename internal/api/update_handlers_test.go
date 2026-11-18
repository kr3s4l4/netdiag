package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kr3s4l4/netdiag/internal/config"
	"github.com/kr3s4l4/netdiag/internal/model"
	"github.com/kr3s4l4/netdiag/internal/update"
)

func TestUpdateHandlersCheckApplyAndRollback(t *testing.T) {
	seed := bytes.Repeat([]byte{0x31}, 32)
	keyID := []byte("netdiag1")
	publicKey, err := update.PublicKeyFromSeed(seed, keyID)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	exe := filepath.Join(dir, "netdiag")
	original := append([]byte{0x7f, 'E', 'L', 'F'}, []byte("original")...)
	if err := os.WriteFile(exe, original, 0o755); err != nil {
		t.Fatal(err)
	}
	artifact := append([]byte{0x7f, 'E', 'L', 'F'}, []byte("v160")...)
	sum := sha256.Sum256(artifact)
	var manifest []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/manifest.json":
			_, _ = w.Write(manifest)
		case "/netdiag":
			_, _ = w.Write(artifact)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	rel := &update.Release{
		Schema:     update.ManifestSchema,
		Version:    "1.6.0",
		URL:        srv.URL + "/netdiag",
		Checksum:   hex.EncodeToString(sum[:]),
		ReleasedAt: time.Now().UTC().Truncate(time.Second),
	}
	if err := update.SignRelease(rel, seed, keyID); err != nil {
		t.Fatal(err)
	}
	manifest, err = json.Marshal(rel)
	if err != nil {
		t.Fatal(err)
	}

	cfg := config.Update{
		Enabled:            true,
		ManifestURL:        srv.URL + "/manifest.json",
		PublicKey:          publicKey,
		CheckIntervalHours: 24,
		DownloadTimeout:    config.Duration(5 * time.Second),
		MaxDownloadMB:      16,
	}
	mgr, err := update.New(update.Options{
		Config:         cfg,
		CurrentVersion: "1.5.0",
		Executable:     exe,
		DataDir:        dir,
	})
	if err != nil {
		t.Fatal(err)
	}
	s := newMetricsServer(t)
	s.updates = mgr
	s.cfg.Update = cfg
	viewer := &model.User{Username: "viewer", Role: "viewer"}
	admin := &model.User{Username: "admin", Role: "admin"}

	check := httptest.NewRecorder()
	s.handleUpdateCheck(check, auditRequest(viewer, http.MethodGet, "/api/v1/update/check", "", "", ""))
	if check.Code != http.StatusOK {
		t.Fatalf("check: código %d: %s", check.Code, check.Body.String())
	}
	var result update.CheckResult
	if err := json.Unmarshal(check.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if !result.Available || result.LatestVersion != "1.6.0" {
		t.Fatalf("check inesperado: %+v", result)
	}

	denied := httptest.NewRecorder()
	s.handleUpdateApply(denied, auditRequest(viewer, http.MethodPost, "/api/v1/update/apply", `{"restart":false}`, "", ""))
	if denied.Code != http.StatusForbidden {
		t.Fatalf("viewer pudo aplicar: código %d", denied.Code)
	}

	apply := httptest.NewRecorder()
	s.handleUpdateApply(apply, auditRequest(admin, http.MethodPost, "/api/v1/update/apply", `{"version":"1.6.0","restart":false}`, "", ""))
	if apply.Code != http.StatusOK {
		t.Fatalf("apply: código %d: %s", apply.Code, apply.Body.String())
	}
	installed, err := os.ReadFile(exe)
	if err != nil || !bytes.Equal(installed, artifact) {
		t.Fatal("la API no instaló el binario firmado")
	}

	status := httptest.NewRecorder()
	s.handleUpdateStatus(status, auditRequest(viewer, http.MethodGet, "/api/v1/update/status", "", "", ""))
	if status.Code != http.StatusOK || mgr.Pending() == nil {
		t.Fatalf("status inesperado: código %d: %s", status.Code, status.Body.String())
	}

	rollback := httptest.NewRecorder()
	s.handleUpdateRollback(rollback, auditRequest(admin, http.MethodPost, "/api/v1/update/rollback", "", "", ""))
	if rollback.Code != http.StatusOK {
		t.Fatalf("rollback: código %d: %s", rollback.Code, rollback.Body.String())
	}
	restored, err := os.ReadFile(exe)
	if err != nil || !bytes.Equal(restored, original) {
		t.Fatal("la API no restauró el binario original")
	}
}
