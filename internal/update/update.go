// Package update implements F23 self-updates: signed manifest checks, verified
// downloads, atomic installation with rollback markers and controlled restart.
package update

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/kr3s4l4/netdiag/internal/config"
)

const (
	manifestMaxBytes = 1 << 20
	pendingSchema    = "netdiag/update-pending/v1"
)

// Options configures an update manager.
type Options struct {
	Config         config.Update
	CurrentVersion string
	Executable     string
	DataDir        string
	Log            *slog.Logger
	HTTPClient     *http.Client
}

// CheckResult is returned by the periodic or manual version check.
type CheckResult struct {
	CurrentVersion string         `json:"current_version"`
	LatestVersion  string         `json:"latest_version,omitempty"`
	Available      bool           `json:"available"`
	Release        *Release       `json:"release,omitempty"`
	CheckedAt      time.Time      `json:"checked_at"`
	Message        string         `json:"message,omitempty"`
	Pending        *PendingUpdate `json:"pending,omitempty"`
}

// ApplyResult describes a completed installation (before any restart).
type ApplyResult struct {
	FromVersion string    `json:"from_version"`
	ToVersion   string    `json:"to_version"`
	InstalledAt time.Time `json:"installed_at"`
	BackupPath  string    `json:"backup_path"`
	Message     string    `json:"message,omitempty"`
}

// PendingUpdate is the rollback marker written before replacing the binary.
type PendingUpdate struct {
	Schema          string    `json:"schema"`
	Version         string    `json:"version"`
	PreviousVersion string    `json:"previous_version"`
	Checksum        string    `json:"checksum"`
	BackupPath      string    `json:"backup_path"`
	AppliedAt       time.Time `json:"applied_at"`
}

// CommitResult reports what happened when a pending marker was reconciled at
// process startup.
type CommitResult struct {
	Confirmed       bool   `json:"confirmed"`
	RolledBack      bool   `json:"rolled_back"`
	Version         string `json:"version,omitempty"`
	PreviousVersion string `json:"previous_version,omitempty"`
}

// RollbackResult reports a manual rollback.
type RollbackResult struct {
	Version string `json:"version,omitempty"`
	Message string `json:"message,omitempty"`
}

// Manager owns update checks and installations for a single binary.
type Manager struct {
	cfg            config.Update
	currentVersion string
	exePath        string
	markerPath     string
	client         *http.Client
	log            *slog.Logger

	mu        sync.Mutex
	latest    *Release
	checkedAt time.Time
	notifier  func(string, any)
}

// New builds an update manager. An empty Executable resolves to the running
// binary; empty CurrentVersion resolves to "dev".
func New(opts Options) (*Manager, error) {
	exe := strings.TrimSpace(opts.Executable)
	if exe == "" {
		resolved, err := os.Executable()
		if err != nil {
			return nil, fmt.Errorf("no se pudo resolver el ejecutable: %w", err)
		}
		exe = resolved
	}
	exe, err := filepath.Abs(exe)
	if err != nil {
		return nil, err
	}
	version := strings.TrimSpace(opts.CurrentVersion)
	if version == "" {
		version = "dev"
	}
	dataDir := strings.TrimSpace(opts.DataDir)
	if dataDir == "" {
		dataDir = filepath.Dir(exe)
	}
	log := opts.Log
	if log == nil {
		log = slog.Default()
	}
	client := opts.HTTPClient
	if client == nil {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
		client = &http.Client{Transport: transport}
	}
	client.Timeout = opts.Config.DownloadTimeout.Duration()
	if client.Timeout <= 0 {
		client.Timeout = 120 * time.Second
	}
	if client.CheckRedirect == nil {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("demasiadas redirecciones al descargar la actualización")
			}
			return validateArtifactURL(req.URL.String())
		}
	}
	return &Manager{
		cfg:            opts.Config,
		currentVersion: version,
		exePath:        exe,
		markerPath:     filepath.Join(dataDir, "update-pending.json"),
		client:         client,
		log:            log,
	}, nil
}

// SetNotifier installs a callback for update events.
func (m *Manager) SetNotifier(fn func(string, any)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.notifier = fn
}

func (m *Manager) notifyLocked(event string, data any) {
	if m.notifier != nil {
		m.notifier(event, data)
	}
}

// Check downloads, verifies and evaluates the signed manifest. It never
// installs anything.
func (m *Manager) Check(ctx context.Context) (*CheckResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.checkLocked(ctx)
}

