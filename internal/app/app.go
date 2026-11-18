// Package app wires the storage, discovery, alerting and HTTP layers together
// and owns the process lifecycle.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/kr3s4l4/netdiag/internal/alerts"
	"github.com/kr3s4l4/netdiag/internal/api"
	"github.com/kr3s4l4/netdiag/internal/auth"
	"github.com/kr3s4l4/netdiag/internal/backup"
	"github.com/kr3s4l4/netdiag/internal/config"
	"github.com/kr3s4l4/netdiag/internal/cryptoutil"
	"github.com/kr3s4l4/netdiag/internal/discover"
	"github.com/kr3s4l4/netdiag/internal/integrations"
	"github.com/kr3s4l4/netdiag/internal/model"
	"github.com/kr3s4l4/netdiag/internal/report"
	"github.com/kr3s4l4/netdiag/internal/store"
	"github.com/kr3s4l4/netdiag/internal/vpn"
	"github.com/kr3s4l4/netdiag/internal/webui"
	"github.com/kr3s4l4/netdiag/internal/wifi"
)

// BuildInfo carries linker-injected build metadata.
type BuildInfo struct {
	Version   string
	Commit    string
	BuildDate string
}

// App is the running NetDiag instance.
type App struct {
	cfg     *config.Config
	cfgPath string
	log     *slog.Logger
	build   BuildInfo

	st           *store.Store
	auth         *auth.Service
	alerts       *alerts.Manager
	integrations *integrations.Manager
	vpn          *vpn.Manager
	wifi         *wifi.Manager
	reports      *report.Generator
	backups      *backup.Manager
	engine       *discover.Engine
	api          *api.Server
	hub          *api.Hub

	mu       sync.Mutex
	cfgMu    sync.RWMutex
	runs     map[int64]*runState
	busyCIDR map[string]int64
	deepLast map[int64]time.Time
	wg       sync.WaitGroup
	closed   bool
}

type runState struct {
	id        int64
	networkID int64
	cidr      string
	cancel    context.CancelFunc
}

const maxConcurrentScans = 3

// New builds the application, opening the database and preparing every layer.
func New(cfg *config.Config, cfgPath string, build BuildInfo, log *slog.Logger) (*App, error) {
	if log == nil {
		log = slog.Default()
	}
	if build.Version == "" {
		build.Version = "dev"
	}
	if err := cfg.EnsureDirs(); err != nil {
		return nil, fmt.Errorf("no se pudo preparar el directorio de datos: %w", err)
	}
	box, err := cryptoutil.LoadOrCreate(filepath.Join(cfg.Storage.DataDir, "secret.key"))
	if err != nil {
		return nil, fmt.Errorf("no se pudo abrir la clave de cifrado: %w", err)
	}
	st, err := store.Open(cfg.Storage.DB, box)
	if err != nil {
		return nil, fmt.Errorf("no se pudo abrir la base de datos: %w", err)
	}
	if err := st.MarkInterruptedScans(); err != nil {
		log.Warn("no se pudieron cerrar escaneos interrumpidos", "error", err)
	}
	if err := st.EnsureBuiltinRules(); err != nil {
		log.Warn("no se pudieron crear las reglas integradas", "error", err)
	}

	hostname, _ := os.Hostname()
	footer := "NetDiag · " + hostname
	reports := report.New(filepath.Join(cfg.Storage.DataDir, "reports"), "NetDiag", footer)

	a := &App{
		cfg:          cfg,
		cfgPath:      cfgPath,
		log:          log,
		build:        build,
		st:           st,
		auth:         auth.New(st, cfg.Security, log),
		alerts:       alerts.New(st, cfg.Alerts, log),
		integrations: integrations.New(st, cfg.Integrations, log),
		vpn:          vpn.New(st, cfg.VPN, log),
		wifi:         wifi.New(st, cfg.WiFi, log),
		reports:      reports,
		backups:      backup.New(st, cfg, cfgPath, log),
		engine:       discover.New(),
		runs:         make(map[int64]*runState),
		busyCIDR:     make(map[string]int64),
		deepLast:     make(map[int64]time.Time),
	}
	a.api = api.New(api.Deps{
		Store:        st,
		Config:       cfg,
		ConfigPath:   cfgPath,
		Auth:         a.auth,
		Alerts:       a.alerts,
		Integrations: a.integrations,
		VPN:          a.vpn,
		WiFi:         a.wifi,
		Reports:      reports,
		Backups:      a.backups,
		Scanner:      a,
		Logger:       log,
		Version:      build.Version,
		Commit:       build.Commit,
		BuildDate:    build.BuildDate,
		StartedAt:    time.Now(),
	})
	a.hub = a.api.Hub()
	a.alerts.SetNotifier(func(al model.Alert) {
		a.hub.Broadcast("alert.new", al)
		a.integrations.Publish(integrations.Event{
			Kind: "alert", Severity: al.Severity, Data: al.Data,
			Title: al.Title, Message: al.Message, HostIP: al.HostIP,
			Timestamp: time.Now(),
		})
	})
	return a, nil
}

