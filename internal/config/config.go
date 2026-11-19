// Package config loads, validates and persists NetDiag configuration.
//
// Precedence is: defaults < YAML file < environment variables < CLI flags.
// The configuration file contains no secrets; alert channel credentials live
// encrypted in the database.
package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Duration is a YAML/JSON friendly time.Duration.
type Duration time.Duration

// Duration returns the underlying time.Duration.
func (d Duration) Duration() time.Duration { return time.Duration(d) }

// MarshalYAML implements yaml.Marshaler.
func (d Duration) MarshalYAML() (any, error) { return time.Duration(d).String(), nil }

// UnmarshalYAML implements yaml.Unmarshaler.
func (d *Duration) UnmarshalYAML(node *yaml.Node) error {
	var s string
	if err := node.Decode(&s); err != nil {
		var n int64
		if err2 := node.Decode(&n); err2 == nil {
			*d = Duration(time.Duration(n))
			return nil
		}
		return err
	}
	v, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("duración inválida %q: %w", s, err)
	}
	*d = Duration(v)
	return nil
}

// MarshalJSON implements json.Marshaler.
func (d Duration) MarshalJSON() ([]byte, error) {
	return []byte(strconv.Quote(time.Duration(d).String())), nil
}

// UnmarshalJSON implements json.Unmarshaler.
func (d *Duration) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		return nil
	}
	v, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	*d = Duration(v)
	return nil
}

// Config is the root configuration document.
type Config struct {
	Server       Server       `yaml:"server" json:"server"`
	Scan         Scan         `yaml:"scan" json:"scan"`
	Monitor      Monitor      `yaml:"monitor" json:"monitor"`
	Alerts       Alerts       `yaml:"alerts" json:"alerts"`
	Integrations Integrations `yaml:"integrations" json:"integrations"`
	Update       Update       `yaml:"update" json:"update"`
	Storage      Storage      `yaml:"storage" json:"storage"`
	Security     Security     `yaml:"security" json:"security"`
	Diagnostics  Diagnostics  `yaml:"diagnostics" json:"diagnostics"`
	VPN          VPN          `yaml:"vpn" json:"vpn"`
	WiFi         WiFi         `yaml:"wifi" json:"wifi"`
	Log          Log          `yaml:"log" json:"log"`
}

// Server holds HTTP server settings.
type Server struct {
	Listen       string   `yaml:"listen" json:"listen"`
	PublicURL    string   `yaml:"public_url" json:"public_url"`
	TLSCert      string   `yaml:"tls_cert" json:"tls_cert"`
	TLSKey       string   `yaml:"tls_key" json:"tls_key"`
	ReadTimeout  Duration `yaml:"read_timeout" json:"read_timeout"`
	WriteTimeout Duration `yaml:"write_timeout" json:"write_timeout"`
	IdleTimeout  Duration `yaml:"idle_timeout" json:"idle_timeout"`
	MaxBodyMB    int      `yaml:"max_body_mb" json:"max_body_mb"`
	EnableCSP    bool     `yaml:"enable_csp" json:"enable_csp"`
	EnableHSTS   bool     `yaml:"enable_hsts" json:"enable_hsts"`
}

// Scan holds discovery defaults.
type Scan struct {
	Profile         string   `yaml:"profile" json:"profile"`
	Timeout         Duration `yaml:"timeout" json:"timeout"`
	Concurrency     int      `yaml:"concurrency" json:"concurrency"`
	PortConcurrency int      `yaml:"port_concurrency" json:"port_concurrency"`
	MaxHosts        int      `yaml:"max_hosts" json:"max_hosts"`
	ResolveDNS      bool     `yaml:"resolve_dns" json:"resolve_dns"`
	BannerGrab      bool     `yaml:"banner_grab" json:"banner_grab"`
	AllowPublic     bool     `yaml:"allow_public" json:"allow_public"`
	SNMP            bool     `yaml:"snmp" json:"snmp"`
	SNMPCommunities []string `yaml:"snmp_communities" json:"snmp_communities"`
	SNMPTimeout     Duration `yaml:"snmp_timeout" json:"snmp_timeout"`
	SNMPVersion     string   `yaml:"snmp_version" json:"snmp_version"`
	SNMPv3User      string   `yaml:"snmp_v3_user" json:"snmp_v3_user"`
	SNMPv3Auth      string   `yaml:"snmp_v3_auth" json:"snmp_v3_auth"`
	SNMPv3Priv      string   `yaml:"snmp_v3_priv" json:"snmp_v3_priv"`
	SNMPv3AuthProto string   `yaml:"snmp_v3_auth_proto" json:"snmp_v3_auth_proto"`
	SNMPv3PrivProto string   `yaml:"snmp_v3_priv_proto" json:"snmp_v3_priv_proto"`
	AutoDiscoverL2  bool     `yaml:"auto_discover_l2" json:"auto_discover_l2"`
}

