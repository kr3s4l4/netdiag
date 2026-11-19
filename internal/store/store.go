// Package store implements the SQLite persistence layer.
package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kr3s4l4/netdiag/internal/cryptoutil"
	"github.com/kr3s4l4/netdiag/internal/model"
	_ "modernc.org/sqlite"
)

// Store wraps the SQLite database.
type Store struct {
	db   *sql.DB
	path string
	box  *cryptoutil.Box
}

// Open opens (and migrates) the database at path.
func Open(path string, box *cryptoutil.Box) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	u := url.URL{Scheme: "file", Path: path}
	q := u.Query()
	q.Add("_pragma", "busy_timeout(5000)")
	q.Add("_pragma", "journal_mode(WAL)")
	q.Add("_pragma", "foreign_keys(1)")
	q.Add("_pragma", "synchronous(NORMAL)")
	q.Add("_pragma", "temp_store(MEMORY)")
	u.RawQuery = q.Encode()
	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(8)
	db.SetMaxIdleConns(4)
	db.SetConnMaxLifetime(time.Hour)
	s := &Store{db: db, path: path, box: box}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }

// Path returns the database file path.
func (s *Store) Path() string { return s.path }

// DB exposes the raw handle for maintenance tasks.
func (s *Store) DB() *sql.DB { return s.db }

// VacuumInto writes a consistent snapshot of the database to path.
func (s *Store) VacuumInto(path string) error {
	_, err := s.db.Exec(`VACUUM INTO ?`, path)
	return err
}

// Size returns the database file size in bytes.
func (s *Store) Size() int64 {
	fi, err := os.Stat(s.path)
	if err != nil {
		return 0
	}
	return fi.Size()
}

