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

// SNMPIPAddress is one row of IP-MIB ipAddrTable (1.3.6.1.2.1.4.20).
type SNMPIPAddress struct {
	ID         int64     `json:"id,omitempty"`
	SiteID     int64     `json:"site_id,omitempty"`
	HostIP     string    `json:"host_ip,omitempty"` // equipo sondeado
	Community  string    `json:"community,omitempty"`
	ScanID     int64     `json:"scan_id,omitempty"`
	ObservedAt time.Time `json:"observed_at,omitempty"`
	IP         string    `json:"ip"`
	IfIndex    int       `json:"if_index"`
	Mask       string    `json:"mask,omitempty"`
}

// SNMPARPRow is one row of the switch ARP table (IP-MIB ipNetToMediaTable or
// ipNetToPhysicalTable). It is evidence, not a live packet capture.
type SNMPARPRow struct {
	ID         int64     `json:"id,omitempty"`
	SiteID     int64     `json:"site_id,omitempty"`
	HostIP     string    `json:"host_ip,omitempty"`
	Community  string    `json:"community,omitempty"`
	ScanID     int64     `json:"scan_id,omitempty"`
	ObservedAt time.Time `json:"observed_at,omitempty"`
	IP         string    `json:"ip"`
	MAC        string    `json:"mac"`
	IfIndex    int       `json:"if_index,omitempty"`
	Type       string    `json:"type,omitempty"`  // dynamic | static | other
	Table      string    `json:"table,omitempty"` // net_to_media | net_to_physical
}

// SNMPVLAN is one VLAN learned from Q-BRIDGE-MIB dot1qVlanStaticTable.
type SNMPVLAN struct {
	ID         int64     `json:"id,omitempty"`
	SiteID     int64     `json:"site_id,omitempty"`
	HostIP     string    `json:"host_ip,omitempty"`
	Community  string    `json:"community,omitempty"`
	ScanID     int64     `json:"scan_id,omitempty"`
	ObservedAt time.Time `json:"observed_at,omitempty"`
	VLAN       int       `json:"vlan"`
	Name       string    `json:"name,omitempty"`
}

// SNMPFDBRow is one forwarding database entry from Q-BRIDGE-MIB (vlan aware)
// or BRIDGE-MIB (flat). Port is the bridge port; IfIndex is its IF-MIB mapping.
type SNMPFDBRow struct {
	ID         int64     `json:"id,omitempty"`
	SiteID     int64     `json:"site_id,omitempty"`
	HostIP     string    `json:"host_ip,omitempty"`
	Community  string    `json:"community,omitempty"`
	ScanID     int64     `json:"scan_id,omitempty"`
	ObservedAt time.Time `json:"observed_at,omitempty"`
	MAC        string    `json:"mac"`
	Port       int       `json:"port"`
	IfIndex    int       `json:"if_index,omitempty"`
	VLAN       int       `json:"vlan,omitempty"`
	Status     string    `json:"status,omitempty"` // learned | static | other
	Table      string    `json:"table,omitempty"`  // q_bridge | bridge
}

// SNMPDeep groups the F46 evidence collected beyond the basic SNMP snapshot.
// It stays out of the host JSON so inventory payloads do not grow with FDB and
// ARP tables; the dedicated diagnostics endpoint returns the persisted rows.
type SNMPDeep struct {
	IPAddresses []SNMPIPAddress `json:"-"`
	ARP         []SNMPARPRow    `json:"-"`
	VLANs       []SNMPVLAN      `json:"-"`
	FDB         []SNMPFDBRow    `json:"-"`
}

