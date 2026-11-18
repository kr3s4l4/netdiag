// Package api exposes the NetDiag REST API and the live WebSocket feed.
//
// All endpoints live under /api/v1. The SPA is served by the app package.
package api

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/kr3s4l4/netdiag/internal/alerts"
	"github.com/kr3s4l4/netdiag/internal/auth"
	"github.com/kr3s4l4/netdiag/internal/backup"
	"github.com/kr3s4l4/netdiag/internal/config"
	"github.com/kr3s4l4/netdiag/internal/integrations"
	"github.com/kr3s4l4/netdiag/internal/model"
	"github.com/kr3s4l4/netdiag/internal/report"
	"github.com/kr3s4l4/netdiag/internal/store"
	"github.com/kr3s4l4/netdiag/internal/vpn"
)

// ScanRequest is the transport payload for starting a discovery run.
type ScanRequest struct {
	SiteID      int64  `json:"site_id,omitempty"`
	NetworkID   int64  `json:"network_id"`
	CIDR        string `json:"cidr"`
	Profile     string `json:"profile"`
	Ports       []int  `json:"ports,omitempty"`
	SNMP        *bool  `json:"snmp,omitempty"`
	Deep        bool   `json:"deep"`
	TimeoutMS   int    `json:"timeout_ms,omitempty"`
	AllowPublic bool   `json:"allow_public,omitempty"`
}

// Scanner is implemented by the app package. Scanning outlives the HTTP
// request that started it, so it takes no request context.
type Scanner interface {
	StartScan(req ScanRequest, actor string) (*model.Scan, error)
	CancelScan(id int64) error
	UpdateConfig(cfg *config.Config) error
}

// Deps bundles everything the API needs.
type Deps struct {
	Store        *store.Store
	Config       *config.Config
	ConfigPath   string
	Auth         *auth.Service
	Alerts       *alerts.Manager
	Integrations *integrations.Manager
	VPN          *vpn.Manager
	Reports      *report.Generator
	Backups      *backup.Manager
	Scanner      Scanner
	Logger       *slog.Logger
	Version      string
	Commit       string
	BuildDate    string
	StartedAt    time.Time
}

// Server implements http.Handler for the API.
type Server struct {
	st           *store.Store
	cfg          *config.Config
	configPath   string
	auth         *auth.Service
	alerts       *alerts.Manager
	integrations *integrations.Manager
	vpn          *vpn.Manager
	reports      *report.Generator
	backups      *backup.Manager
	scanner      Scanner
	log          *slog.Logger
	hub          *Hub
	limiter      *ipLimiter

	version   string
	commit    string
	buildDate string
	startedAt time.Time

	mux *http.ServeMux
	ctx context.Context
}

// New builds the API server.
func New(d Deps) *Server {
	log := d.Logger
	if log == nil {
		log = slog.Default()
	}
	s := &Server{
		st:           d.Store,
		cfg:          d.Config,
		configPath:   d.ConfigPath,
		auth:         d.Auth,
		alerts:       d.Alerts,
		integrations: d.Integrations,
		vpn:          d.VPN,
		reports:      d.Reports,
		backups:      d.Backups,
		scanner:      d.Scanner,
		log:          log,
		hub:          NewHub(log),
		limiter:      newIPLimiter(rateLimitFor(d.Config)),
		version:      d.Version,
		commit:       d.Commit,
		buildDate:    d.BuildDate,
		startedAt:    d.StartedAt,
		mux:          http.NewServeMux(),
	}
	if s.startedAt.IsZero() {
		s.startedAt = time.Now()
	}
	s.register()
	return s
}

// Hub returns the event hub so the app can publish scan progress.
func (s *Server) Hub() *Hub { return s.hub }

// Handler returns the fully wrapped API handler.
func (s *Server) Handler() http.Handler {
	return s.middleware(s.mux)
}

// SetConfig swaps the live configuration after a settings update.
func (s *Server) SetConfig(cfg *config.Config) {
	s.cfg = cfg
	s.limiter.setLimit(rateLimitFor(cfg))
}

func rateLimitFor(cfg *config.Config) int {
	if cfg == nil {
		return 600
	}
	return cfg.Security.APIRate
}

