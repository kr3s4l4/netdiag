// Package model contains the domain types shared by every layer of NetDiag.
// Types in this package are transport-friendly: they carry JSON tags and are
// safe to serialise directly from the API.
package model

import "time"

// HostStatus is the reachability state of a host.
type HostStatus string

const (
	StatusUp       HostStatus = "up"
	StatusDown     HostStatus = "down"
	StatusUnknown  HostStatus = "unknown"
	StatusArchived HostStatus = "archived"
)

// DeviceType is a coarse classification produced by the fingerprint engine.
type DeviceType string

const (
	DevRouter      DeviceType = "router"
	DevSwitch      DeviceType = "switch"
	DevAccessPoint DeviceType = "access_point"
	DevFirewall    DeviceType = "firewall"
	DevServer      DeviceType = "server"
	DevWorkstation DeviceType = "workstation"
	DevLaptop      DeviceType = "laptop"
	DevPhone       DeviceType = "phone"
	DevTablet      DeviceType = "tablet"
	DevPrinter     DeviceType = "printer"
	DevCamera      DeviceType = "camera"
	DevIoT         DeviceType = "iot"
	DevNAS         DeviceType = "nas"
	DevVirtual     DeviceType = "virtual"
	DevMedia       DeviceType = "media"
	DevGameConsole DeviceType = "game_console"
	DevUnknown     DeviceType = "unknown"
)

// Scan status values.
const (
	ScanQueued      = "queued"
	ScanRunning     = "running"
	ScanDone        = "done"
	ScanCanceled    = "canceled"
	ScanError       = "error"
	ScanInterrupted = "interrupted"
)

// Severities and alert states.
const (
	SeverityInfo     = "info"
	SeverityWarning  = "warning"
	SeverityCritical = "critical"

	AlertActive   = "active"
	AlertAcked    = "acked"
	AlertResolved = "resolved"
)

// Port describes one observed TCP/UDP endpoint on a host.
type Port struct {
	Number   int      `json:"number"`
	Protocol string   `json:"protocol"` // tcp | udp
	State    string   `json:"state"`    // open | closed | filtered
	Service  string   `json:"service,omitempty"`
	Product  string   `json:"product,omitempty"`
	Version  string   `json:"version,omitempty"`
	Banner   string   `json:"banner,omitempty"`
	TLS      *TLSCert `json:"tls,omitempty"`
}

// TLSCert is a compact view of a certificate presented by a service.
type TLSCert struct {
	Subject    string    `json:"subject,omitempty"`
	Issuer     string    `json:"issuer,omitempty"`
	NotAfter   time.Time `json:"not_after,omitempty"`
	DNSNames   []string  `json:"dns_names,omitempty"`
	SelfSigned bool      `json:"self_signed,omitempty"`
}

// SNMPInterface is a row from IF-MIB.
type SNMPInterface struct {
	Index     int    `json:"index"`
	Name      string `json:"name"`
	Descr     string `json:"descr,omitempty"`
	MAC       string `json:"mac,omitempty"`
	Status    string `json:"status,omitempty"`
	SpeedMbps int64  `json:"speed_mbps,omitempty"`
	Alias     string `json:"alias,omitempty"`
}

// Neighbor is a discovered L2/L3 adjacency (LLDP, CDP or bridge FDB).
type Neighbor struct {
	Protocol      string `json:"protocol"` // lldp | cdp | bridge | arp
	RemoteName    string `json:"remote_name,omitempty"`
	RemoteIP      string `json:"remote_ip,omitempty"`
	RemoteMAC     string `json:"remote_mac,omitempty"`
	RemotePort    string `json:"remote_port,omitempty"`
	LocalPort     string `json:"local_port,omitempty"`
	RemoteChassis string `json:"remote_chassis,omitempty"`
	RemoteSysDesc string `json:"remote_sys_desc,omitempty"`
	Confidence    int    `json:"confidence"`
}

// SNMPSummary is the persisted SNMP snapshot of a device.
type SNMPSummary struct {
	SysDescr    string          `json:"sys_descr,omitempty"`
	SysObjectID string          `json:"sys_object_id,omitempty"`
	SysName     string          `json:"sys_name,omitempty"`
	SysLocation string          `json:"sys_location,omitempty"`
	SysContact  string          `json:"sys_contact,omitempty"`
	UptimeSecs  uint32          `json:"uptime_secs,omitempty"`
	Interfaces  []SNMPInterface `json:"interfaces,omitempty"`
	Neighbors   []Neighbor      `json:"neighbors,omitempty"`
	Version     string          `json:"version,omitempty"`
}