// PassiveObservation is one metadata-only frame seen by the optional passive
// capture. Payloads are never decoded beyond the protocol header or stored.
type PassiveObservation struct {
	ID         int64     `json:"id,omitempty"`
	SiteID     int64     `json:"site_id,omitempty"`
	ScanID     int64     `json:"scan_id,omitempty"`
	Kind       string    `json:"kind"` // garp | icmp_redirect | dhcp_server | inbound_scan | dns_tunnel
	Interface  string    `json:"interface,omitempty"`
	SrcIP      string    `json:"src_ip,omitempty"`
	DstIP      string    `json:"dst_ip,omitempty"`
	SrcMAC     string    `json:"src_mac,omitempty"`
	Protocol   string    `json:"protocol,omitempty"` // arp | icmp | udp | tcp
	Detail     string    `json:"detail,omitempty"`   // puerto, dominio o Mensaje DHCP
	Count      int       `json:"count,omitempty"`
	ObservedAt time.Time `json:"observed_at,omitempty"`
}

// SNMPDeepSnapshot is the retained deep evidence for one site and window.
type SNMPDeepSnapshot struct {
	IPAddresses []SNMPIPAddress `json:"ip_addresses"`
	ARP         []SNMPARPRow    `json:"arp"`
	VLANs       []SNMPVLAN      `json:"vlans"`
	FDB         []SNMPFDBRow    `json:"fdb"`
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
	Community   string          `json:"community,omitempty"` // enmascarada, nunca en claro
	Deep        SNMPDeep        `json:"-"`
}

