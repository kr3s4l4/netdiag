// Package backup creates self-contained ZIP snapshots of the database,
// configuration and encryption key, and prunes old copies.
package backup

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/kr3s4l4/netdiag/internal/config"
	"github.com/kr3s4l4/netdiag/internal/store"
)

// ErrInvalidName is returned for names that could escape the backup folder.
var ErrInvalidName = errors.New("nombre de copia inválido")

// Entry describes one backup file.
type Entry struct {
	Name      string    `json:"name"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"created_at"`
}

// Manager owns backup creation and retention.
type Manager struct {
	st      *store.Store
	cfgPath string
	log     *slog.Logger

	mu  sync.RWMutex
	cfg *config.Config
}

// New builds a backup manager.
func New(st *store.Store, cfg *config.Config, cfgPath string, log *slog.Logger) *Manager {
	if log == nil {
		log = slog.Default()
	}
	return &Manager{st: st, cfg: cfg, cfgPath: cfgPath, log: log}
}

// UpdateConfig swaps the live configuration.
func (m *Manager) UpdateConfig(cfg *config.Config) {
	m.mu.Lock()
	m.cfg = cfg
	m.mu.Unlock()
}

func (m *Manager) current() *config.Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cfg
}

// Dir returns the configured backup directory.
func (m *Manager) Dir() string {
	cfg := m.current()
	if cfg == nil {
		return ""
	}
	return cfg.Storage.BackupDir
}

// Create writes a backup into the configured directory.
func (m *Manager) Create() (*Entry, error) {
	dir := m.Dir()
	if dir == "" {
		return nil, errors.New("no hay directorio de copias configurado")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	name := "netdiag-" + time.Now().Format("20060102-150405") + ".zip"
	path := filepath.Join(dir, name)
	for i := 1; ; i++ {
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			break
		}
		path = filepath.Join(dir, fmt.Sprintf("netdiag-%s-%d.zip", time.Now().Format("20060102-150405"), i))
	}
	entry, err := m.CreateTo(path)
	if err != nil {
		return nil, err
	}
	if err := m.prune(); err != nil {
		m.log.Warn("no se pudieron purgar copias antiguas", "error", err)
	}
	return entry, nil
}