var migrations = [][]string{
	{
		`CREATE TABLE settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at INTEGER NOT NULL
		)`,
		`CREATE TABLE networks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL DEFAULT '',
			cidr TEXT NOT NULL UNIQUE,
			iface TEXT NOT NULL DEFAULT '',
			enabled INTEGER NOT NULL DEFAULT 1,
			is_local INTEGER NOT NULL DEFAULT 0,
			created_at INTEGER NOT NULL,
			last_scan_at INTEGER
		)`,
		`CREATE TABLE scans (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			network_id INTEGER NOT NULL DEFAULT 0,
			cidr TEXT NOT NULL,
			profile TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'queued',
			stage TEXT NOT NULL DEFAULT '',
			progress REAL NOT NULL DEFAULT 0,
			total INTEGER NOT NULL DEFAULT 0,
			scanned INTEGER NOT NULL DEFAULT 0,
			hosts_found INTEGER NOT NULL DEFAULT 0,
			trigger TEXT NOT NULL DEFAULT '',
			options TEXT NOT NULL DEFAULT '{}',
			error TEXT NOT NULL DEFAULT '',
			started_at INTEGER,
			finished_at INTEGER
		)`,
		`CREATE INDEX idx_scans_started ON scans(started_at DESC)`,
		`CREATE TABLE hosts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			identity TEXT NOT NULL UNIQUE,
			ip TEXT NOT NULL DEFAULT '',
			mac TEXT NOT NULL DEFAULT '',
			hostname TEXT NOT NULL DEFAULT '',
			vendor TEXT NOT NULL DEFAULT '',
			model TEXT NOT NULL DEFAULT '',
			os TEXT NOT NULL DEFAULT '',
			device_type TEXT NOT NULL DEFAULT 'unknown',
			confidence INTEGER NOT NULL DEFAULT 0,
			status TEXT NOT NULL DEFAULT 'unknown',
			is_gateway INTEGER NOT NULL DEFAULT 0,
			favorite INTEGER NOT NULL DEFAULT 0,
			notes TEXT NOT NULL DEFAULT '',
			tags TEXT NOT NULL DEFAULT '[]',
			ports TEXT NOT NULL DEFAULT '[]',
			latency_ms REAL NOT NULL DEFAULT 0,
			ttl INTEGER NOT NULL DEFAULT 0,
			first_seen INTEGER NOT NULL,
			last_seen INTEGER NOT NULL,
			network_id INTEGER NOT NULL DEFAULT 0,
			last_scan_id INTEGER NOT NULL DEFAULT 0,
			snmp TEXT NOT NULL DEFAULT '',
			evidence TEXT NOT NULL DEFAULT '[]'
		)`,
		`CREATE INDEX idx_hosts_ip ON hosts(ip)`,
		`CREATE INDEX idx_hosts_mac ON hosts(mac)`,
		`CREATE INDEX idx_hosts_last_seen ON hosts(last_seen DESC)`,
		`CREATE INDEX idx_hosts_status ON hosts(status)`,
		`CREATE INDEX idx_hosts_type ON hosts(device_type)`,
		`CREATE TABLE host_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			host_id INTEGER NOT NULL,
			kind TEXT NOT NULL,
			message TEXT NOT NULL,
			data TEXT NOT NULL DEFAULT '{}',
			created_at INTEGER NOT NULL,
			FOREIGN KEY(host_id) REFERENCES hosts(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX idx_events_host ON host_events(host_id, created_at DESC)`,
		`CREATE INDEX idx_events_created ON host_events(created_at DESC)`,
		`CREATE TABLE metrics (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			host_id INTEGER NOT NULL,
			ts INTEGER NOT NULL,
			rtt_ms REAL NOT NULL DEFAULT 0,
			up INTEGER NOT NULL DEFAULT 1,
			loss REAL NOT NULL DEFAULT 0,
			FOREIGN KEY(host_id) REFERENCES hosts(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX idx_metrics_host_ts ON metrics(host_id, ts)`,
		`CREATE INDEX idx_metrics_ts ON metrics(ts)`,
		`CREATE TABLE alert_rules (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			type TEXT NOT NULL,
			enabled INTEGER NOT NULL DEFAULT 1,
			severity TEXT NOT NULL DEFAULT 'warning',
			config TEXT NOT NULL DEFAULT '{}',
			channel_ids TEXT NOT NULL DEFAULT '[]',
			cooldown_sec INTEGER NOT NULL DEFAULT 900,
			builtin INTEGER NOT NULL DEFAULT 0,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		)`,
		`CREATE TABLE channels (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			type TEXT NOT NULL,
			enabled INTEGER NOT NULL DEFAULT 1,
			config_enc BLOB NOT NULL,
			has_secret INTEGER NOT NULL DEFAULT 0,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL,
			last_test_at INTEGER,
			last_error TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE alerts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			rule_id INTEGER NOT NULL DEFAULT 0,
			rule_name TEXT NOT NULL DEFAULT '',
			host_id INTEGER NOT NULL DEFAULT 0,
			host_ip TEXT NOT NULL DEFAULT '',
			host_name TEXT NOT NULL DEFAULT '',
			severity TEXT NOT NULL DEFAULT 'warning',
			status TEXT NOT NULL DEFAULT 'active',
			title TEXT NOT NULL,
			message TEXT NOT NULL,
			data TEXT NOT NULL DEFAULT '{}',
			created_at INTEGER NOT NULL,
			acked_at INTEGER,
			resolved_at INTEGER,
			dedupe_key TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX idx_alerts_status ON alerts(status, created_at DESC)`,
		`CREATE INDEX idx_alerts_dedupe ON alerts(dedupe_key, status)`,
		`CREATE TABLE audit (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			ts INTEGER NOT NULL,
			actor TEXT NOT NULL DEFAULT '',
			action TEXT NOT NULL,
			target TEXT NOT NULL DEFAULT '',
			detail TEXT NOT NULL DEFAULT '{}',
			ip TEXT NOT NULL DEFAULT '',
			user_agent TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX idx_audit_ts ON audit(ts DESC)`,
		`CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'admin',
			created_at INTEGER NOT NULL,
			last_login_at INTEGER
		)`,
		`CREATE TABLE sessions (
			id TEXT PRIMARY KEY,
			user_id INTEGER NOT NULL,
			csrf TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			expires_at INTEGER NOT NULL,
			last_seen_at INTEGER NOT NULL,
			ip TEXT NOT NULL DEFAULT '',
			user_agent TEXT NOT NULL DEFAULT '',
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX idx_sessions_expires ON sessions(expires_at)`,
		`CREATE TABLE neighbors (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			local_host_id INTEGER NOT NULL,
			protocol TEXT NOT NULL,
			remote_name TEXT NOT NULL DEFAULT '',
			remote_ip TEXT NOT NULL DEFAULT '',
			remote_mac TEXT NOT NULL DEFAULT '',
			remote_port TEXT NOT NULL DEFAULT '',
			local_port TEXT NOT NULL DEFAULT '',
			remote_chassis TEXT NOT NULL DEFAULT '',
			remote_sys_desc TEXT NOT NULL DEFAULT '',
			confidence INTEGER NOT NULL DEFAULT 0,
			updated_at INTEGER NOT NULL,
			FOREIGN KEY(local_host_id) REFERENCES hosts(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX idx_neighbors_local ON neighbors(local_host_id)`,
		`CREATE TABLE reports (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			format TEXT NOT NULL,
			kind TEXT NOT NULL,
			size INTEGER NOT NULL DEFAULT 0,
			path TEXT NOT NULL,
			created_at INTEGER NOT NULL
		)`,
		`CREATE INDEX idx_reports_created ON reports(created_at DESC)`,
	},
	{
		// v1.2: multi-site. Networks keep uniqueness per site, so two offices
		// may monitor the same private range, which is the common case.
		`CREATE TABLE sites (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			slug TEXT NOT NULL UNIQUE,
			description TEXT NOT NULL DEFAULT '',
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		)`,
		`INSERT INTO sites(id, name, slug, description, created_at, updated_at)
			VALUES(1, 'Principal', 'principal', '', CAST(strftime('%s','now') AS INTEGER) * 1000,
			CAST(strftime('%s','now') AS INTEGER) * 1000)`,
		`CREATE TABLE networks_v2 (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			site_id INTEGER NOT NULL DEFAULT 1,
			name TEXT NOT NULL DEFAULT '',
			cidr TEXT NOT NULL,
			iface TEXT NOT NULL DEFAULT '',
			enabled INTEGER NOT NULL DEFAULT 1,
			is_local INTEGER NOT NULL DEFAULT 0,
			created_at INTEGER NOT NULL,
			last_scan_at INTEGER,
			UNIQUE(site_id, cidr)
		)`,
		`INSERT INTO networks_v2(id, site_id, name, cidr, iface, enabled, is_local, created_at, last_scan_at)
			SELECT id, 1, name, cidr, iface, enabled, is_local, created_at, last_scan_at FROM networks`,
		`DROP TABLE networks`,
		`ALTER TABLE networks_v2 RENAME TO networks`,
		`ALTER TABLE hosts ADD COLUMN site_id INTEGER NOT NULL DEFAULT 1`,
		`ALTER TABLE scans ADD COLUMN site_id INTEGER NOT NULL DEFAULT 1`,
		`CREATE INDEX idx_networks_site ON networks(site_id)`,
		`CREATE INDEX idx_hosts_site ON hosts(site_id)`,
		`CREATE INDEX idx_scans_site ON scans(site_id)`,
		`UPDATE hosts SET site_id = COALESCE((SELECT n.site_id FROM networks n WHERE n.id = hosts.network_id), 1)`,
		`UPDATE scans SET site_id = COALESCE((SELECT n.site_id FROM networks n WHERE n.id = scans.network_id), 1)`,
	},
	{
		// v1.2.1: reports can be attributed to a site (0 = global report).
		`ALTER TABLE reports ADD COLUMN site_id INTEGER NOT NULL DEFAULT 0`,
		`CREATE INDEX idx_reports_site ON reports(site_id)`,
	},
	{
		// v1.2.2: namespace host identities by site so the same IP or MAC can
		// exist in several sites without colliding.
		`UPDATE hosts SET identity = printf('s%d:%s', site_id, identity)`,
	},
	{
		// v1.4: outbound integrations with encrypted credentials.
		`CREATE TABLE integrations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			type TEXT NOT NULL,
			enabled INTEGER NOT NULL DEFAULT 1,
			config_enc BLOB NOT NULL,
			has_secret INTEGER NOT NULL DEFAULT 0,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL,
			last_test_at INTEGER,
			last_error TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX idx_integrations_type ON integrations(type)`,
	},
	{
		// v1.5: F12 VPN tunnel observations.
		`CREATE TABLE vpn_tunnels (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			site_id INTEGER NOT NULL DEFAULT 1,
			identity TEXT NOT NULL UNIQUE,
			type TEXT NOT NULL,
			local_ip TEXT NOT NULL DEFAULT '',
			remote_ip TEXT NOT NULL DEFAULT '',
			local_port INTEGER NOT NULL DEFAULT 0,
			remote_port INTEGER NOT NULL DEFAULT 0,
			bytes_sent INTEGER NOT NULL DEFAULT 0,
			bytes_recv INTEGER NOT NULL DEFAULT 0,
			state TEXT NOT NULL DEFAULT 'observed',
			evidence TEXT NOT NULL DEFAULT '[]',
			confidence INTEGER NOT NULL DEFAULT 0,
			first_seen INTEGER NOT NULL,
			last_seen INTEGER NOT NULL
		)`,
		`CREATE INDEX idx_vpn_tunnels_site ON vpn_tunnels(site_id, type)`,
	},
	{
		// v1.5.1: F11 passive WiFi observations.
		`CREATE TABLE wifi_aps (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			site_id INTEGER NOT NULL DEFAULT 1,
			bssid TEXT NOT NULL,
			ssid TEXT NOT NULL DEFAULT '',
			channel INTEGER NOT NULL DEFAULT 0,
			band TEXT NOT NULL DEFAULT '',
			encryption TEXT NOT NULL DEFAULT '',
			signal_dbm INTEGER NOT NULL DEFAULT 0,
			vendor TEXT NOT NULL DEFAULT '',
			beacons INTEGER NOT NULL DEFAULT 0,
			clients INTEGER NOT NULL DEFAULT 0,
			rogue INTEGER NOT NULL DEFAULT 0,
			rogue_info TEXT NOT NULL DEFAULT '',
			rogue_confidence INTEGER NOT NULL DEFAULT 0,
			first_seen INTEGER NOT NULL,
			last_seen INTEGER NOT NULL,
			UNIQUE(site_id, bssid)
		)`,
		`CREATE TABLE wifi_clients (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			site_id INTEGER NOT NULL DEFAULT 1,
			mac TEXT NOT NULL,
			ap_bssid TEXT NOT NULL DEFAULT '',
			signal_dbm INTEGER NOT NULL DEFAULT 0,
			last_seen INTEGER NOT NULL,
			UNIQUE(site_id, mac)
		)`,
		`CREATE INDEX idx_wifi_aps_channel ON wifi_aps(site_id, channel)`,
		`CREATE INDEX idx_wifi_clients_ap ON wifi_clients(site_id, ap_bssid)`,
	},
	{
		// v1.5.2: F18 custom dashboards (fixed 12-column grid, max 16 tiles).
		`CREATE TABLE dashboards (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			site_id INTEGER NOT NULL DEFAULT 1,
			name TEXT NOT NULL,
			widgets TEXT NOT NULL DEFAULT '[]',
			created_at INTEGER NOT NULL,
			modified INTEGER NOT NULL,
			UNIQUE(site_id, name)
		)`,
		`CREATE INDEX idx_dashboards_site ON dashboards(site_id)`,
	},
	{
		// v1.5.3: F21 temporal comparison. Snapshots are compact inventory
		// and topology captures used to diff moments beyond metric samples.
		`CREATE TABLE history_snapshots (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			site_id INTEGER NOT NULL DEFAULT 1,
			ts INTEGER NOT NULL,
			hosts TEXT NOT NULL DEFAULT '[]',
			graph TEXT NOT NULL DEFAULT '{}'
		)`,
		`CREATE INDEX idx_history_snapshots_ts ON history_snapshots(site_id, ts DESC)`,
	},
	{
		// v1.5.4: F22 audit mode. Full, gzip-compressed baselines per site.
		`CREATE TABLE audit_snapshots (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			site_id INTEGER NOT NULL DEFAULT 1,
			name TEXT NOT NULL,
			notes TEXT NOT NULL DEFAULT '',
			ts INTEGER NOT NULL,
			raw_size INTEGER NOT NULL DEFAULT 0,
			size INTEGER NOT NULL DEFAULT 0,
			data BLOB NOT NULL
		)`,
		`CREATE INDEX idx_audit_snapshots_site_ts ON audit_snapshots(site_id, ts DESC)`,
	},
	{
		// v1.5.5: F40 per-user saved inventory/alert views. Values are
		// sanitized by the store before they reach this table (name and query
		// are HTML-escaped), so a view can be shared through a URL safely.
		`CREATE TABLE saved_views (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL DEFAULT 0,
			site_id INTEGER NOT NULL DEFAULT 0,
			name TEXT NOT NULL,
			kind TEXT NOT NULL DEFAULT 'hosts',
			query TEXT NOT NULL DEFAULT '',
			severity TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT '',
			window TEXT NOT NULL DEFAULT '',
			columns_json TEXT NOT NULL DEFAULT '[]',
			sort TEXT NOT NULL DEFAULT '',
			desc INTEGER NOT NULL DEFAULT 0,
			builtin INTEGER NOT NULL DEFAULT 0,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL,
			UNIQUE(user_id, name)
		)`,
		`CREATE INDEX idx_saved_views_user ON saved_views(user_id, kind, site_id)`,
	},
	{
		// v1.5.6: F43 IP→MAC history from scans. It powers the ARP anomaly
		// detectors without privileged packet capture. Bindings are raw
		// evidence: the detector, not this table, decides what is suspicious.
		`CREATE TABLE arp_observations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			site_id INTEGER NOT NULL DEFAULT 1,
			ip TEXT NOT NULL,
			mac TEXT NOT NULL,
			hostname TEXT NOT NULL DEFAULT '',
			is_gateway INTEGER NOT NULL DEFAULT 0,
			scan_id INTEGER NOT NULL DEFAULT 0,
			observed_at INTEGER NOT NULL
		)`,
		`CREATE INDEX idx_arp_observations_ip ON arp_observations(site_id, ip, observed_at DESC)`,
		`CREATE INDEX idx_arp_observations_mac ON arp_observations(site_id, mac, observed_at DESC)`,
	},
	{
		// v1.5.7: F44 DHCP replies collected with an unprivileged UDP probe.
		// The table is raw evidence; the detector decides what is rogue based
		// on the gateway, the authorized list and the observation history.
		`CREATE TABLE dhcp_observations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			site_id INTEGER NOT NULL DEFAULT 1,
			ip TEXT NOT NULL,
			server_id TEXT NOT NULL DEFAULT '',
			mac TEXT NOT NULL DEFAULT '',
			message_type TEXT NOT NULL DEFAULT '',
			gateway TEXT NOT NULL DEFAULT '',
			interface TEXT NOT NULL DEFAULT '',
			lease_seconds INTEGER NOT NULL DEFAULT 0,
			subnet TEXT NOT NULL DEFAULT '',
			routers TEXT NOT NULL DEFAULT '',
			dns TEXT NOT NULL DEFAULT '',
			scan_id INTEGER NOT NULL DEFAULT 0,
			observed_at INTEGER NOT NULL
		)`,
		`CREATE INDEX idx_dhcp_observations_server ON dhcp_observations(site_id, server_id, observed_at DESC)`,
		`CREATE INDEX idx_dhcp_observations_ip ON dhcp_observations(site_id, ip, observed_at DESC)`,
		`CREATE INDEX idx_dhcp_observations_time ON dhcp_observations(site_id, observed_at DESC)`,
	},
	{
		// v1.5.8: security findings can point at devices that never answered a
		// ping (a rogue DHCP server, for example). Persist the site directly on
		// the alert so it is not orphaned when there is no inventory host.
		`ALTER TABLE alerts ADD COLUMN site_id INTEGER NOT NULL DEFAULT 0`,
		`CREATE INDEX idx_alerts_site ON alerts(site_id, created_at DESC)`,
	},
	{
		// v1.5.9: F46 raw SNMP evidence. Separate tables keep each family
		// queryable and let retention prune them without touching inventory.
		`CREATE TABLE snmp_ip_addresses (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			site_id INTEGER NOT NULL DEFAULT 1,
			host_ip TEXT NOT NULL DEFAULT '',
			community TEXT NOT NULL DEFAULT '',
			ip TEXT NOT NULL,
			if_index INTEGER NOT NULL DEFAULT 0,
			mask TEXT NOT NULL DEFAULT '',
			scan_id INTEGER NOT NULL DEFAULT 0,
			observed_at INTEGER NOT NULL
		)`,
		`CREATE INDEX idx_snmp_ip_time ON snmp_ip_addresses(site_id, observed_at DESC)`,
		`CREATE TABLE snmp_arp_rows (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			site_id INTEGER NOT NULL DEFAULT 1,
			host_ip TEXT NOT NULL DEFAULT '',
			community TEXT NOT NULL DEFAULT '',
			ip TEXT NOT NULL,
			mac TEXT NOT NULL DEFAULT '',
			if_index INTEGER NOT NULL DEFAULT 0,
			row_type TEXT NOT NULL DEFAULT '',
			table_name TEXT NOT NULL DEFAULT '',
			scan_id INTEGER NOT NULL DEFAULT 0,
			observed_at INTEGER NOT NULL
		)`,
		`CREATE INDEX idx_snmp_arp_ip ON snmp_arp_rows(site_id, ip, observed_at DESC)`,
		`CREATE INDEX idx_snmp_arp_time ON snmp_arp_rows(site_id, observed_at DESC)`,
		`CREATE TABLE snmp_vlans (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			site_id INTEGER NOT NULL DEFAULT 1,
			host_ip TEXT NOT NULL DEFAULT '',
			community TEXT NOT NULL DEFAULT '',
			vlan_id INTEGER NOT NULL,
			name TEXT NOT NULL DEFAULT '',
			scan_id INTEGER NOT NULL DEFAULT 0,
			observed_at INTEGER NOT NULL
		)`,
		`CREATE INDEX idx_snmp_vlan_time ON snmp_vlans(site_id, observed_at DESC)`,
		`CREATE TABLE snmp_fdb_rows (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			site_id INTEGER NOT NULL DEFAULT 1,
			host_ip TEXT NOT NULL DEFAULT '',
			community TEXT NOT NULL DEFAULT '',
			mac TEXT NOT NULL DEFAULT '',
			port INTEGER NOT NULL DEFAULT 0,
			if_index INTEGER NOT NULL DEFAULT 0,
			vlan INTEGER NOT NULL DEFAULT 0,
			status TEXT NOT NULL DEFAULT '',
			table_name TEXT NOT NULL DEFAULT '',
			scan_id INTEGER NOT NULL DEFAULT 0,
			observed_at INTEGER NOT NULL
		)`,
		`CREATE INDEX idx_snmp_fdb_mac ON snmp_fdb_rows(site_id, mac, observed_at DESC)`,
		`CREATE INDEX idx_snmp_fdb_time ON snmp_fdb_rows(site_id, observed_at DESC)`,
	},
	{
		// v1.5.10: F47 passive capture evidence. Only protocol metadata is
		// stored; packet payloads never reach the database.
		`CREATE TABLE passive_observations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			site_id INTEGER NOT NULL DEFAULT 1,
			scan_id INTEGER NOT NULL DEFAULT 0,
			kind TEXT NOT NULL,
			interface TEXT NOT NULL DEFAULT '',
			src_ip TEXT NOT NULL DEFAULT '',
			dst_ip TEXT NOT NULL DEFAULT '',
			src_mac TEXT NOT NULL DEFAULT '',
			protocol TEXT NOT NULL DEFAULT '',
			detail TEXT NOT NULL DEFAULT '',
			packets INTEGER NOT NULL DEFAULT 1,
			observed_at INTEGER NOT NULL
		)`,
		`CREATE INDEX idx_passive_kind ON passive_observations(site_id, kind, observed_at DESC)`,
		`CREATE INDEX idx_passive_time ON passive_observations(site_id, observed_at DESC)`,
	},
	{
		// v1.5.11: F59 local human triage of diagnostic findings. Reviews
		// expire after 30 days and never close alerts automatically.
		`CREATE TABLE finding_reviews (
			site_id INTEGER NOT NULL,
			finding_type TEXT NOT NULL,
			host_key TEXT NOT NULL,
			status TEXT NOT NULL,
			note TEXT NOT NULL DEFAULT '',
			reviewer TEXT NOT NULL DEFAULT '',
			expires_at INTEGER NOT NULL,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL,
			PRIMARY KEY (site_id, finding_type, host_key)
		)`,
		`CREATE INDEX idx_finding_reviews_expires ON finding_reviews(expires_at)`,
	},
}