// Site is a logical location (office, branch, datacenter) that groups
// networks and hosts. A single instance can monitor several sites without
// running several servers.
type Site struct {
	ID           int64      `json:"id"`
	Name         string     `json:"name"`
	Slug         string     `json:"slug"`
	Description  string     `json:"description,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	NetworkCount int        `json:"network_count"`
	HostCount    int        `json:"host_count"`
	UpCount      int        `json:"up_count"`
	LastScanAt   *time.Time `json:"last_scan_at,omitempty"`
}

// Host is a discovered network device.
type Host struct {
	ID            int64        `json:"id"`
	Identity      string       `json:"-"`
	SiteID        int64        `json:"site_id"`
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
	SiteID     int64      `json:"site_id"`
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
	SiteID     int64       `json:"site_id"`
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

// The enterprise diagnostics types below describe heuristics built from the
// data NetDiag already collects (LLDP/CDP/FDB adjacency, IF-MIB, inventory and
// availability samples). NetDiag does not sniff raw 802.1D/802.1Q frames, so
// "protocol" and "flags" are normalized from observed evidence rather than
// decoded from BPDUs. Every finding carries its evidence and confidence.

// BPDU is a normalized spanning-tree announcement reconstructed from L2
// adjacency and device inventory, not from a captured frame.
type BPDU struct {
	Protocol string   `json:"protocol,omitempty"` // stp | rstp | mstp | unknown
	RootID   string   `json:"root_id,omitempty"`
	BridgeID string   `json:"bridge_id,omitempty"`
	Port     string   `json:"port,omitempty"`
	Cost     int      `json:"cost,omitempty"`
	Flags    []string `json:"flags,omitempty"`
}

// STPLoop is a cycle observed in the high-confidence L2 adjacency graph.
type STPLoop struct {
	BridgeIDs     []string `json:"bridge_ids"`
	AffectedPorts []string `json:"affected_ports"`
	Rate          float64  `json:"rate"`
	Confidence    int      `json:"confidence"`
	Evidence      []string `json:"evidence,omitempty"`
}

// VLANLeak is an L2 port that reaches devices in different monitored subnets.
type VLANLeak struct {
	VlanSrc    string   `json:"vlan_src"`
	VlanDst    string   `json:"vlan_dst"`
	Count      int      `json:"count"`
	SourceMAC  string   `json:"source_mac,omitempty"`
	HostIDs    []int64  `json:"host_ids,omitempty"`
	Port       string   `json:"port,omitempty"`
	Confidence int      `json:"confidence"`
	Evidence   []string `json:"evidence,omitempty"`
}

// DuplexMismatch compares the advertised speed of both ends of an L2 link.
type DuplexMismatch struct {
	Port       string   `json:"port"`
	SpeedA     int64    `json:"speed_a,omitempty"`
	SpeedB     int64    `json:"speed_b,omitempty"`
	DuplexA    string   `json:"duplex_a,omitempty"`
	DuplexB    string   `json:"duplex_b,omitempty"`
	HostA      int64    `json:"host_a"`
	HostB      int64    `json:"host_b"`
	Confidence int      `json:"confidence"`
	Evidence   []string `json:"evidence,omitempty"`
}

// MTUProblem flags a low-MTU local path together with loss symptoms.
type MTUProblem struct {
	Src             string   `json:"src"`
	Dst             string   `json:"dst"`
	MTUPath         int      `json:"mtu_path"`
	FragmentedCount int      `json:"fragmented_count"`
	Confidence      int      `json:"confidence"`
	Evidence        []string `json:"evidence,omitempty"`
}

// DiagnosticAnomaly is the common envelope emitted by every detector.
// Cause, Impact, NextSteps and Limitations are deterministic explanations
// derived from Type; Recommendation is kept for backward compatibility.
type DiagnosticAnomaly struct {
	ID             string         `json:"id"`
	Type           string         `json:"type"`
	Category       string         `json:"category"`
	Severity       string         `json:"severity"`
	Title          string         `json:"title"`
	Detail         string         `json:"detail"`
	HostIDs        []int64        `json:"host_ids,omitempty"`
	HostIPs        []string       `json:"host_ips,omitempty"`
	Confidence     int            `json:"confidence"`
	Evidence       []string       `json:"evidence,omitempty"`
	Recommendation string         `json:"recommendation,omitempty"`
	Cause          string         `json:"cause,omitempty"`
	Impact         string         `json:"impact,omitempty"`
	NextSteps      []string       `json:"next_steps,omitempty"`
	Limitations    string         `json:"limitations,omitempty"`
	Hypotheses     []Hypothesis   `json:"hypotheses,omitempty"`
	HostKey        string         `json:"host_key,omitempty"`
	Review         *FindingReview `json:"review,omitempty"`
	DetectedAt     time.Time      `json:"detected_at"`
}

// Hypothesis is a deterministic, ranked explanation for a finding. It is
// always a lead to check by a person, never a claim of compromise.
type Hypothesis struct {
	Rank        int      `json:"rank"`
	Label       string   `json:"label"`
	Confidence  int      `json:"confidence"`
	Why         string   `json:"why"`
	Evidence    []string `json:"evidence,omitempty"`
	Check       string   `json:"check"`
	Limitations string   `json:"limitations,omitempty"`
}

// FindingReview is a local human triage decision for a diagnostic finding.
// It is keyed by type and stable host identity, not by the volatile finding ID.
type FindingReview struct {
	SiteID      int64     `json:"site_id"`
	FindingType string    `json:"finding_type"`
	HostKey     string    `json:"host_key"`
	Status      string    `json:"status"`
	Note        string    `json:"note,omitempty"`
	Reviewer    string    `json:"reviewer,omitempty"`
	ExpiresAt   time.Time `json:"expires_at"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ARPObservation is one IP→MAC binding seen during a scan. It is the raw
// evidence used to detect ARP conflicts and possible L2 impersonation without
// capturing packets or requiring privileged access.
type ARPObservation struct {
	ID         int64     `json:"id"`
	SiteID     int64     `json:"site_id"`
	IP         string    `json:"ip"`
	MAC        string    `json:"mac"`
	Hostname   string    `json:"hostname,omitempty"`
	IsGateway  bool      `json:"is_gateway,omitempty"`
	ScanID     int64     `json:"scan_id,omitempty"`
	ObservedAt time.Time `json:"observed_at"`
}

// DHCPObservation is one DHCP server reply seen by the active UDP probe. It
// uses only unprivileged datagram sockets: no raw frames and no promiscuous
// mode. ServerID is option 54 (or the source when the option is absent).
type DHCPObservation struct {
	ID           int64     `json:"id"`
	SiteID       int64     `json:"site_id"`
	IP           string    `json:"ip"`
	ServerID     string    `json:"server_id,omitempty"`
	MAC          string    `json:"mac,omitempty"`
	MessageType  string    `json:"message_type"`
	Gateway      string    `json:"gateway,omitempty"`
	Interface    string    `json:"interface,omitempty"`
	LeaseSeconds int64     `json:"lease_seconds,omitempty"`
	Subnet       string    `json:"subnet,omitempty"`
	Routers      []string  `json:"routers,omitempty"`
	DNS          []string  `json:"dns,omitempty"`
	ScanID       int64     `json:"scan_id,omitempty"`
	ObservedAt   time.Time `json:"observed_at"`
}

// DiagnosticsReport is the result of an enterprise diagnostics run.
type DiagnosticsReport struct {
	SiteID          int64               `json:"site_id"`
	GeneratedAt     time.Time           `json:"generated_at"`
	From            time.Time           `json:"from"`
	To              time.Time           `json:"to"`
	EnterpriseMode  bool                `json:"enterprise_mode"`
	HostsAnalyzed   int                 `json:"hosts_analyzed"`
	SamplesAnalyzed int                 `json:"samples_analyzed"`
	Anomalies       []DiagnosticAnomaly `json:"anomalies"`
	Counts          map[string]int      `json:"counts"`
	STP             []STPLoop           `json:"stp"`
	VLAN            []VLANLeak          `json:"vlan"`
	Duplex          []DuplexMismatch    `json:"duplex"`
	MTU             []MTUProblem        `json:"mtu"`
	Limitations     []string            `json:"limitations"`
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

// Integration is an outbound event exporter (Elasticsearch, webhook, syslog,
// Kafka REST Proxy, Redis Streams, chat services). Secrets live encrypted in
// config_enc and are masked before reaching clients.
type Integration struct {
	ID         int64          `json:"id"`
	Name       string         `json:"name"`
	Type       string         `json:"type"`
	Enabled    bool           `json:"enabled"`
	Config     map[string]any `json:"config"`
	HasSecret  bool           `json:"has_secret"`
	LastTestAt *time.Time     `json:"last_test_at,omitempty"`
	LastError  string         `json:"last_error,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
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
	SiteID     int64          `json:"site_id,omitempty"`
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
	SiteID    int64     `json:"site_id,omitempty"`
	Name      string    `json:"name"`
	Format    string    `json:"format"`
	Kind      string    `json:"kind"`
	Size      int64     `json:"size"`
	Path      string    `json:"-"`
	CreatedAt time.Time `json:"created_at"`
}

// GraphNode is a topology vertex.
type GraphNode struct {
	ID           string     `json:"id"`
	Kind         string     `json:"kind"` // host | gateway | switch | router | internet | network
	HostID       int64      `json:"host_id,omitempty"`
	Label        string     `json:"label"`
	IP           string     `json:"ip,omitempty"`
	MAC          string     `json:"mac,omitempty"`
	Vendor       string     `json:"vendor,omitempty"`
	DeviceType   DeviceType `json:"device_type,omitempty"`
	Status       HostStatus `json:"status,omitempty"`
	IsGateway    bool       `json:"is_gateway,omitempty"`
	IsDHCPServer bool       `json:"is_dhcp_server,omitempty"`
	IsSelf       bool       `json:"is_self,omitempty"`
	NetworkID    int64      `json:"network_id,omitempty"`
	Network      string     `json:"network,omitempty"`
	Confidence   int        `json:"confidence,omitempty"`
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

// SiteBundle is the portable JSON representation of one site: its identity,
// network definitions and host metadata. It intentionally excludes scan
// history, metrics and alerts so it stays small and mergeable.
type SiteBundle struct {
	Schema     string          `json:"schema"`
	ExportedAt time.Time       `json:"exported_at"`
	Site       SiteInfo        `json:"site"`
	Networks   []BundleNetwork `json:"networks"`
	Hosts      []BundleHost    `json:"hosts"`
}

// SiteInfo identifies the exported site.
type SiteInfo struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description,omitempty"`
}

// BundleNetwork is a network definition inside a bundle.
type BundleNetwork struct {
	Name      string `json:"name"`
	CIDR      string `json:"cidr"`
	Interface string `json:"interface,omitempty"`
	Enabled   bool   `json:"enabled"`
}

// BundleHost carries the stable identity and the user-maintained metadata of
// a host. Operational telemetry (ports, metrics, counters) is not exported.
type BundleHost struct {
	Identity   string   `json:"identity"`
	IP         string   `json:"ip"`
	MAC        string   `json:"mac,omitempty"`
	Hostname   string   `json:"hostname,omitempty"`
	Vendor     string   `json:"vendor,omitempty"`
	Model      string   `json:"model,omitempty"`
	OS         string   `json:"os,omitempty"`
	DeviceType string   `json:"device_type,omitempty"`
	Favorite   bool     `json:"favorite"`
	Notes      string   `json:"notes,omitempty"`
	Tags       []string `json:"tags,omitempty"`
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
	Demo         bool              `json:"demo,omitempty"`
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
	Demo          bool   `json:"demo,omitempty"`
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
	SiteID     int64
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
	SiteID int64
	Status string
	Limit  int
	Page   int
}

// AlertFilter filters alerts.
type AlertFilter struct {
	SiteID   int64
	Status   string
	Severity string
	HostID   int64
	Search   string
	From     *time.Time
	To       *time.Time
	Page     int
	Limit    int
}

// Widget is one tile of a custom dashboard. Config is opaque to the backend
// apart from the validated source field, which must belong to the catalog.
type Widget struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Position struct {
		X int `json:"x"`
		Y int `json:"y"`
	} `json:"position"`
	Size struct {
		W int `json:"w"`
		H int `json:"h"`
	} `json:"size"`
	Config map[string]any `json:"config,omitempty"`
}

// Dashboard is a user-defined view for one site (project).
type Dashboard struct {
	ID        int64     `json:"id"`
	SiteID    int64     `json:"site_id"`
	Name      string    `json:"name"`
	Widgets   []Widget  `json:"widgets"`
	CreatedAt time.Time `json:"created_at"`
	Modified  time.Time `json:"modified"`
}

// WifiAP is one access point observed during a passive 802.11 capture.
type WifiAP struct {
	ID              int64     `json:"id"`
	SiteID          int64     `json:"site_id"`
	BSSID           string    `json:"bssid"`
	SSID            string    `json:"ssid"`
	Channel         int       `json:"channel"`
	Band            string    `json:"band"`
	Encryption      string    `json:"encryption"`
	SignalDBM       int       `json:"signal_dbm"`
	Vendor          string    `json:"vendor,omitempty"`
	Beacons         int64     `json:"beacons"`
	Clients         int       `json:"clients"`
	Rogue           bool      `json:"rogue"`
	RogueInfo       string    `json:"rogue_info,omitempty"`
	RogueConfidence int       `json:"rogue_confidence,omitempty"`
	FirstSeen       time.Time `json:"first_seen"`
	LastSeen        time.Time `json:"last_seen"`
}

// WifiClient is a station observed in the same capture.
type WifiClient struct {
	ID        int64     `json:"id"`
	SiteID    int64     `json:"site_id"`
	MAC       string    `json:"mac"`
	APBSSID   string    `json:"ap_bssid"`
	SignalDBM int       `json:"signal_dbm"`
	LastSeen  time.Time `json:"last_seen"`
}

// WifiChannel aggregates AP load per channel for interference analysis.
type WifiChannel struct {
	Channel int    `json:"channel"`
	Band    string `json:"band"`
	APs     int    `json:"aps"`
	Clients int    `json:"clients"`
	Overlap int    `json:"overlap"`
	Load    string `json:"load"` // baja | media | alta
	Suggest string `json:"suggest,omitempty"`
}

// VPNTunnel is a heuristic view of a VPN tunnel observed on the host or in
// the scan inventory. Bytes are cumulative counters when available (local
// interfaces); remote-only observations report zero with an explicit note.
type VPNTunnel struct {
	ID         int64     `json:"id"`
	SiteID     int64     `json:"site_id"`
	Identity   string    `json:"-"`
	Type       string    `json:"type"` // wireguard | openvpn | ipsec | pptp
	LocalIP    string    `json:"local_ip,omitempty"`
	RemoteIP   string    `json:"remote_ip,omitempty"`
	LocalPort  int       `json:"local_port,omitempty"`
	RemotePort int       `json:"remote_port,omitempty"`
	BytesSent  int64     `json:"bytes_sent"`
	BytesRecv  int64     `json:"bytes_recv"`
	State      string    `json:"state"` // active | observed | stale
	Evidence   []string  `json:"evidence,omitempty"`
	Confidence int       `json:"confidence"`
	FirstSeen  time.Time `json:"first_seen"`
	LastSeen   time.Time `json:"last_seen"`
}

// HistoryAggregate summarises availability/latency samples in one period.
type HistoryAggregate struct {
	Samples      int     `json:"samples"`
	Availability float64 `json:"availability"`
	AvgRTT       float64 `json:"avg_rtt"`
	MinRTT       float64 `json:"min_rtt"`
	MaxRTT       float64 `json:"max_rtt"`
	HostsUp      int     `json:"hosts_up"`
	HostsDown    int     `json:"hosts_down"`
}

// HistoryHost is one device's behaviour inside a comparison period.
type HistoryHost struct {
	ID           int64      `json:"id"`
	IP           string     `json:"ip"`
	Hostname     string     `json:"hostname,omitempty"`
	MAC          string     `json:"mac,omitempty"`
	DeviceType   DeviceType `json:"device_type"`
	Status       HostStatus `json:"status"`
	Samples      int        `json:"samples"`
	UpSamples    int        `json:"up_samples"`
	DownSamples  int        `json:"down_samples"`
	Availability float64    `json:"availability"`
	AvgRTT       float64    `json:"avg_rtt"`
	FirstSeen    time.Time  `json:"first_seen,omitempty"`
	LastSeen     time.Time  `json:"last_seen,omitempty"`
}

// HistoryTopology is a compact, comparable view of a topology snapshot.
type HistoryTopology struct {
	Nodes       int            `json:"nodes"`
	Edges       int            `json:"edges"`
	ByType      map[string]int `json:"by_type,omitempty"`
	NodeIDs     []string       `json:"node_ids,omitempty"`
	EdgeIDs     []string       `json:"edge_ids,omitempty"`
	Fingerprint string         `json:"fingerprint,omitempty"`
	SnapshotAt  *time.Time     `json:"snapshot_at,omitempty"`
}

// HistoryPeriod is one side of a temporal comparison.
type HistoryPeriod struct {
	Label    string           `json:"label"`
	From     time.Time        `json:"from"`
	To       time.Time        `json:"to"`
	Metrics  HistoryAggregate `json:"metrics"`
	Hosts    []HistoryHost    `json:"hosts"`
	RTT      []SeriesPoint    `json:"rtt"`
	Uptime   []SeriesPoint    `json:"uptime"`
	Topology HistoryTopology  `json:"topology"`
}

// HistoryChange is one directional difference from period B to period A.
type HistoryChange struct {
	Type     string   `json:"type"`
	Entity   string   `json:"entity"` // host | topology | fleet
	ID       string   `json:"id"`
	Label    string   `json:"label,omitempty"`
	From     any      `json:"from,omitempty"`
	To       any      `json:"to,omitempty"`
	Evidence []string `json:"evidence,omitempty"`
}

// HistorySnapshot is a compact inventory/topology capture used by F21.
type HistorySnapshot struct {
	ID       int64           `json:"id"`
	SiteID   int64           `json:"site_id"`
	TS       time.Time       `json:"ts"`
	Hosts    []HistoryHost   `json:"hosts"`
	Topology HistoryTopology `json:"topology"`
}

// DiffReport compares two moments in time (F21). It is derived from stored
// metrics and, when available, lightweight inventory/topology snapshots.
type DiffReport struct {
	PeriodA     HistoryPeriod   `json:"period_a"`
	PeriodB     HistoryPeriod   `json:"period_b"`
	Changes     []HistoryChange `json:"changes"`
	Summary     map[string]int  `json:"summary"`
	Narrative   []string        `json:"narrative,omitempty"`
	Limitations []string        `json:"limitations,omitempty"`
	Truncated   bool            `json:"truncated,omitempty"`
}

// AuditSnapshot is a manually captured, compressed baseline of a site (F22).
// Data is only populated when explicitly requested by the API.
type AuditSnapshot struct {
	ID      int64              `json:"id"`
	SiteID  int64              `json:"site_id"`
	Name    string             `json:"name"`
	Notes   string             `json:"notes,omitempty"`
	TS      time.Time          `json:"ts"`
	Size    int                `json:"size"`     // compressed bytes stored
	RawSize int                `json:"raw_size"` // JSON bytes before compression
	Data    *AuditSnapshotData `json:"data,omitempty"`
}

// AuditSnapshotData is the portable, redacted content of an audit baseline.
// Secrets are never included: only configuration key names are retained.
type AuditSnapshotData struct {
	Schema       string             `json:"schema"`
	SiteID       int64              `json:"site_id"`
	SiteName     string             `json:"site_name,omitempty"`
	CapturedAt   time.Time          `json:"captured_at"`
	Hosts        []AuditHost        `json:"hosts"`
	Networks     []AuditNetwork     `json:"networks"`
	Neighbors    []AuditNeighbor    `json:"neighbors"`
	Rules        []AuditRule        `json:"rules"`
	Channels     []AuditChannel     `json:"channels"`
	Integrations []AuditIntegration `json:"integrations"`
	Topology     AuditTopologyData  `json:"topology"`
	Scans        []AuditScan        `json:"scans"`
	Alerts       AuditAlertSummary  `json:"alerts"`
	Config       AuditConfigSummary `json:"config"`
}

// AuditHost is a compact inventory entry inside an audit baseline.
type AuditHost struct {
	ID         int64      `json:"id"`
	Identity   string     `json:"identity"`
	IP         string     `json:"ip"`
	MAC        string     `json:"mac,omitempty"`
	Hostname   string     `json:"hostname,omitempty"`
	Vendor     string     `json:"vendor,omitempty"`
	Model      string     `json:"model,omitempty"`
	OS         string     `json:"os,omitempty"`
	DeviceType DeviceType `json:"device_type"`
	Status     HostStatus `json:"status"`
	Confidence int        `json:"confidence"`
	IsGateway  bool       `json:"is_gateway,omitempty"`
	Favorite   bool       `json:"favorite,omitempty"`
	Notes      string     `json:"notes,omitempty"`
	Tags       []string   `json:"tags,omitempty"`
	Ports      []Port     `json:"ports,omitempty"`
	FirstSeen  time.Time  `json:"first_seen,omitempty"`
	LastSeen   time.Time  `json:"last_seen,omitempty"`
}

type AuditNetwork struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	CIDR      string `json:"cidr"`
	Interface string `json:"interface,omitempty"`
	Enabled   bool   `json:"enabled"`
	IsLocal   bool   `json:"is_local,omitempty"`
}