// CreateTo writes a backup at an explicit path.
func (m *Manager) CreateTo(path string) (*Entry, error) {
	if path == "" {
		return nil, errors.New("ruta de copia vacía")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	cfg := m.current()
	if cfg == nil {
		return nil, errors.New("configuración no disponible")
	}
	tmpDB, err := os.CreateTemp(filepath.Dir(path), ".netdiag-*.db")
	if err != nil {
		return nil, err
	}
	tmpDBPath := tmpDB.Name()
	_ = tmpDB.Close()
	_ = os.Remove(tmpDBPath)
	defer os.Remove(tmpDBPath)

	if err := m.st.VacuumInto(tmpDBPath); err != nil {
		return nil, fmt.Errorf("no se pudo congelar la base de datos: %w", err)
	}

	tmpZip, err := os.CreateTemp(filepath.Dir(path), ".netdiag-*.zip")
	if err != nil {
		return nil, err
	}
	tmpZipPath := tmpZip.Name()
	defer os.Remove(tmpZipPath)

	zw := zip.NewWriter(tmpZip)
	manifest := map[string]any{
		"app":        "netdiag",
		"created_at": time.Now().UTC().Format(time.RFC3339),
		"database":   "netdiag.db",
		"config":     "config.yaml",
		"secret":     "secret.key",
		"license":    "license.json",
	}
	if err := addJSON(zw, "manifest.json", manifest); err != nil {
		zw.Close()
		tmpZip.Close()
		return nil, err
	}
	if err := addFile(zw, "netdiag.db", tmpDBPath); err != nil {
		zw.Close()
		tmpZip.Close()
		return nil, err
	}
	if m.cfgPath != "" {
		if err := addFile(zw, "config.yaml", m.cfgPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			zw.Close()
			tmpZip.Close()
			return nil, err
		}
	}
	secret := filepath.Join(cfg.Storage.DataDir, "secret.key")
	if err := addFile(zw, "secret.key", secret); err != nil && !errors.Is(err, os.ErrNotExist) {
		zw.Close()
		tmpZip.Close()
		return nil, err
	}
	licence := filepath.Join(cfg.Storage.DataDir, "license.json")
	if err := addFile(zw, "license.json", licence); err != nil && !errors.Is(err, os.ErrNotExist) {
		zw.Close()
		tmpZip.Close()
		return nil, err
	}
	if err := zw.Close(); err != nil {
		tmpZip.Close()
		return nil, err
	}
	if err := tmpZip.Close(); err != nil {
		return nil, err
	}
	if err := os.Chmod(tmpZipPath, 0o600); err != nil {
		return nil, err
	}
	if err := os.Rename(tmpZipPath, path); err != nil {
		return nil, err
	}
	fi, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	return &Entry{Name: filepath.Base(path), Size: fi.Size(), CreatedAt: fi.ModTime()}, nil
}

// List returns backups newest first.
func (m *Manager) List() ([]Entry, error) {
	dir := m.Dir()
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return []Entry{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := make([]Entry, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".zip") {
			continue
		}
		fi, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, Entry{Name: e.Name(), Size: fi.Size(), CreatedAt: fi.ModTime()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

// Path resolves a backup name inside the backup directory.
func (m *Manager) Path(name string) (string, error) {
	clean := filepath.Base(strings.TrimSpace(name))
	if clean != name || clean == "." || clean == ".." || !strings.HasSuffix(strings.ToLower(clean), ".zip") {
		return "", ErrInvalidName
	}
	path := filepath.Join(m.Dir(), clean)
	if _, err := os.Stat(path); err != nil {
		return "", err
	}
	return path, nil
}

// Delete removes one backup.
func (m *Manager) Delete(name string) error {
	path, err := m.Path(name)
	if err != nil {
		return err
	}
	return os.Remove(path)
}

func (m *Manager) prune() error {
	keep := 7
	if cfg := m.current(); cfg != nil && cfg.Storage.BackupKeep > 0 {
		keep = cfg.Storage.BackupKeep
	}
	items, err := m.List()
	if err != nil {
		return err
	}
	for i := keep; i < len(items); i++ {
		if err := m.Delete(items[i].Name); err != nil {
			return err
		}
	}
	return nil
}

// Start runs periodic backups until ctx is cancelled.
func (m *Manager) Start(ctx context.Context) {
	cfg := m.current()
	if cfg == nil || !cfg.Storage.BackupEnabled {
		return
	}
	// Ensure there is an initial snapshot even if the process restarts often.
	if items, err := m.List(); err == nil && len(items) == 0 {
		if _, err := m.Create(); err != nil {
			m.log.Warn("no se pudo crear la copia inicial", "error", err)
		} else {
			m.log.Info("copia de seguridad inicial creada", "dir", m.Dir())
		}
	}
	interval := cfg.Storage.BackupInterval.Duration()
	if interval < 15*time.Minute {
		interval = 24 * time.Hour
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := m.Create(); err != nil {
				m.log.Warn("falló la copia de seguridad programada", "error", err)
			} else {
				m.log.Info("copia de seguridad creada", "dir", m.Dir())
			}
		}
	}
}

func addFile(zw *zip.Writer, name, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return err
	}
	hdr, err := zip.FileInfoHeader(fi)
	if err != nil {
		return err
	}
	hdr.Name = name
	hdr.Method = zip.Deflate
	hdr.SetMode(0o600)
	w, err := zw.CreateHeader(hdr)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, f)
	return err
}

func addJSON(zw *zip.Writer, name string, v any) error {
	hdr := &zip.FileHeader{Name: name, Method: zip.Deflate}
	hdr.SetMode(0o600)
	w, err := zw.CreateHeader(hdr)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