func (s *Store) migrate() error {
	var version int
	if err := s.db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		return err
	}
	for i := version; i < len(migrations); i++ {
		tx, err := s.db.Begin()
		if err != nil {
			return err
		}
		for _, stmt := range migrations[i] {
			if _, err := tx.Exec(stmt); err != nil {
				tx.Rollback()
				return fmt.Errorf("migración %d: %w", i+1, err)
			}
		}
		if _, err := tx.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, i+1)); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

// --- helpers ---

func nowMS() int64 { return time.Now().UnixMilli() }

func fromMS(v int64) time.Time { return time.UnixMilli(v) }

func fromMSNull(v sql.NullInt64) *time.Time {
	if !v.Valid {
		return nil
	}
	t := time.UnixMilli(v.Int64)
	return &t
}

func timePtrMS(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UnixMilli()
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func jsonMarshal(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "null"
	}
	return string(b)
}

func jsonObject(raw string) map[string]any {
	if raw == "" {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return nil
	}
	return m
}

func jsonInt64s(raw string) []int64 {
	var out []int64
	if raw == "" {
		return nil
	}
	_ = json.Unmarshal([]byte(raw), &out)
	return out
}

// ErrNotFound is returned by lookups that yield no row.
var ErrNotFound = errors.New("no encontrado")