type AuditNeighbor struct {
	HostIdentity  string `json:"host_identity"`
	Protocol      string `json:"protocol"`
	RemoteName    string `json:"remote_name,omitempty"`
	RemoteIP      string `json:"remote_ip,omitempty"`
	RemoteMAC     string `json:"remote_mac,omitempty"`
	RemotePort    string `json:"remote_port,omitempty"`
	LocalPort     string `json:"local_port,omitempty"`
	RemoteChassis string `json:"remote_chassis,omitempty"`
	Confidence    int    `json:"confidence"`
}

type AuditRule struct {
	ID          int64    `json:"id"`
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Enabled     bool     `json:"enabled"`
	Severity    string   `json:"severity"`
	ChannelIDs  []int64  `json:"channel_ids,omitempty"`
	CooldownSec int      `json:"cooldown_sec"`
	Builtin     bool     `json:"builtin,omitempty"`
	ConfigKeys  []string `json:"config_keys,omitempty"`
}

type AuditChannel struct {
	ID         int64    `json:"id"`
	Name       string   `json:"name"`
	Type       string   `json:"type"`
	Enabled    bool     `json:"enabled"`
	HasSecret  bool     `json:"has_secret,omitempty"`
	ConfigKeys []string `json:"config_keys,omitempty"`
}