// Monitor holds the periodic scan scheduler.
type Monitor struct {
	Enabled      bool     `yaml:"enabled" json:"enabled"`
	Interval     Duration `yaml:"interval" json:"interval"`
	NetworkIDs   []int64  `yaml:"network_ids" json:"network_ids"`
	DeepInterval Duration `yaml:"deep_interval" json:"deep_interval"`
}

// Alerts holds alert engine settings.
type Alerts struct {
	Enabled             bool     `yaml:"enabled" json:"enabled"`
	WebhookTimeout      Duration `yaml:"webhook_timeout" json:"webhook_timeout"`
	AllowPrivateTargets bool     `yaml:"allow_private_targets" json:"allow_private_targets"`
	SMTPInsecure        bool     `yaml:"smtp_insecure" json:"smtp_insecure"`
	MaxPerMinute        int      `yaml:"max_per_minute" json:"max_per_minute"`
}

// Integrations configures the outbound event exporters. Credentials are
// stored encrypted per integration, never in this file.
type Integrations struct {
	Enabled             bool     `yaml:"enabled" json:"enabled"`
	AllowPrivateTargets bool     `yaml:"allow_private_targets" json:"allow_private_targets"`
	Timeout             Duration `yaml:"timeout" json:"timeout"`
	MaxPerMinute        int      `yaml:"max_per_minute" json:"max_per_minute"`
	QueueSize           int      `yaml:"queue_size" json:"queue_size"`
}

// Update configures F23 self-updates. The public key is a minisign public key
// (base64 of "Ed" || key_id || Ed25519 public key, or the full two-line file).
// No credentials belong here: manifests are downloaded and verified
// unauthenticated, and the signing key is intentionally not stored.
type Update struct {
	Enabled            bool     `yaml:"enabled" json:"enabled"`
	ManifestURL        string   `yaml:"manifest_url" json:"manifest_url"`
	PublicKey          string   `yaml:"public_key" json:"public_key"`
	CheckIntervalHours int      `yaml:"check_interval_hours" json:"check_interval_hours"`
	DownloadTimeout    Duration `yaml:"download_timeout" json:"download_timeout"`
	MaxDownloadMB      int      `yaml:"max_download_mb" json:"max_download_mb"`
	RestartCommand     string   `yaml:"restart_command" json:"restart_command"`
	RestartArgs        []string `yaml:"restart_args" json:"restart_args"`
}

// Storage holds persistence settings.
type Storage struct {
	DataDir        string   `yaml:"data_dir" json:"data_dir"`
	DB             string   `yaml:"db" json:"db"`
	RetentionDays  int      `yaml:"retention_days" json:"retention_days"`
	MetricsDays    int      `yaml:"metrics_days" json:"metrics_days"`
	AuditDays      int      `yaml:"audit_days" json:"audit_days"`
	MaxScans       int      `yaml:"max_scans" json:"max_scans"`
	BackupEnabled  bool     `yaml:"backup_enabled" json:"backup_enabled"`
	BackupDir      string   `yaml:"backup_dir" json:"backup_dir"`
	BackupInterval Duration `yaml:"backup_interval" json:"backup_interval"`
	BackupKeep     int      `yaml:"backup_keep" json:"backup_keep"`
}

// Security holds auth and hardening settings.
type Security struct {
	Auth         bool     `yaml:"auth" json:"auth"`
	SessionHours int      `yaml:"session_hours" json:"session_hours"`
	LoginRate    int      `yaml:"login_rate" json:"login_rate"` // attempts per minute per IP
	APIRate      int      `yaml:"api_rate" json:"api_rate"`     // requests per minute per IP
	TrustProxy   bool     `yaml:"trust_proxy" json:"trust_proxy"`
	AllowedHosts []string `yaml:"allowed_hosts" json:"allowed_hosts"`
	MetricsToken string   `yaml:"metrics_token" json:"metrics_token"`
}

