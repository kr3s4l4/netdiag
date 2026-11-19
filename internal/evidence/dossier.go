// Package evidence builds local, signed .ndc dossiers with reports, diffs and
// audits. It never uploads anything and never uses a remote service.
package evidence

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/kr3s4l4/netdiag/internal/audits"
	"github.com/kr3s4l4/netdiag/internal/config"
	"github.com/kr3s4l4/netdiag/internal/container"
	"github.com/kr3s4l4/netdiag/internal/cryptoutil"
	"github.com/kr3s4l4/netdiag/internal/history"
	"github.com/kr3s4l4/netdiag/internal/model"
	"github.com/kr3s4l4/netdiag/internal/report"
	"github.com/kr3s4l4/netdiag/internal/store"
	"github.com/kr3s4l4/netdiag/internal/update"
)

// Entry describes one generated dossier.
type Entry struct {
	Path      string    `json:"path"`
	Name      string    `json:"name"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"created_at"`
	Valid     bool      `json:"valid"`
}

// Options overrides the configured defaults for one manual generation.
type Options struct {
	SiteID   int64
	Days     int
	SeedFile string
	KeyPub   string
	Out      string
	Version  string
}

// Manager owns dossier generation, verification and retention.
type Manager struct {
	st      *store.Store
	reports *report.Generator
	version string
	log     *slog.Logger

	mu  sync.RWMutex
	cfg *config.Config
}

// New builds an evidence manager.
func New(st *store.Store, reports *report.Generator, cfg *config.Config, version string, log *slog.Logger) *Manager {
	if log == nil {
		log = slog.Default()
	}
	return &Manager{st: st, reports: reports, cfg: cfg, version: version, log: log}
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

// Dir returns the local dossier directory.
func (m *Manager) Dir() string {
	cfg := m.current()
	if cfg == nil {
		return ""
	}
	return filepath.Join(cfg.Storage.DataDir, "dossiers")
}

// Generate writes and verifies one signed dossier.
func (m *Manager) Generate(opt Options) (*Entry, error) {
	cfg := m.current()
	if cfg == nil || m.st == nil {
		return nil, errors.New("gestor de evidencia sin configuración o almacén")
	}
	d := cfg.Evidence.Dossier
	siteID := opt.SiteID
	if siteID <= 0 {
		siteID = 1
	}
	days := opt.Days
	if days <= 0 {
		days = d.Days
	}
	if days <= 0 {
		days = 30
	}
	seedFile := strings.TrimSpace(opt.SeedFile)
	if seedFile == "" {
		seedFile = d.SeedFile
	}
	if seedFile == "" {
		return nil, errors.New("evidence.dossier.seed_file no configurada y no se indicó --seed-file")
	}
	seed, keyID, err := cryptoutil.LoadSigningMaterial(seedFile, "")
	if err != nil {
		return nil, err
	}
	publicKey := cryptoutil.PublicKeyB64(opt.KeyPub)
	if publicKey == "" {
		publicKey = cryptoutil.PublicKeyB64(d.KeyPub)
	}
	if publicKey == "" {
		publicKey, err = update.PublicKeyFromSeed(seed, keyID)
		if err != nil {
			return nil, err
		}
	}

	dir := m.Dir()
	if dir == "" {
		return nil, errors.New("directorio de dossiers no configurado")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	out := strings.TrimSpace(opt.Out)
	if out == "" {
		out = filepath.Join(dir, "netdiag-dossier-"+time.Now().Format("20060102-150405")+".ndc")
		for i := 1; ; i++ {
			if _, err := os.Stat(out); errors.Is(err, os.ErrNotExist) {
				break
			}
			out = filepath.Join(dir, fmt.Sprintf("netdiag-dossier-%s-%d.ndc", time.Now().Format("20060102-150405"), i))
		}
	} else if err := os.MkdirAll(filepath.Dir(out), 0o700); err != nil {
		return nil, err
	}

	files, err := m.files(siteID, days, m.version)
	if err != nil {
		return nil, err
	}
	version := strings.TrimSpace(opt.Version)
	if version == "" {
		version = m.version
	}
	if version == "" {
		version = "dev"
	}
	if err := container.Build(out, container.BuildOptions{
		Version:  version,
		Kind:     "evidence",
		Producer: "netdiag-evidence",
		Files:    files,
		Seed:     seed,
		KeyID:    keyID,
	}); err != nil {
		return nil, fmt.Errorf("no se pudo construir el dossier: %w", err)
	}
	verified, err := container.Verify(out, publicKey)
	if err != nil {
		return nil, err
	}
	if !verified.Valid {
		problem := verified.Signature.Problem
		if problem == "" {
			problem = "contenido no íntegro"
		}
		return nil, fmt.Errorf("dossier no válido: %s", problem)
	}
	info, err := os.Stat(out)
	if err != nil {
		return nil, err
	}
	entry := &Entry{
		Path: out, Name: filepath.Base(out), Size: info.Size(),
		CreatedAt: time.Now().UTC(), Valid: true,
	}
	if strings.TrimSpace(opt.Out) == "" {
		if err := m.prune(); err != nil {
			m.log.Warn("no se pudieron purgar dossiers antiguos", "error", err)
		}
	}
	return entry, nil
}

// List returns dossiers in the configured directory, newest first.
func (m *Manager) List() ([]Entry, error) {
	dir := m.Dir()
	items, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []Entry{}, nil
		}
		return nil, err
	}
	entries := []Entry{}
	for _, item := range items {
		if item.IsDir() || filepath.Ext(item.Name()) != ".ndc" {
			continue
		}
		info, err := item.Info()
		if err != nil {
			continue
		}
		entries = append(entries, Entry{
			Path: filepath.Join(dir, item.Name()), Name: item.Name(),
			Size: info.Size(), CreatedAt: info.ModTime(),
		})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].CreatedAt.After(entries[j].CreatedAt) })
	return entries, nil
}