type AuditIntegration struct {
	ID         int64    `json:"id"`
	Name       string   `json:"name"`
	Type       string   `json:"type"`
	Enabled    bool     `json:"enabled"`
	HasSecret  bool     `json:"has_secret,omitempty"`
	ConfigKeys []string `json:"config_keys,omitempty"`
}

type AuditTopologyData struct {
	Nodes       []GraphNode `json:"nodes"`
	Edges       []GraphEdge `json:"edges"`
	Fingerprint string      `json:"fingerprint,omitempty"`
}

type AuditScan struct {
	ID         int64      `json:"id"`
	CIDR       string     `json:"cidr"`
	Profile    string     `json:"profile,omitempty"`
	Status     string     `json:"status"`
	HostsFound int        `json:"hosts_found"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
}

type AuditAlertSummary struct {
	Active   int `json:"active"`
	Critical int `json:"critical"`
	Warnings int `json:"warnings"`
}

type AuditConfigSummary struct {
	MonitorEnabled      bool   `json:"monitor_enabled"`
	MonitorInterval     string `json:"monitor_interval,omitempty"`
	EnterpriseMode      bool   `json:"enterprise_mode"`
	AlertsEnabled       bool   `json:"alerts_enabled"`
	IntegrationsEnabled bool   `json:"integrations_enabled"`
	MetricsDays         int    `json:"metrics_days"`
	RetentionDays       int    `json:"retention_days"`
}

// AuditChange is one difference between two audit baselines.
type AuditChange struct {
	Type     string   `json:"type"`
	Entity   string   `json:"entity"`
	ID       string   `json:"id"`
	Label    string   `json:"label,omitempty"`
	Field    string   `json:"field,omitempty"`
	From     any      `json:"from,omitempty"`
	To       any      `json:"to,omitempty"`
	Evidence []string `json:"evidence,omitempty"`
}

// AuditDiff is the result of comparing two audit baselines.
type AuditDiff struct {
	FromID      int64          `json:"from_id"`
	ToID        int64          `json:"to_id"`
	FromName    string         `json:"from_name"`
	ToName      string         `json:"to_name"`
	FromTS      time.Time      `json:"from_ts"`
	ToTS        time.Time      `json:"to_ts"`
	Changes     []AuditChange  `json:"changes"`
	Summary     map[string]int `json:"summary"`
	Truncated   bool           `json:"truncated,omitempty"`
	Limitations []string       `json:"limitations,omitempty"`
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