func (s *Server) register() {
	m := s.mux

	// ---- auth ----
	m.HandleFunc("GET /api/v1/auth/status", s.handleAuthStatus)
	m.HandleFunc("POST /api/v1/auth/login", s.handleLogin)
	m.HandleFunc("POST /api/v1/auth/setup", s.handleSetup)
	m.HandleFunc("POST /api/v1/auth/logout", s.handleLogout)
	m.HandleFunc("POST /api/v1/auth/password", s.handleChangePassword)
	m.HandleFunc("GET /api/v1/auth/users", s.handleListUsers)
	m.HandleFunc("POST /api/v1/auth/users", s.handleCreateUser)
	m.HandleFunc("PATCH /api/v1/auth/users/{id}", s.handleUpdateUser)
	m.HandleFunc("DELETE /api/v1/auth/users/{id}", s.handleDeleteUser)
	m.HandleFunc("POST /api/v1/auth/users/{id}/password", s.handleResetUserPassword)

	// ---- dashboard ----
	m.HandleFunc("GET /api/v1/dashboard", s.handleDashboard)

	// ---- sites ----
	m.HandleFunc("GET /api/v1/sites", s.handleListSites)
	m.HandleFunc("POST /api/v1/sites", s.handleCreateSite)
	m.HandleFunc("GET /api/v1/sites/{id}", s.handleGetSite)
	m.HandleFunc("PUT /api/v1/sites/{id}", s.handleUpdateSite)
	m.HandleFunc("DELETE /api/v1/sites/{id}", s.handleDeleteSite)
	m.HandleFunc("GET /api/v1/sites/{id}/export", s.handleExportSite)
	m.HandleFunc("POST /api/v1/sites/{id}/import", s.handleImportSite)

	// ---- hosts ----
	m.HandleFunc("GET /api/v1/hosts", s.handleListHosts)
	m.HandleFunc("GET /api/v1/hosts/export", s.handleExportHosts)
	m.HandleFunc("GET /api/v1/hosts/{id}", s.handleGetHost)
	m.HandleFunc("PATCH /api/v1/hosts/{id}", s.handleUpdateHost)
	m.HandleFunc("DELETE /api/v1/hosts/{id}", s.handleDeleteHost)
	m.HandleFunc("GET /api/v1/hosts/{id}/events", s.handleHostEvents)
	m.HandleFunc("GET /api/v1/hosts/{id}/history", s.handleHostHistory)
	m.HandleFunc("GET /api/v1/hosts/{id}/neighbors", s.handleHostNeighbors)
	m.HandleFunc("POST /api/v1/hosts/{id}/rescan", s.handleRescanHost)
	m.HandleFunc("GET /api/v1/ports/summary", s.handlePortsSummary)

	// ---- networks ----
	m.HandleFunc("GET /api/v1/networks", s.handleListNetworks)
	m.HandleFunc("POST /api/v1/networks", s.handleCreateNetwork)
	m.HandleFunc("GET /api/v1/networks/suggest", s.handleSuggestNetworks)
	m.HandleFunc("PUT /api/v1/networks/{id}", s.handleUpdateNetwork)
	m.HandleFunc("DELETE /api/v1/networks/{id}", s.handleDeleteNetwork)

	// ---- scans ----
	m.HandleFunc("GET /api/v1/scans", s.handleListScans)
	m.HandleFunc("POST /api/v1/scans", s.handleStartScan)
	m.HandleFunc("GET /api/v1/scans/{id}", s.handleGetScan)
	m.HandleFunc("POST /api/v1/scans/{id}/cancel", s.handleCancelScan)

	// ---- alerts, rules and channels ----
	m.HandleFunc("GET /api/v1/alerts", s.handleListAlerts)
	m.HandleFunc("GET /api/v1/alerts/stats", s.handleAlertStats)
	m.HandleFunc("POST /api/v1/alerts/{id}/ack", s.handleAckAlert)
	m.HandleFunc("POST /api/v1/alerts/{id}/resolve", s.handleResolveAlert)
	m.HandleFunc("GET /api/v1/alerts/rules", s.handleListRules)
	m.HandleFunc("POST /api/v1/alerts/rules", s.handleCreateRule)
	m.HandleFunc("PUT /api/v1/alerts/rules/{id}", s.handleUpdateRule)
	m.HandleFunc("DELETE /api/v1/alerts/rules/{id}", s.handleDeleteRule)
	m.HandleFunc("GET /api/v1/alerts/channels", s.handleListChannels)
	m.HandleFunc("POST /api/v1/alerts/channels", s.handleCreateChannel)
	m.HandleFunc("PUT /api/v1/alerts/channels/{id}", s.handleUpdateChannel)
	m.HandleFunc("DELETE /api/v1/alerts/channels/{id}", s.handleDeleteChannel)
	m.HandleFunc("POST /api/v1/alerts/channels/{id}/test", s.handleTestChannel)

	// ---- external integrations ----
	m.HandleFunc("GET /api/v1/integrations", s.handleListIntegrations)
	m.HandleFunc("POST /api/v1/integrations", s.handleCreateIntegration)
	m.HandleFunc("PUT /api/v1/integrations/{id}", s.handleUpdateIntegration)
	m.HandleFunc("DELETE /api/v1/integrations/{id}", s.handleDeleteIntegration)
	m.HandleFunc("POST /api/v1/integrations/{id}/test", s.handleTestIntegration)

	// ---- F12 VPN tunnels ----
	m.HandleFunc("GET /api/v1/vpn/tunnels", s.handleListVPNTunnels)
	m.HandleFunc("POST /api/v1/vpn/scan", s.handleVPNScan)
	m.HandleFunc("DELETE /api/v1/vpn/tunnels/{id}", s.handleDeleteVPNTunnel)

	// ---- reports ----
	m.HandleFunc("GET /api/v1/reports", s.handleListReports)
	m.HandleFunc("POST /api/v1/reports", s.handleCreateReport)
	m.HandleFunc("GET /api/v1/reports/{id}/download", s.handleDownloadReport)
	m.HandleFunc("DELETE /api/v1/reports/{id}", s.handleDeleteReport)

	// ---- topology ----
	m.HandleFunc("GET /api/v1/topology", s.handleTopology)

	// ---- enterprise diagnostics ----
	m.HandleFunc("GET /api/v1/diagnostics/enterprise", s.handleEnterpriseDiagnostics)
	m.HandleFunc("GET /api/v1/diagnostics/stp", s.handleSTPDiagnostics)
	m.HandleFunc("GET /api/v1/diagnostics/vlan", s.handleVLANDiagnostics)

	// ---- system ----
	m.HandleFunc("GET /api/v1/system/info", s.handleSystemInfo)
	m.HandleFunc("GET /api/v1/system/interfaces", s.handleSystemInterfaces)
	m.HandleFunc("GET /api/v1/system/config", s.handleGetConfig)
	m.HandleFunc("PUT /api/v1/system/config", s.handleUpdateConfig)
	m.HandleFunc("GET /api/v1/system/audit", s.handleAudit)
	m.HandleFunc("GET /api/v1/system/health", s.handleHealth)
	m.HandleFunc("GET /api/v1/system/backups", s.handleListBackups)
	m.HandleFunc("POST /api/v1/system/backups", s.handleCreateBackup)
	m.HandleFunc("GET /api/v1/system/backups/{name}/download", s.handleDownloadBackup)
	m.HandleFunc("DELETE /api/v1/system/backups/{name}", s.handleDeleteBackup)
	m.HandleFunc("GET /api/v1/ws", s.handleWS)

	// ---- public operational endpoints ----
	m.HandleFunc("GET /api/v1/openapi.json", s.handleOpenAPI)
	m.HandleFunc("GET /metrics", s.handleMetrics)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"version":  s.version,
		"uptime_s": int64(time.Since(s.startedAt).Seconds()),
	})
}

func (s *Server) handleSystemInterfaces(w http.ResponseWriter, r *http.Request) {
	ifaces, err := s.systemInterfaces()
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "no se pudieron leer las interfaces: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": ifaces})
}

// dbSize returns the database file size, or 0 when unknown.
func (s *Server) dbSize() int64 {
	info, err := os.Stat(s.st.Path())
	if err != nil {
		return 0
	}
	return info.Size()
}