// Log holds logging settings.
// Diagnostics tunes the enterprise heuristics. They never sniff raw frames:
// findings come from adjacency, IF-MIB, inventory, metrics, ARP history and a
// short DHCP UDP probe on the local segment.
type Diagnostics struct {
	EnterpriseMode        bool     `yaml:"enterprise_mode" json:"enterprise_mode"`
	MinConfidence         int      `yaml:"min_confidence" json:"min_confidence"`
	MinAvailability       float64  `yaml:"min_availability" json:"min_availability"`
	MaxRTTMS              float64  `yaml:"max_rtt_ms" json:"max_rtt_ms"`
	MaxAnomalies          int      `yaml:"max_anomalies" json:"max_anomalies"`
	StormHosts            int      `yaml:"storm_hosts" json:"storm_hosts"`
	AuthorizedAPs         []string `yaml:"authorized_aps" json:"authorized_aps"`
	ARPHistoryDays        int      `yaml:"arp_history_days" json:"arp_history_days"`
	MACIPLimit            int      `yaml:"mac_ip_limit" json:"mac_ip_limit"`
	DHCPProbe             bool     `yaml:"dhcp_probe" json:"dhcp_probe"`
	DHCPProbeTimeout      Duration `yaml:"dhcp_probe_timeout" json:"dhcp_probe_timeout"`
	DHCPHistoryDays       int      `yaml:"dhcp_history_days" json:"dhcp_history_days"`
	AuthorizedDHCPServers []string `yaml:"authorized_dhcp_servers" json:"authorized_dhcp_servers"`
}

// VPN controls the periodic heuristic detection of VPN tunnels. Detection is
// passive: it reads local kernel tables and never injects or decrypts traffic.
type VPN struct {
	Enabled       bool     `yaml:"enabled" json:"enabled"`
	Interval      Duration `yaml:"interval" json:"interval"`
	MinConfidence int      `yaml:"min_confidence" json:"min_confidence"`
	StaleAfter    Duration `yaml:"stale_after" json:"stale_after"`
}

// WiFi configures passive 802.11 monitoring. The interface must already be in
// monitor mode; NetDiag never changes the device type.
type WiFi struct {
	Enabled          bool     `yaml:"enabled" json:"enabled"`
	Interface        string   `yaml:"interface" json:"interface"`
	Interval         Duration `yaml:"interval" json:"interval"`
	ScanSeconds      int      `yaml:"scan_seconds" json:"scan_seconds"`
	Channels         []int    `yaml:"channels" json:"channels"`
	AuthorizedBSSIDs []string `yaml:"authorized_bssids" json:"authorized_bssids"`
	KnownSSIDs       []string `yaml:"known_ssids" json:"known_ssids"`
}

type Log struct {
	Level  string `yaml:"level" json:"level"`
	Format string `yaml:"format" json:"format"` // text | json
}

// DefaultPath returns the configuration file location.
func DefaultPath() string {
	if p := os.Getenv("NETDIAG_CONFIG"); p != "" {
		return p
	}
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "netdiag", "config.yaml")
}

// DefaultDataDir returns the default data directory.
func DefaultDataDir() string {
	if p := os.Getenv("NETDIAG_DATA_DIR"); p != "" {
		return p
	}
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(base, "netdiag")
}

// Defaults returns a fully populated configuration.
func Defaults() *Config {
	return &Config{
		Server: Server{
			Listen:       "127.0.0.1:8080",
			ReadTimeout:  Duration(20 * time.Second),
			WriteTimeout: Duration(60 * time.Second),
			IdleTimeout:  Duration(120 * time.Second),
			MaxBodyMB:    2,
			EnableCSP:    true,
		},
		Scan: Scan{
			Profile:         "common",
			Timeout:         Duration(900 * time.Millisecond),
			Concurrency:     128,
			PortConcurrency: 256,
			MaxHosts:        4096,
			ResolveDNS:      true,
			BannerGrab:      true,
			SNMP:            false,
			SNMPCommunities: []string{"public"},
			SNMPTimeout:     Duration(time.Second),
			SNMPVersion:     "2c",
			SNMPv3AuthProto: "sha",
			SNMPv3PrivProto: "aes",
			AutoDiscoverL2:  true,
		},
		Monitor: Monitor{
			Enabled:      true,
			Interval:     Duration(5 * time.Minute),
			DeepInterval: Duration(6 * time.Hour),
		},
		Alerts: Alerts{
			Enabled:        true,
			WebhookTimeout: Duration(10 * time.Second),
			MaxPerMinute:   30,
		},
		Integrations: Integrations{
			Enabled:      true,
			Timeout:      Duration(10 * time.Second),
			MaxPerMinute: 60,
			QueueSize:    512,
		},
		Update: Update{
			Enabled:            false,
			CheckIntervalHours: 24,
			DownloadTimeout:    Duration(120 * time.Second),
			MaxDownloadMB:      512,
		},
		VPN: VPN{
			Enabled:       true,
			Interval:      Duration(5 * time.Minute),
			MinConfidence: 60,
			StaleAfter:    Duration(30 * time.Minute),
		},
		WiFi: WiFi{
			Enabled:     false,
			Interval:    Duration(5 * time.Minute),
			ScanSeconds: 10,
		},
		Storage: Storage{
			DataDir:        DefaultDataDir(),
			DB:             "netdiag.db",
			RetentionDays:  30,
			MetricsDays:    30,
			AuditDays:      90,
			MaxScans:       500,
			BackupEnabled:  true,
			BackupInterval: Duration(24 * time.Hour),
			BackupKeep:     7,
		},
		Security: Security{
			Auth:         true,
			SessionHours: 12,
			LoginRate:    5,
			APIRate:      600,
		},
		Diagnostics: Diagnostics{
			EnterpriseMode:        true,
			MinConfidence:         60,
			MinAvailability:       99,
			MaxRTTMS:              200,
			MaxAnomalies:          200,
			StormHosts:            3,
			ARPHistoryDays:        30,
			MACIPLimit:            5,
			DHCPProbe:             true,
			DHCPProbeTimeout:      Duration(700 * time.Millisecond),
			DHCPHistoryDays:       30,
			AuthorizedDHCPServers: []string{},
		},
		Log: Log{Level: "info", Format: "text"},
	}
}