func (m *Manager) checkLocked(ctx context.Context) (*CheckResult, error) {
	res := &CheckResult{CurrentVersion: m.currentVersion, CheckedAt: time.Now()}
	if pending := m.pendingLocked(); pending != nil {
		res.Pending = pending
	}
	if !m.cfg.Enabled {
		res.Message = "comprobaciones de actualización desactivadas"
		return res, nil
	}
	data, err := m.fetch(ctx, m.cfg.ManifestURL, manifestMaxBytes, "application/json")
	if err != nil {
		return nil, fmt.Errorf("no se pudo descargar el manifiesto: %w", err)
	}
	rel, err := ParseManifest(data, m.cfg.PublicKey)
	if err != nil {
		return nil, err
	}
	m.latest, m.checkedAt = rel, time.Now()
	res.CheckedAt = m.checkedAt
	res.LatestVersion, res.Release = rel.Version, rel
	res.Available = IsNewer(rel.Version, m.currentVersion)
	if res.Available {
		res.Message = "hay una versión más reciente disponible"
	} else {
		res.Message = "NetDiag está actualizado"
	}
	m.notifyLocked("update.checked", res)
	if res.Available {
		m.notifyLocked("update.available", res)
	}
	return res, nil
}

// Apply verifies the signed manifest again, downloads the binary, checks its
// SHA256, installs it atomically and writes a rollback marker. It does not
// restart the process; callers decide when to do so.
func (m *Manager) Apply(ctx context.Context, wantVersion string) (*ApplyResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.cfg.Enabled {
		return nil, errors.New("las actualizaciones están desactivadas")
	}
	if pending := m.pendingLocked(); pending != nil {
		return nil, fmt.Errorf("ya hay una actualización pendiente (%s) sin confirmar", pending.Version)
	}
	check, err := m.checkLocked(ctx)
	if err != nil {
		return nil, err
	}
	if !check.Available || check.Release == nil {
		return nil, errors.New("no hay una versión más reciente disponible")
	}
	rel := check.Release
	if wantVersion != "" && strings.TrimSpace(wantVersion) != rel.Version {
		return nil, fmt.Errorf("la versión publicada (%s) no coincide con la solicitada (%s)", rel.Version, wantVersion)
	}

	artifact, err := m.fetch(ctx, rel.URL, int64(m.cfg.MaxDownloadMB)<<20, "application/octet-stream")
	if err != nil {
		return nil, fmt.Errorf("no se pudo descargar el binario: %w", err)
	}
	sum := sha256.Sum256(artifact)
	if got := hex.EncodeToString(sum[:]); got != strings.ToLower(rel.Checksum) {
		return nil, fmt.Errorf("checksum del binario no coincide: esperado %s, obtenido %s", rel.Checksum, got)
	}
	if runtime.GOOS == "linux" && (len(artifact) < 4 || !bytes.Equal(artifact[:4], []byte{0x7f, 'E', 'L', 'F'})) {
		return nil, errors.New("el binario descargado no es un ejecutable ELF")
	}

	info, err := os.Stat(m.exePath)
	if err != nil {
		return nil, fmt.Errorf("no se pudo inspeccionar el ejecutable actual: %w", err)
	}
	mode := info.Mode().Perm()
	if mode&0o111 == 0 {
		mode = 0o755
	}
	dir := filepath.Dir(m.exePath)
	tmp, err := os.CreateTemp(dir, ".netdiag-update-*")
	if err != nil {
		return nil, fmt.Errorf("no se pudo crear el archivo temporal: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }
	if _, err := tmp.Write(artifact); err != nil {
		_ = tmp.Close()
		cleanup()
		return nil, err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		cleanup()
		return nil, err
	}
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		cleanup()
		return nil, err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return nil, err
	}

	backupPath := m.exePath + ".netdiag-old"
	if _, err := os.Lstat(backupPath); err == nil {
		cleanup()
		return nil, fmt.Errorf("existe una copia de seguridad previa en %s; confirma o revierte antes de actualizar", backupPath)
	} else if !errors.Is(err, os.ErrNotExist) {
		cleanup()
		return nil, err
	}
	if err := os.Rename(m.exePath, backupPath); err != nil {
		cleanup()
		return nil, fmt.Errorf("no se pudo crear la copia de seguridad: %w", err)
	}
	if err := os.Rename(tmpName, m.exePath); err != nil {
		if rb := os.Rename(backupPath, m.exePath); rb != nil {
			m.log.Error("no se pudo restaurar el binario tras un fallo de instalación", "error", rb)
		}
		cleanup()
		return nil, fmt.Errorf("no se pudo instalar el binario: %w", err)
	}
	syncDir(dir)

	marker := PendingUpdate{
		Schema:          pendingSchema,
		Version:         rel.Version,
		PreviousVersion: m.currentVersion,
		Checksum:        strings.ToLower(rel.Checksum),
		BackupPath:      backupPath,
		AppliedAt:       time.Now().UTC(),
	}
	if err := m.writeMarkerLocked(marker); err != nil {
		if rb := os.Rename(backupPath, m.exePath); rb != nil {
			m.log.Error("no se pudo restaurar el binario tras fallar el marcador", "error", rb)
		}
		cleanup()
		return nil, fmt.Errorf("no se pudo escribir el marcador de rollback: %w", err)
	}
	result := &ApplyResult{
		FromVersion: m.currentVersion,
		ToVersion:   rel.Version,
		InstalledAt: marker.AppliedAt,
		BackupPath:  backupPath,
		Message:     "binario instalado; reinicia para completar la actualización",
	}
	m.currentVersion = rel.Version
	m.latest = nil
	m.notifyLocked("update.installed", result)
	return result, nil
}

// CommitPending reconciles a rollback marker at startup. If the running binary
// matches the installed checksum the marker and backup are discarded;
// otherwise the previous binary is restored automatically.
func (m *Manager) CommitPending() (*CommitResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	marker, err := m.readMarkerLocked()
	if err != nil || marker == nil {
		return nil, err
	}
	if marker.Schema != pendingSchema || marker.BackupPath == "" {
		return nil, errors.New("marcador de actualización no válido")
	}
	data, err := os.ReadFile(m.exePath)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) == strings.ToLower(marker.Checksum) {
		_ = os.Remove(m.markerPath)
		_ = os.Remove(marker.BackupPath)
		syncDir(filepath.Dir(m.exePath))
		m.log.Info("actualización confirmada", "version", marker.Version)
		return &CommitResult{Confirmed: true, Version: marker.Version}, nil
	}
	m.log.Warn("el binario instalado no coincide con el marcador; revirtiendo", "version", marker.Version)
	if err := restoreBackup(marker.BackupPath, m.exePath); err != nil {
		return nil, err
	}
	_ = os.Remove(m.markerPath)
	syncDir(filepath.Dir(m.exePath))
	return &CommitResult{
		RolledBack:      true,
		PreviousVersion: marker.PreviousVersion,
	}, nil
}