// --- settings ---

// GetSettings returns all key/value settings.
func (s *Store) GetSettings() (map[string]string, error) {
	rows, err := s.db.Query(`SELECT key, value FROM settings`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, rows.Err()
}

// GetSetting returns one setting or def.
func (s *Store) GetSetting(key, def string) string {
	var v string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&v)
	if err != nil {
		return def
	}
	return v
}

// SetSetting upserts one setting.
func (s *Store) SetSetting(key, value string) error {
	_, err := s.db.Exec(`INSERT INTO settings(key, value, updated_at) VALUES(?,?,?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at`,
		key, value, nowMS())
	return err
}

// SetSettings upserts several settings in one transaction.
func (s *Store) SetSettings(kv map[string]string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := nowMS()
	for k, v := range kv {
		if _, err := tx.Exec(`INSERT INTO settings(key, value, updated_at) VALUES(?,?,?)
			ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at`, k, v, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// SetBox replaces the encryption box (used by tests).
func (s *Store) SetBox(box *cryptoutil.Box) { s.box = box }

// escapeLike escapes LIKE wildcards.
func escapeLike(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}

// ChannelSecretFields lists config keys that must never be returned by the API.
func ChannelSecretFields(typ string) []string {
	switch typ {
	case "webhook":
		return []string{"secret"}
	case "slack", "discord":
		return []string{"url"}
	case "telegram":
		return []string{"token"}
	case "pagerduty":
		return []string{"routing_key"}
	case "smtp":
		return []string{"password"}
	case "syslog":
		return nil
	}
	return nil
}

// ValidateChannelType reports whether typ is supported.
func ValidateChannelType(typ string) bool {
	switch typ {
	case "webhook", "slack", "discord", "telegram", "pagerduty", "smtp", "syslog":
		return true
	}
	return false
}

// IntegrationTypes is the canonical list of supported outbound exporters.
var IntegrationTypes = []string{
	"elasticsearch", "syslog", "webhook", "slack", "discord", "msteams",
	"telegram", "pagerduty", "kafka", "redis",
}

// IntegrationSecretFields lists config keys that must be encrypted and masked.
func IntegrationSecretFields(typ string) []string {
	switch typ {
	case "elasticsearch":
		return []string{"api_key", "username", "password"}
	case "webhook", "slack", "discord", "msteams":
		return []string{"url", "secret"}
	case "telegram":
		return []string{"token"}
	case "pagerduty":
		return []string{"routing_key"}
	case "kafka":
		return []string{"username", "password", "token"}
	case "redis":
		return []string{"password", "username"}
	case "syslog":
		return nil
	}
	return nil
}

// ValidateIntegrationType reports whether typ is supported.
func ValidateIntegrationType(typ string) bool {
	for _, supported := range IntegrationTypes {
		if typ == supported {
			return true
		}
	}
	return false
}

// ValidateRuleType reports whether typ is supported.
func ValidateRuleType(typ string) bool {
	switch typ {
	case "host_down", "host_new", "latency_high", "port_opened", "port_closed",
		"service_changed", "device_changed", "subnet_usage", "scan_failed",
		"anomaly", "arp_spoofing", "dhcp_rogue", "snmp_anomaly", "passive_anomaly", "tls_expiring":
		return true
	}
	return false
}

// EnsureBuiltinRules inserts the default alert rules once.
func (s *Store) EnsureBuiltinRules() error {
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM alert_rules`).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	now := nowMS()
	rules := []struct {
		name, typ, sev, cfg string
		cooldown            int
	}{
		{"Host caído", "host_down", model.SeverityCritical, `{"down_after_sec":120}`, 1800},
		{"Host nuevo", "host_new", model.SeverityInfo, `{}`, 0},
		{"Latencia alta", "latency_high", model.SeverityWarning, `{"threshold_ms":150,"consecutive":3}`, 900},
		{"Puerto abierto", "port_opened", model.SeverityWarning, `{"ignore_ephemeral":true}`, 300},
		{"Servicio cambiado", "service_changed", model.SeverityWarning, `{}`, 600},
		{"Uso de subred", "subnet_usage", model.SeverityWarning, `{"percent":90}`, 3600},
		{"Anomalía de red", "anomaly", model.SeverityCritical, `{"min_confidence":75}`, 3600},
	}
	for _, r := range rules {
		if _, err := s.db.Exec(`INSERT INTO alert_rules(name,type,enabled,severity,config,channel_ids,cooldown_sec,builtin,created_at,updated_at)
			VALUES(?,?,1,?,?,'[]',?,1,?,?)`, r.name, r.typ, r.sev, r.cfg, r.cooldown, now, now); err != nil {
			return err
		}
	}
	return nil
}