// Store exposes the database (used by CLI subcommands).
func (a *App) Store() *store.Store { return a.st }

// Auth exposes the auth service (used by CLI subcommands).
func (a *App) Auth() *auth.Service { return a.auth }

// Hub exposes the event hub.
func (a *App) Hub() *api.Hub { return a.hub }

// Backups exposes the backup manager (used by CLI subcommands).
func (a *App) Backups() *backup.Manager { return a.backups }

// Close releases the database.
func (a *App) Close() error {
	a.mu.Lock()
	if a.closed {
		a.mu.Unlock()
		return nil
	}
	a.closed = true
	a.mu.Unlock()
	return a.st.Close()
}

// Run serves HTTP until the context is cancelled.
func (a *App) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	a.alerts.Start(ctx)
	a.integrations.Start(ctx)
	go a.vpn.Start(ctx)
	go a.wifi.Start(ctx)
	go a.monitorLoop(ctx)
	go a.maintenanceLoop(ctx)
	if a.backups != nil {
		go a.backups.Start(ctx)
	}

	handler := webui.Handler(a.api.Handler())
	srv := &http.Server{
		Addr:              a.cfg.Server.Listen,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       a.cfg.Server.ReadTimeout.Duration(),
		WriteTimeout:      a.cfg.Server.WriteTimeout.Duration(),
		IdleTimeout:       a.cfg.Server.IdleTimeout.Duration(),
		MaxHeaderBytes:    1 << 20,
	}

	errCh := make(chan error, 1)
	go func() {
		var err error
		if a.cfg.Server.TLSCert != "" && a.cfg.Server.TLSKey != "" {
			a.log.Info("servidor HTTPS escuchando", "addr", a.cfg.Server.Listen)
			err = srv.ListenAndServeTLS(a.cfg.Server.TLSCert, a.cfg.Server.TLSKey)
		} else {
			a.log.Info("servidor HTTP escuchando", "addr", a.cfg.Server.Listen,
				"url", "http://"+a.cfg.Server.Listen)
			err = srv.ListenAndServe()
		}
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		a.log.Info("apagando NetDiag")
	case err := <-errCh:
		cancel()
		a.stopScans()
		a.wg.Wait()
		a.integrations.Stop()
		a.alerts.Stop()
		return err
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		a.log.Warn("apagado del servidor incompleto", "error", err)
		_ = srv.Close()
	}
	a.stopScans()
	done := make(chan struct{})
	go func() {
		a.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(20 * time.Second):
		a.log.Warn("escaneos aún en curso tras el tiempo de espera")
	}
	a.integrations.Stop()
	a.alerts.Stop()
	return nil
}

func (a *App) stopScans() {
	a.mu.Lock()
	states := make([]*runState, 0, len(a.runs))
	for _, st := range a.runs {
		states = append(states, st)
	}
	a.mu.Unlock()
	for _, st := range states {
		st.cancel()
	}
}

// currentCfg returns the live configuration pointer.
func (a *App) currentCfg() *config.Config {
	a.cfgMu.RLock()
	defer a.cfgMu.RUnlock()
	return a.cfg
}

// UpdateConfig swaps configuration for the running process.
func (a *App) UpdateConfig(cfg *config.Config) error {
	if cfg == nil {
		return errors.New("configuración vacía")
	}
	a.cfgMu.Lock()
	a.cfg = cfg
	a.cfgMu.Unlock()
	a.alerts.UpdateConfig(cfg.Alerts)
	a.integrations.UpdateConfig(cfg.Integrations)
	a.vpn.UpdateConfig(cfg.VPN)
	a.wifi.UpdateConfig(cfg.WiFi)
	if a.backups != nil {
		a.backups.UpdateConfig(cfg)
	}
	return nil
}

func (a *App) maintenanceLoop(ctx context.Context) {
	ticker := time.NewTicker(6 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.runMaintenance()
		}
	}
}

func (a *App) runMaintenance() {
	cfg := a.currentCfg()
	a.auth.PurgeExpiredSessions()
	if cfg.Storage.MetricsDays > 0 {
		if err := a.st.PurgeMetrics(cfg.Storage.MetricsDays); err != nil {
			a.log.Warn("no se pudieron purgar métricas", "error", err)
		}
	}
	if cfg.Storage.RetentionDays > 0 {
		if err := a.st.PurgeAlerts(cfg.Storage.RetentionDays); err != nil {
			a.log.Warn("no se pudieron purgar alertas", "error", err)
		}
	}
	if cfg.Storage.AuditDays > 0 {
		if err := a.st.PurgeAudit(cfg.Storage.AuditDays); err != nil {
			a.log.Warn("no se pudo purgar la auditoría", "error", err)
		}
	}
	if cfg.Storage.MaxScans > 0 {
		if err := a.st.PurgeScans(cfg.Storage.MaxScans); err != nil {
			a.log.Warn("no se pudo purgar el historial de escaneos", "error", err)
		}
	}
}