// Rollback restores the previous binary from the marker or the default backup.
func (m *Manager) Rollback() (*RollbackResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	marker, _ := m.readMarkerLocked()
	backup := m.exePath + ".netdiag-old"
	previous := ""
	if marker != nil && marker.BackupPath != "" {
		backup = marker.BackupPath
		previous = marker.PreviousVersion
	}
	if err := restoreBackup(backup, m.exePath); err != nil {
		return nil, err
	}
	if marker != nil {
		_ = os.Remove(m.markerPath)
	}
	syncDir(filepath.Dir(m.exePath))
	m.currentVersion = previous
	if m.currentVersion == "" {
		m.currentVersion = "dev"
	}
	result := &RollbackResult{Version: m.currentVersion, Message: "versión anterior restaurada; reinicia para aplicarla"}
	m.notifyLocked("update.rolled_back", result)
	return result, nil
}

// Restart replaces the process via syscall.Exec, or runs the configured
// restart command (for example systemctl). It never uses a shell.
func (m *Manager) Restart(ctx context.Context) error {
	if strings.TrimSpace(m.cfg.RestartCommand) == "" {
		m.log.Info("reiniciando NetDiag mediante exec")
		args := append([]string{m.exePath}, os.Args[1:]...)
		return syscall.Exec(m.exePath, args, os.Environ())
	}
	m.log.Info("reiniciando NetDiag mediante comando configurado", "command", m.cfg.RestartCommand)
	cmd := exec.CommandContext(ctx, m.cfg.RestartCommand, m.cfg.RestartArgs...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}

// Pending returns a copy of the active rollback marker, if any.
func (m *Manager) Pending() *PendingUpdate {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.pendingLocked()
}

// CurrentVersion returns the version the manager considers installed.
func (m *Manager) CurrentVersion() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.currentVersion
}

func (m *Manager) fetch(ctx context.Context, rawURL string, max int64, accept string) ([]byte, error) {
	if err := validateArtifactURL(rawURL); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", accept)
	resp, err := m.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("respuesta HTTP %d", resp.StatusCode)
	}
	return readLimited(resp.Body, max)
}

func (m *Manager) writeMarkerLocked(marker PendingUpdate) error {
	data, err := json.MarshalIndent(marker, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(m.markerPath)
	tmp, err := os.CreateTemp(dir, ".update-pending-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(name)
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		_ = os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(name)
		return err
	}
	if err := os.Rename(name, m.markerPath); err != nil {
		_ = os.Remove(name)
		return err
	}
	syncDir(dir)
	return nil
}

func (m *Manager) readMarkerLocked() (*PendingUpdate, error) {
	data, err := os.ReadFile(m.markerPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var marker PendingUpdate
	if err := json.Unmarshal(data, &marker); err != nil {
		return nil, fmt.Errorf("marcador de actualización corrupto: %w", err)
	}
	return &marker, nil
}

func (m *Manager) pendingLocked() *PendingUpdate {
	marker, err := m.readMarkerLocked()
	if err != nil {
		m.log.Warn("no se pudo leer el marcador de actualización", "error", err)
		return nil
	}
	return marker
}

func restoreBackup(backup, target string) error {
	if _, err := os.Stat(backup); err != nil {
		return fmt.Errorf("no se encontró la copia de seguridad %s: %w", backup, err)
	}
	if err := os.Rename(backup, target); err != nil {
		return fmt.Errorf("no se pudo restaurar la copia de seguridad: %w", err)
	}
	return nil
}

func syncDir(dir string) {
	f, err := os.Open(dir)
	if err != nil {
		return
	}
	_ = f.Sync()
	_ = f.Close()
}