// Host is a discovered network device.
type Host struct {
	ID            int64        `json:"id"`
	Identity      string       `json:"-"`
	IP            string       `json:"ip"`
	MAC           string       `json:"mac,omitempty"`
	Hostname      string       `json:"hostname,omitempty"`
	Vendor        string       `json:"vendor,omitempty"`
	Model         string       `json:"model,omitempty"`
	OS            string       `json:"os,omitempty"`
	DeviceType    DeviceType   `json:"device_type"`
	Confidence    int          `json:"confidence"`
	Status        HostStatus   `json:"status"`
	IsGateway     bool         `json:"is_gateway"`
	Favorite      bool         `json:"favorite"`
	Notes         string       `json:"notes,omitempty"`
	Tags          []string     `json:"tags"`
	Ports         []Port       `json:"ports"`
	OpenPortCount int          `json:"open_port_count"`
	LatencyMS     float64      `json:"latency_ms"`
	TTL           int          `json:"ttl,omitempty"`
	FirstSeen     time.Time    `json:"first_seen"`
	LastSeen      time.Time    `json:"last_seen"`
	NetworkID     int64        `json:"network_id"`
	LastScanID    int64        `json:"last_scan_id,omitempty"`
	SNMP          *SNMPSummary `json:"snmp,omitempty"`
	Evidence      []string     `json:"evidence,omitempty"`
}

// Network is a monitored CIDR.
type Network struct {
	ID         int64      `json:"id"`
	Name       string     `json:"name"`
	CIDR       string     `json:"cidr"`
	Interface  string     `json:"interface,omitempty"`
	Enabled    bool       `json:"enabled"`
	IsLocal    bool       `json:"is_local"`
	CreatedAt  time.Time  `json:"created_at"`
	LastScanAt *time.Time `json:"last_scan_at,omitempty"`
	HostCount  int        `json:"host_count,omitempty"`
	UpCount    int        `json:"up_count,omitempty"`
}

// ScanOptions captures what a scan should do.
type ScanOptions struct {
	Profile     string `json:"profile"`         // quick | common | full | custom
	Ports       []int  `json:"ports,omitempty"` // explicit list for custom
	SNMP        bool   `json:"snmp"`
	Deep        bool   `json:"deep"`
	TimeoutMS   int    `json:"timeout_ms"`
	Concurrency int    `json:"concurrency"`
	MaxHosts    int    `json:"max_hosts"`
	ResolveDNS  bool   `json:"resolve_dns"`
	BannerGrab  bool   `json:"banner_grab"`
	AllowPublic bool   `json:"allow_public"`
}

// Scan is one discovery run.
type Scan struct {
	ID         int64       `json:"id"`
	NetworkID  int64       `json:"network_id"`
	CIDR       string      `json:"cidr"`
	Profile    string      `json:"profile"`
	Status     string      `json:"status"`
	Stage      string      `json:"stage"`
	Progress   float64     `json:"progress"`
	Total      int         `json:"total"`
	Scanned    int         `json:"scanned"`
	HostsFound int         `json:"hosts_found"`
	Trigger    string      `json:"trigger"`
	Options    ScanOptions `json:"options"`
	Error      string      `json:"error,omitempty"`
	StartedAt  *time.Time  `json:"started_at,omitempty"`
	FinishedAt *time.Time  `json:"finished_at,omitempty"`
}