// Start runs the scheduler inside netdiag serve, like backups do.
func (m *Manager) Start(ctx context.Context) {
	cfg := m.current()
	if cfg == nil || !cfg.Evidence.Dossier.Enabled {
		return
	}
	interval := time.Duration(cfg.Evidence.Dossier.IntervalHours) * time.Hour
	if interval < time.Hour {
		interval = 24 * time.Hour
	}
	generate := func() {
		if _, err := m.Generate(Options{}); err != nil {
			m.log.Warn("no se pudo generar el dossier de evidencia", "error", err)
		}
	}
	generate()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			generate()
		}
	}
}

func (m *Manager) prune() error {
	entries, err := m.List()
	if err != nil {
		return err
	}
	keep := m.current().Evidence.Dossier.Keep
	if keep < 1 {
		keep = 7
	}
	for _, entry := range entries[min(keep, len(entries)):] {
		if err := os.Remove(entry.Path); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) files(siteID int64, days int, version string) ([]container.InputFile, error) {
	now := time.Now().UTC()
	a, b := history.DefaultPeriods(now, days, 24*time.Hour)
	diff, err := history.BuildReport(m.st, siteID, a, b)
	if err != nil {
		return nil, fmt.Errorf("no se pudo construir el diff histórico: %w", err)
	}
	files := []container.InputFile{}
	if data, err := json.MarshalIndent(diff, "", "  "); err == nil {
		files = append(files, container.InputFile{Name: "diff.json", Data: data})
	}

	hosts, _ := m.st.ListAllHostsForSite(siteID, 5000)
	stats, _ := m.st.Dashboard(siteID)
	if m.reports != nil {
		req := report.Request{
			SiteID: siteID, Kind: "executive", Format: "pdf",
			From: a.From, To: a.To, Hosts: hosts,
			Changes: diff.Changes, Narrative: diff.Narrative,
			Version: version, RetentionDays: days,
		}
		if stats != nil {
			req.Stats = *stats
		}
		if rec, err := m.reports.Generate(req); err == nil {
			files = append(files, container.InputFile{Name: "informe-ejecutivo.pdf", Path: rec.Path})
		}
	}

	since := now.AddDate(0, 0, -days)
	if logs, err := m.st.ListAudit(model.AuditFilter{From: &since, Limit: 500, Page: 1}); err == nil {
		if data, err := json.MarshalIndent(logs, "", "  "); err == nil {
			files = append(files, container.InputFile{Name: "auditoria.json", Data: data})
		}
	}

	snapshots, _ := m.st.ListAuditSnapshots(siteID, 200)
	recent := []model.AuditSnapshot{}
	for _, snap := range snapshots {
		if !snap.TS.Before(since) {
			recent = append(recent, snap)
		}
	}
	if len(recent) > 24 {
		recent = recent[:24]
	}
	full := []*model.AuditSnapshot{}
	for _, snap := range recent {
		if item, err := m.st.GetAuditSnapshot(snap.ID, true); err == nil && item != nil {
			full = append(full, item)
		}
	}
	if len(full) > 0 {
		if data, err := json.MarshalIndent(full, "", "  "); err == nil {
			files = append(files, container.InputFile{Name: "audit-snapshots.json", Data: data})
		}
		oldest, newest := full[len(full)-1], full[0]
		if diffAudit := audits.Diff(oldest, newest); diffAudit != nil {
			if data, err := json.MarshalIndent(diffAudit, "", "  "); err == nil {
				files = append(files, container.InputFile{Name: "audit-diff.json", Data: data})
			}
		}
	}

	meta := map[string]any{
		"schema": "netdiag.dossier/v1", "site_id": siteID, "days": days,
		"from": a.From, "to": a.To, "generated_at": now,
		"contains":   []string{"diff.json", "informe-ejecutivo.pdf", "auditoria.json"},
		"limitation": "El contenedor da integridad y procedencia, no confidencialidad.",
	}
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return nil, err
	}
	files = append(files, container.InputFile{Name: "dossier.json", Data: data})
	if len(files) == 0 {
		return nil, errors.New("el dossier no tiene contenido")
	}
	return files, nil
}