// Load reads configuration from path (or the default path when empty).
// A missing file is not an error: defaults are used.
func Load(path string) (*Config, error) {
	cfg := Defaults()
	if path == "" {
		path = DefaultPath()
	}
	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("config %s: %w", path, err)
		}
	case errors.Is(err, os.ErrNotExist):
		// defaults
	default:
		return nil, fmt.Errorf("config %s: %w", path, err)
	}
	applyEnv(cfg)
	if err := cfg.Normalize(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Normalize expands paths and applies derived defaults.
func (c *Config) Normalize() error {
	if c.Storage.DataDir == "" {
		c.Storage.DataDir = DefaultDataDir()
	}
	if strings.HasPrefix(c.Storage.DataDir, "~") {
		home, _ := os.UserHomeDir()
		c.Storage.DataDir = filepath.Join(home, strings.TrimPrefix(c.Storage.DataDir, "~"))
	}
	abs, err := filepath.Abs(c.Storage.DataDir)
	if err != nil {
		return err
	}
	c.Storage.DataDir = abs
	if c.Storage.DB == "" {
		c.Storage.DB = "netdiag.db"
	}
	if !filepath.IsAbs(c.Storage.DB) {
		c.Storage.DB = filepath.Join(c.Storage.DataDir, c.Storage.DB)
	}
	if c.Storage.BackupDir == "" {
		c.Storage.BackupDir = filepath.Join(c.Storage.DataDir, "backups")
	}
	if strings.HasPrefix(c.Storage.BackupDir, "~") {
		home, _ := os.UserHomeDir()
		c.Storage.BackupDir = filepath.Join(home, strings.TrimPrefix(c.Storage.BackupDir, "~"))
	}
	if !filepath.IsAbs(c.Storage.BackupDir) {
		c.Storage.BackupDir = filepath.Join(c.Storage.DataDir, c.Storage.BackupDir)
	}
	if c.Storage.BackupInterval.Duration() <= 0 {
		c.Storage.BackupInterval = Duration(24 * time.Hour)
	}
	if c.Storage.BackupKeep <= 0 {
		c.Storage.BackupKeep = 7
	}
	if strings.HasPrefix(c.Server.TLSCert, "~") {
		home, _ := os.UserHomeDir()
		c.Server.TLSCert = filepath.Join(home, strings.TrimPrefix(c.Server.TLSCert, "~"))
	}
	if strings.HasPrefix(c.Server.TLSKey, "~") {
		home, _ := os.UserHomeDir()
		c.Server.TLSKey = filepath.Join(home, strings.TrimPrefix(c.Server.TLSKey, "~"))
	}
	c.Scan.Profile = strings.ToLower(strings.TrimSpace(c.Scan.Profile))
	if c.Scan.Profile == "" {
		c.Scan.Profile = "common"
	}
	if c.Log.Level == "" {
		c.Log.Level = "info"
	}
	if c.Log.Format == "" {
		c.Log.Format = "text"
	}
	if len(c.Scan.SNMPCommunities) == 0 {
		c.Scan.SNMPCommunities = []string{"public"}
	}
	if len(c.Security.AllowedHosts) > 0 {
		seen := make(map[string]bool, len(c.Security.AllowedHosts))
		hosts := make([]string, 0, len(c.Security.AllowedHosts))
		for _, h := range c.Security.AllowedHosts {
			h = strings.TrimSuffix(strings.ToLower(strings.Trim(strings.TrimSpace(h), "[]")), ".")
			if h == "" || seen[h] {
				continue
			}
			seen[h] = true
			hosts = append(hosts, h)
		}
		c.Security.AllowedHosts = hosts
	}
	if c.Diagnostics.MinConfidence <= 0 {
		c.Diagnostics.MinConfidence = 60
	}
	if c.Diagnostics.MinAvailability <= 0 {
		c.Diagnostics.MinAvailability = 99
	}
	if c.Diagnostics.MaxRTTMS <= 0 {
		c.Diagnostics.MaxRTTMS = 200
	}
	if c.Diagnostics.MaxAnomalies <= 0 {
		c.Diagnostics.MaxAnomalies = 200
	}
	if c.Diagnostics.StormHosts <= 0 {
		c.Diagnostics.StormHosts = 3
	}
	aps := make([]string, 0, len(c.Diagnostics.AuthorizedAPs))
	for _, mac := range c.Diagnostics.AuthorizedAPs {
		mac = strings.ToLower(strings.TrimSpace(mac))
		if mac != "" {
			aps = append(aps, mac)
		}
	}
	c.Diagnostics.AuthorizedAPs = aps
	if c.Diagnostics.DHCPProbeTimeout.Duration() <= 0 {
		c.Diagnostics.DHCPProbeTimeout = Duration(700 * time.Millisecond)
	}
	if c.Diagnostics.DHCPHistoryDays < 1 {
		c.Diagnostics.DHCPHistoryDays = 30
	}
	dhcpServers := make([]string, 0, len(c.Diagnostics.AuthorizedDHCPServers))
	seenDHCP := map[string]bool{}
	for _, value := range c.Diagnostics.AuthorizedDHCPServers {
		value = strings.ToLower(strings.TrimSpace(value))
		if value != "" && !seenDHCP[value] {
			seenDHCP[value] = true
			dhcpServers = append(dhcpServers, value)
		}
	}
	c.Diagnostics.AuthorizedDHCPServers = dhcpServers
	if c.Integrations.Timeout.Duration() <= 0 {
		c.Integrations.Timeout = Duration(10 * time.Second)
	}
	if c.Integrations.MaxPerMinute <= 0 {
		c.Integrations.MaxPerMinute = 60
	}
	if c.Integrations.QueueSize <= 0 {
		c.Integrations.QueueSize = 512
	}
	c.Update.ManifestURL = strings.TrimSpace(c.Update.ManifestURL)
	c.Update.PublicKey = minisignPublicKeyB64(c.Update.PublicKey)
	c.Update.RestartCommand = strings.TrimSpace(c.Update.RestartCommand)
	if c.Update.CheckIntervalHours <= 0 {
		c.Update.CheckIntervalHours = 24
	}
	if c.Update.DownloadTimeout.Duration() <= 0 {
		c.Update.DownloadTimeout = Duration(120 * time.Second)
	}
	if c.Update.MaxDownloadMB <= 0 {
		c.Update.MaxDownloadMB = 512
	}
	if c.VPN.Interval.Duration() <= 0 {
		c.VPN.Interval = Duration(5 * time.Minute)
	}
	if c.VPN.MinConfidence <= 0 {
		c.VPN.MinConfidence = 60
	}
	if c.VPN.StaleAfter.Duration() <= 0 {
		c.VPN.StaleAfter = Duration(30 * time.Minute)
	}
	if c.WiFi.Interval.Duration() <= 0 {
		c.WiFi.Interval = Duration(5 * time.Minute)
	}
	if c.WiFi.ScanSeconds <= 0 {
		c.WiFi.ScanSeconds = 10
	}
	authorized := make([]string, 0, len(c.WiFi.AuthorizedBSSIDs))
	for _, mac := range c.WiFi.AuthorizedBSSIDs {
		mac = strings.ToLower(strings.TrimSpace(mac))
		if mac != "" {
			authorized = append(authorized, mac)
		}
	}
	c.WiFi.AuthorizedBSSIDs = authorized
	known := make([]string, 0, len(c.WiFi.KnownSSIDs))
	for _, ssid := range c.WiFi.KnownSSIDs {
		ssid = strings.TrimSpace(ssid)
		if ssid != "" {
			known = append(known, ssid)
		}
	}
	c.WiFi.KnownSSIDs = known
	return c.Validate()
}

// Validate checks values that would otherwise fail at runtime.
func (c *Config) Validate() error {
	var errs []string
	if _, _, err := net.SplitHostPort(c.Server.Listen); err != nil {
		errs = append(errs, "server.listen inválido: "+err.Error())
	}
	switch c.Scan.Profile {
	case "quick", "common", "full", "custom":
	default:
		errs = append(errs, "scan.profile debe ser quick|common|full|custom")
	}
	if c.Scan.Concurrency < 1 || c.Scan.Concurrency > 1024 {
		errs = append(errs, "scan.concurrency debe estar entre 1 y 1024")
	}
	if c.Scan.PortConcurrency < 1 || c.Scan.PortConcurrency > 2048 {
		errs = append(errs, "scan.port_concurrency debe estar entre 1 y 2048")
	}
	if c.Scan.MaxHosts < 1 || c.Scan.MaxHosts > 65536 {
		errs = append(errs, "scan.max_hosts debe estar entre 1 y 65536")
	}
	if c.Scan.Timeout.Duration() < 50*time.Millisecond || c.Scan.Timeout.Duration() > 10*time.Second {
		errs = append(errs, "scan.timeout debe estar entre 50ms y 10s")
	}
	if c.Monitor.Interval.Duration() < 30*time.Second {
		errs = append(errs, "monitor.interval mínimo 30s")
	}
	if c.Security.SessionHours < 1 || c.Security.SessionHours > 720 {
		errs = append(errs, "security.session_hours debe estar entre 1 y 720")
	}
	if c.Storage.RetentionDays < 1 {
		errs = append(errs, "storage.retention_days debe ser >= 1")
	}
	if c.Storage.BackupEnabled {
		if c.Storage.BackupInterval.Duration() < 15*time.Minute {
			errs = append(errs, "storage.backup_interval mínimo 15m")
		}
		if c.Storage.BackupKeep < 1 || c.Storage.BackupKeep > 365 {
			errs = append(errs, "storage.backup_keep debe estar entre 1 y 365")
		}
	}
	if c.Diagnostics.MinConfidence < 1 || c.Diagnostics.MinConfidence > 100 {
		errs = append(errs, "diagnostics.min_confidence debe estar entre 1 y 100")
	}
	if c.Diagnostics.MinAvailability < 1 || c.Diagnostics.MinAvailability > 100 {
		errs = append(errs, "diagnostics.min_availability debe estar entre 1 y 100")
	}
	if c.Diagnostics.MaxRTTMS < 1 || c.Diagnostics.MaxRTTMS > 60000 {
		errs = append(errs, "diagnostics.max_rtt_ms debe estar entre 1 y 60000")
	}
	if c.Diagnostics.MaxAnomalies < 1 || c.Diagnostics.MaxAnomalies > 1000 {
		errs = append(errs, "diagnostics.max_anomalies debe estar entre 1 y 1000")
	}
	if c.Diagnostics.StormHosts < 2 || c.Diagnostics.StormHosts > 100 {
		errs = append(errs, "diagnostics.storm_hosts debe estar entre 2 y 100")
	}
	if d := c.Diagnostics.DHCPProbeTimeout.Duration(); d < 100*time.Millisecond || d > 5*time.Second {
		errs = append(errs, "diagnostics.dhcp_probe_timeout debe estar entre 100ms y 5s")
	}
	if c.Diagnostics.DHCPHistoryDays < 1 || c.Diagnostics.DHCPHistoryDays > 3650 {
		errs = append(errs, "diagnostics.dhcp_history_days debe estar entre 1 y 3650")
	}
	for _, value := range c.Diagnostics.AuthorizedDHCPServers {
		if net.ParseIP(value) == nil {
			if _, err := net.ParseMAC(value); err != nil {
				errs = append(errs, "diagnostics.authorized_dhcp_servers contiene un valor inválido: "+value)
			}
		}
	}
	if c.Integrations.Timeout.Duration() < time.Second || c.Integrations.Timeout.Duration() > time.Minute {
		errs = append(errs, "integrations.timeout debe estar entre 1s y 1m")
	}
	if c.Integrations.MaxPerMinute < 1 || c.Integrations.MaxPerMinute > 10000 {
		errs = append(errs, "integrations.max_per_minute debe estar entre 1 y 10000")
	}
	if c.Integrations.QueueSize < 16 || c.Integrations.QueueSize > 100000 {
		errs = append(errs, "integrations.queue_size debe estar entre 16 y 100000")
	}
	if c.VPN.Interval.Duration() < 30*time.Second {
		errs = append(errs, "vpn.interval mínimo 30s")
	}
	if c.VPN.MinConfidence < 1 || c.VPN.MinConfidence > 100 {
		errs = append(errs, "vpn.min_confidence debe estar entre 1 y 100")
	}
	if c.VPN.StaleAfter.Duration() < time.Minute {
		errs = append(errs, "vpn.stale_after mínimo 1m")
	}
	if c.WiFi.Interval.Duration() < 30*time.Second {
		errs = append(errs, "wifi.interval mínimo 30s")
	}
	if c.WiFi.ScanSeconds < 1 || c.WiFi.ScanSeconds > 300 {
		errs = append(errs, "wifi.scan_seconds debe estar entre 1 y 300")
	}
	if c.Update.CheckIntervalHours < 1 || c.Update.CheckIntervalHours > 720 {
		errs = append(errs, "update.check_interval_hours debe estar entre 1 y 720")
	}
	if c.Update.DownloadTimeout.Duration() < time.Second || c.Update.DownloadTimeout.Duration() > 30*time.Minute {
		errs = append(errs, "update.download_timeout debe estar entre 1s y 30m")
	}
	if c.Update.MaxDownloadMB < 1 || c.Update.MaxDownloadMB > 8192 {
		errs = append(errs, "update.max_download_mb debe estar entre 1 y 8192")
	}
	if strings.ContainsRune(c.Update.RestartCommand, 0) {
		errs = append(errs, "update.restart_command contiene caracteres NUL")
	}
	for _, arg := range c.Update.RestartArgs {
		if strings.ContainsRune(arg, 0) {
			errs = append(errs, "update.restart_args contiene caracteres NUL")
			break
		}
	}
	if c.Update.Enabled {
		if c.Update.ManifestURL == "" {
			errs = append(errs, "update.manifest_url es obligatoria cuando update.enabled=true")
		} else {
			u, err := url.Parse(c.Update.ManifestURL)
			if err != nil || u.Hostname() == "" || (u.Scheme != "https" && u.Scheme != "http") {
				errs = append(errs, "update.manifest_url debe ser una URL http(s) válida")
			} else if u.User != nil {
				errs = append(errs, "update.manifest_url no puede incluir credenciales")
			} else if u.Scheme == "http" && !isLocalhost(u.Hostname()) {
				errs = append(errs, "update.manifest_url debe usar HTTPS salvo en localhost")
			}
		}
		if c.Update.PublicKey == "" {
			errs = append(errs, "update.public_key es obligatoria cuando update.enabled=true")
		} else {
			raw, err := base64.StdEncoding.DecodeString(c.Update.PublicKey)
			if err != nil || len(raw) != 42 || string(raw[:2]) != "Ed" {
				errs = append(errs, "update.public_key debe ser una clave pública minisign válida")
			}
		}
	}
	for _, ch := range c.WiFi.Channels {
		if ch < 1 || ch > 233 {
			errs = append(errs, "wifi.channels contiene un canal fuera de rango (1-233)")
			break
		}
	}
	if (c.Server.TLSCert == "") != (c.Server.TLSKey == "") {
		errs = append(errs, "server.tls_cert y server.tls_key deben configurarse juntos")
	}
	if len(errs) > 0 {
		return errors.New("configuración inválida:\n - " + strings.Join(errs, "\n - "))
	}
	return nil
}

// Save writes the configuration atomically with 0600 permissions.
func (c *Config) Save(path string) error {
	if path == "" {
		path = DefaultPath()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// EnsureDirs creates the data directory structure.
func (c *Config) EnsureDirs() error {
	dirs := []string{c.Storage.DataDir, filepath.Join(c.Storage.DataDir, "reports"), filepath.Join(c.Storage.DataDir, "logs")}
	if c.Storage.BackupEnabled {
		dirs = append(dirs, c.Storage.BackupDir)
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o700); err != nil {
			return err
		}
	}
	return nil
}

// minisignPublicKeyB64 accepts either the bare base64 key or a complete
// two-line minisign public key file and returns just the encoded key.
func minisignPublicKeyB64(s string) string {
	s = strings.TrimSpace(s)
	if !strings.ContainsAny(s, "\r\n") {
		return s
	}
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" || strings.HasPrefix(line, "untrusted comment:") || strings.HasPrefix(line, "trusted comment:") {
			continue
		}
		return line
	}
	return ""
}