// Event is a host lifecycle event.
type Event struct {
	ID        int64          `json:"id"`
	HostID    int64          `json:"host_id"`
	HostIP    string         `json:"host_ip,omitempty"`
	Kind      string         `json:"kind"`
	Message   string         `json:"message"`
	Data      map[string]any `json:"data,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

// Metric is one availability/latency sample.
type Metric struct {
	HostID int64     `json:"host_id"`
	TS     time.Time `json:"ts"`
	RTTMS  float64   `json:"rtt_ms"`
	Up     bool      `json:"up"`
	Loss   float64   `json:"loss"`
}

// SeriesPoint is a downsampled metric point for charts.
type SeriesPoint struct {
	TS    time.Time `json:"ts"`
	Value float64   `json:"value"`
}

// HostHistory is a chart-ready history payload.
type HostHistory struct {
	HostID       int64         `json:"host_id"`
	From         time.Time     `json:"from"`
	To           time.Time     `json:"to"`
	Availability float64       `json:"availability"`
	AvgRTT       float64       `json:"avg_rtt"`
	MinRTT       float64       `json:"min_rtt"`
	MaxRTT       float64       `json:"max_rtt"`
	Samples      int           `json:"samples"`
	RTT          []SeriesPoint `json:"rtt"`
	Uptime       []SeriesPoint `json:"uptime"`
}

// AlertRule defines when to raise an alert.
type AlertRule struct {
	ID          int64          `json:"id"`
	Name        string         `json:"name"`
	Type        string         `json:"type"`
	Enabled     bool           `json:"enabled"`
	Severity    string         `json:"severity"`
	Config      map[string]any `json:"config"`
	ChannelIDs  []int64        `json:"channel_ids"`
	CooldownSec int            `json:"cooldown_sec"`
	Builtin     bool           `json:"builtin"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// AlertChannel is a notification destination.
type AlertChannel struct {
	ID         int64          `json:"id"`
	Name       string         `json:"name"`
	Type       string         `json:"type"`
	Enabled    bool           `json:"enabled"`
	Config     map[string]any `json:"config"`
	HasSecret  bool           `json:"has_secret"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	LastTestAt *time.Time     `json:"last_test_at,omitempty"`
	LastError  string         `json:"last_error,omitempty"`
}

// Alert is a raised notification.
type Alert struct {
	ID         int64          `json:"id"`
	RuleID     int64          `json:"rule_id"`
	RuleName   string         `json:"rule_name"`
	HostID     int64          `json:"host_id,omitempty"`
	HostIP     string         `json:"host_ip,omitempty"`
	HostName   string         `json:"host_name,omitempty"`
	Severity   string         `json:"severity"`
	Status     string         `json:"status"`
	Title      string         `json:"title"`
	Message    string         `json:"message"`
	Data       map[string]any `json:"data,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
	AckedAt    *time.Time     `json:"acked_at,omitempty"`
	ResolvedAt *time.Time     `json:"resolved_at,omitempty"`
	DedupeKey  string         `json:"-"`
}

// AlertStats is used by the dashboard and alerts header.
type AlertStats struct {
	Active   int `json:"active"`
	Acked    int `json:"acked"`
	Resolved int `json:"resolved"`
	Critical int `json:"critical"`
	Warning  int `json:"warning"`
}

// AuditEntry is an immutable security/action log row.
type AuditEntry struct {
	ID        int64          `json:"id"`
	TS        time.Time      `json:"ts"`
	Actor     string         `json:"actor"`
	Action    string         `json:"action"`
	Target    string         `json:"target,omitempty"`
	Detail    map[string]any `json:"detail,omitempty"`
	IP        string         `json:"ip,omitempty"`
	UserAgent string         `json:"user_agent,omitempty"`
}

// User is a local account.
type User struct {
	ID          int64      `json:"id"`
	Username    string     `json:"username"`
	Role        string     `json:"role"` // admin | viewer
	CreatedAt   time.Time  `json:"created_at"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
}

// Session is a server-side session record.
type Session struct {
	ID         string
	UserID     int64
	CSRF       string
	CreatedAt  time.Time
	ExpiresAt  time.Time
	LastSeenAt time.Time
	IP         string
	UserAgent  string
}

// Report is a generated file.
type Report struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Format    string    `json:"format"`
	Kind      string    `json:"kind"`
	Size      int64     `json:"size"`
	Path      string    `json:"-"`
	CreatedAt time.Time `json:"created_at"`
}

// GraphNode is a topology vertex.
type GraphNode struct {
	ID         string     `json:"id"`
	Kind       string     `json:"kind"` // host | gateway | switch | router | internet | network
	HostID     int64      `json:"host_id,omitempty"`
	Label      string     `json:"label"`
	IP         string     `json:"ip,omitempty"`
	MAC        string     `json:"mac,omitempty"`
	Vendor     string     `json:"vendor,omitempty"`
	DeviceType DeviceType `json:"device_type,omitempty"`
	Status     HostStatus `json:"status,omitempty"`
	IsGateway  bool       `json:"is_gateway,omitempty"`
	NetworkID  int64      `json:"network_id,omitempty"`
	Network    string     `json:"network,omitempty"`
	Confidence int        `json:"confidence,omitempty"`
}

// GraphEdge is a topology edge.
type GraphEdge struct {
	ID         string `json:"id"`
	Source     string `json:"source"`
	Target     string `json:"target"`
	Kind       string `json:"kind"` // lldp | cdp | bridge | gateway | subnet | routed | arp
	Label      string `json:"label,omitempty"`
	Confidence int    `json:"confidence"`
}

// Graph is a complete topology payload.
type Graph struct {
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
	Stats GraphStats  `json:"stats"`
}

// GraphStats summarises a graph for the UI.
type GraphStats struct {
	Nodes    int            `json:"nodes"`
	Edges    int            `json:"edges"`
	Isolated int            `json:"isolated"`
	ByType   map[string]int `json:"by_type"`
}

// DashboardStats powers the main dashboard.
type DashboardStats struct {
	HostsTotal     int            `json:"hosts_total"`
	HostsUp        int            `json:"hosts_up"`
	HostsDown      int            `json:"hosts_down"`
	HostsNew24h    int            `json:"hosts_new_24h"`
	AlertsActive   int            `json:"alerts_active"`
	AlertsCritical int            `json:"alerts_critical"`
	Networks       int            `json:"networks"`
	Scans24h       int            `json:"scans_24h"`
	AvgRTT         float64        `json:"avg_rtt"`
	Availability   float64        `json:"availability"`
	ByType         map[string]int `json:"by_type"`
	ByStatus       map[string]int `json:"by_status"`
	LastScan       *Scan          `json:"last_scan,omitempty"`
}

// SystemInterface describes a local NIC.
type SystemInterface struct {
	Name      string          `json:"name"`
	Index     int             `json:"index"`
	MAC       string          `json:"mac,omitempty"`
	MTU       int             `json:"mtu"`
	Up        bool            `json:"up"`
	Loopback  bool            `json:"loopback"`
	Addrs     []InterfaceAddr `json:"addrs"`
	IsPrimary bool            `json:"is_primary"`
	Gateway   string          `json:"gateway,omitempty"`
}

// InterfaceAddr is an address bound to a NIC.
type InterfaceAddr struct {
	IP      string `json:"ip"`
	CIDR    string `json:"cidr"`
	Family  string `json:"family"` // ipv4 | ipv6
	Private bool   `json:"private"`
}

// SystemInfo is returned by /system/info.
type SystemInfo struct {
	Version      string            `json:"version"`
	Commit       string            `json:"commit"`
	BuildDate    string            `json:"build_date"`
	GoVersion    string            `json:"go_version"`
	Hostname     string            `json:"hostname"`
	OS           string            `json:"os"`
	Arch         string            `json:"arch"`
	UptimeSecs   int64             `json:"uptime_secs"`
	StartedAt    time.Time         `json:"started_at"`
	DataDir      string            `json:"data_dir"`
	DBSize       int64             `json:"db_size"`
	Capabilities map[string]bool   `json:"capabilities"`
	Interfaces   []SystemInterface `json:"interfaces"`
}

// Hello is returned by /auth/status before login.
type AuthStatus struct {
	SetupRequired bool   `json:"setup_required"`
	AuthEnabled   bool   `json:"auth_enabled"`
	Authenticated bool   `json:"authenticated"`
	User          *User  `json:"user,omitempty"`
	CSRF          string `json:"csrf,omitempty"`
	Version       string `json:"version,omitempty"`
}

// ListResult is the standard paginated envelope.
type ListResult[T any] struct {
	Items []T `json:"items"`
	Total int `json:"total"`
	Page  int `json:"page"`
	Limit int `json:"limit"`
}

// HostFilter is the query model for host listing.
type HostFilter struct {
	Search     string
	Status     string
	DeviceType string
	NetworkID  int64
	Tag        string
	Favorite   bool
	Sort       string
	Desc       bool
	Page       int
	Limit      int
}

// ScanFilter filters scan history.
type ScanFilter struct {
	Status string
	Limit  int
	Page   int
}

// AlertFilter filters alerts.
type AlertFilter struct {
	Status   string
	Severity string
	HostID   int64
	Search   string
	From     *time.Time
	To       *time.Time
	Page     int
	Limit    int
}

// AuditFilter filters audit entries.
type AuditFilter struct {
	From   *time.Time
	To     *time.Time
	Actor  string
	Action string
	Page   int
	Limit  int
}