func isLocalhost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func applyEnv(c *Config) {
	set := func(key string, fn func(string)) {
		if v, ok := os.LookupEnv(key); ok {
			fn(v)
		}
	}
	set("NETDIAG_LISTEN", func(v string) { c.Server.Listen = v })
	set("NETDIAG_PUBLIC_URL", func(v string) { c.Server.PublicURL = v })
	set("NETDIAG_DATA_DIR", func(v string) { c.Storage.DataDir = v })
	set("NETDIAG_DB", func(v string) { c.Storage.DB = v })
	set("NETDIAG_LOG_LEVEL", func(v string) { c.Log.Level = v })
	set("NETDIAG_LOG_FORMAT", func(v string) { c.Log.Format = v })
	set("NETDIAG_PROFILE", func(v string) { c.Scan.Profile = v })
	set("NETDIAG_SCAN_TIMEOUT", func(v string) {
		if d, err := time.ParseDuration(v); err == nil {
			c.Scan.Timeout = Duration(d)
		}
	})
	set("NETDIAG_SCAN_CONCURRENCY", func(v string) {
		if n, err := strconv.Atoi(v); err == nil {
			c.Scan.Concurrency = n
		}
	})
	set("NETDIAG_AUTH", func(v string) {
		c.Security.Auth = v == "1" || strings.EqualFold(v, "true") || strings.EqualFold(v, "yes")
	})
	set("NETDIAG_SESSION_HOURS", func(v string) {
		if n, err := strconv.Atoi(v); err == nil {
			c.Security.SessionHours = n
		}
	})
	set("NETDIAG_MONITOR", func(v string) {
		c.Monitor.Enabled = v == "1" || strings.EqualFold(v, "true") || strings.EqualFold(v, "yes")
	})
	set("NETDIAG_MONITOR_INTERVAL", func(v string) {
		if d, err := time.ParseDuration(v); err == nil {
			c.Monitor.Interval = Duration(d)
		}
	})
	set("NETDIAG_VPN", func(v string) {
		c.VPN.Enabled = v == "1" || strings.EqualFold(v, "true") || strings.EqualFold(v, "yes")
	})
	set("NETDIAG_VPN_INTERVAL", func(v string) {
		if d, err := time.ParseDuration(v); err == nil {
			c.VPN.Interval = Duration(d)
		}
	})
	set("NETDIAG_WIFI_IFACE", func(v string) { c.WiFi.Interface = v })
	set("NETDIAG_WIFI_SCAN_SECONDS", func(v string) {
		if n, err := strconv.Atoi(v); err == nil {
			c.WiFi.ScanSeconds = n
		}
	})
	set("NETDIAG_SNMP", func(v string) {
		c.Scan.SNMP = v == "1" || strings.EqualFold(v, "true") || strings.EqualFold(v, "yes")
	})
	set("NETDIAG_SNMP_COMMUNITIES", func(v string) {
		var out []string
		for _, s := range strings.Split(v, ",") {
			if s = strings.TrimSpace(s); s != "" {
				out = append(out, s)
			}
		}
		if len(out) > 0 {
			c.Scan.SNMPCommunities = out
		}
	})
	set("NETDIAG_DHCP_PROBE", func(v string) {
		c.Diagnostics.DHCPProbe = v == "1" || strings.EqualFold(v, "true") || strings.EqualFold(v, "yes")
	})
	set("NETDIAG_DHCP_PROBE_TIMEOUT", func(v string) {
		if d, err := time.ParseDuration(v); err == nil {
			c.Diagnostics.DHCPProbeTimeout = Duration(d)
		}
	})
	set("NETDIAG_DHCP_AUTHORIZED", func(v string) {
		var out []string
		for _, s := range strings.Split(v, ",") {
			if s = strings.TrimSpace(s); s != "" {
				out = append(out, s)
			}
		}
		c.Diagnostics.AuthorizedDHCPServers = out
	})
	set("NETDIAG_ALLOW_PUBLIC", func(v string) {
		c.Scan.AllowPublic = v == "1" || strings.EqualFold(v, "true") || strings.EqualFold(v, "yes")
	})
	set("NETDIAG_TLS_CERT", func(v string) { c.Server.TLSCert = v })
	set("NETDIAG_TLS_KEY", func(v string) { c.Server.TLSKey = v })
	set("NETDIAG_UPDATE_ENABLED", func(v string) {
		c.Update.Enabled = v == "1" || strings.EqualFold(v, "true") || strings.EqualFold(v, "yes")
	})
	set("NETDIAG_UPDATE_MANIFEST_URL", func(v string) { c.Update.ManifestURL = v })
	set("NETDIAG_UPDATE_PUBLIC_KEY", func(v string) { c.Update.PublicKey = v })
	set("NETDIAG_UPDATE_INTERVAL_HOURS", func(v string) {
		if n, err := strconv.Atoi(v); err == nil {
			c.Update.CheckIntervalHours = n
		}
	})
	set("NETDIAG_UPDATE_RESTART_COMMAND", func(v string) { c.Update.RestartCommand = v })
}
