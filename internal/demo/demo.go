// Package demo builds a synthetic, self-contained NetDiag database. It never
// opens a socket and never reads the host's real inventory.
package demo

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/kr3s4l4/netdiag/internal/config"
	"github.com/kr3s4l4/netdiag/internal/cryptoutil"
	"github.com/kr3s4l4/netdiag/internal/model"
	"github.com/kr3s4l4/netdiag/internal/store"
)

const (
	// MarkerName is written inside the demo directory so the UI and exports
	// can tell synthetic data apart from a real deployment.
	MarkerName = "demo.json"
	dbName     = "netdiag.db"
	maxSites   = 20
	maxHosts   = 5000
)

// Options controls the synthetic dataset. Now is injectable for tests.
type Options struct {
	Seed    int64
	Sites   int
	Hosts   int
	DataDir string
	Now     time.Time
}

// Result summarises what was generated.
type Result struct {
	DBPath       string    `json:"db_path"`
	DemoDir      string    `json:"demo_dir"`
	Seed         int64     `json:"seed"`
	Sites        int       `json:"sites"`
	Hosts        int       `json:"hosts"`
	Alerts       int       `json:"alerts"`
	Metrics      int       `json:"metrics"`
	ManagedHosts int       `json:"managed_hosts"`
	GeneratedAt  time.Time `json:"generated_at"`
}

type marker struct {
	Demo        bool      `json:"demo"`
	Seed        int64     `json:"seed"`
	Sites       int       `json:"sites"`
	Hosts       int       `json:"hosts"`
	GeneratedAt time.Time `json:"generated_at"`
}

type siteData struct {
	site    *model.Site
	network *model.Network
	hosts   []*model.Host
	gateway *model.Host
}

// Generate recreates <data_dir>/demo/netdiag.db from scratch with synthetic
// data. The real database at <data_dir>/netdiag.db is never touched.
func Generate(opts Options) (*Result, error) {
	if opts.Seed == 0 {
		opts.Seed = 1
	}
	if opts.Sites <= 0 {
		opts.Sites = 3
	}
	if opts.Sites > maxSites {
		opts.Sites = maxSites
	}
	if opts.Hosts <= 0 {
		opts.Hosts = 120
	}
	if opts.Hosts > maxHosts {
		opts.Hosts = maxHosts
	}
	if opts.DataDir == "" {
		opts.DataDir = config.DefaultDataDir()
	}
	now := opts.Now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	now = now.Truncate(time.Second)

	demoDir := filepath.Join(opts.DataDir, "demo")
	if err := os.MkdirAll(demoDir, 0o700); err != nil {
		return nil, fmt.Errorf("no se pudo crear el directorio demo: %w", err)
	}
	dbPath := filepath.Join(demoDir, dbName)
	for _, suffix := range []string{"", "-wal", "-shm"} {
		if err := os.Remove(dbPath + suffix); err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("no se pudo reiniciar la base demo: %w", err)
		}
	}
	_ = os.Remove(filepath.Join(demoDir, MarkerName))

	box, err := cryptoutil.LoadOrCreate(filepath.Join(demoDir, "secret.key"))
	if err != nil {
		return nil, err
	}
	st, err := store.Open(dbPath, box)
	if err != nil {
		return nil, err
	}
	defer st.Close()
	if err := st.EnsureBuiltinRules(); err != nil {
		return nil, err
	}

	rng := rand.New(rand.NewSource(opts.Seed))
	sites, err := buildSites(st, rng, opts.Sites, now)
	if err != nil {
		return nil, err
	}
	hosts, metrics, err := buildHosts(st, rng, sites, opts.Hosts, now)
	if err != nil {
		return nil, err
	}
	alerts, err := buildAlerts(st, rng, sites, now)
	if err != nil {
		return nil, err
	}
	if err := buildScans(st, sites, now); err != nil {
		return nil, err
	}
	if err := buildSnapshots(st, sites, now, alerts); err != nil {
		return nil, err
	}
	managed, err := st.CountManagedHosts(now.AddDate(0, 0, -30))
	if err != nil {
		return nil, err
	}

	info := marker{Demo: true, Seed: opts.Seed, Sites: len(sites), Hosts: hosts, GeneratedAt: now}
	raw, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(demoDir, MarkerName), append(raw, '\n'), 0o644); err != nil {
		return nil, err
	}
	return &Result{
		DBPath:       dbPath,
		DemoDir:      demoDir,
		Seed:         opts.Seed,
		Sites:        len(sites),
		Hosts:        hosts,
		Alerts:       alerts,
		Metrics:      metrics,
		ManagedHosts: managed,
		GeneratedAt:  now,
	}, nil
}

func buildSites(st *store.Store, rng *rand.Rand, count int, now time.Time) ([]*siteData, error) {
	existing, err := st.ListSites()
	if err != nil {
		return nil, err
	}
	var sites []*siteData
	if len(existing) > 0 {
		first := existing[0]
		sites = append(sites, &siteData{site: &first})
	}
	for i := len(sites); i < count; i++ {
		name := fmt.Sprintf("Sede Demo %d", i+1)
		s, err := st.CreateSite(name, "Sede sintética generada por netdiag demo")
		if err != nil {
			return nil, err
		}
		sites = append(sites, &siteData{site: s})
	}
	for i, sd := range sites {
		cidr := fmt.Sprintf("10.%d.0.0/24", 20+i)
		n, _, err := st.EnsureNetwork(&model.Network{
			SiteID:    sd.site.ID,
			Name:      "LAN Demo",
			CIDR:      cidr,
			Interface: "demo0",
			Enabled:   true,
			IsLocal:   true,
			CreatedAt: now.AddDate(0, 0, -30),
		})
		if err != nil {
			return nil, err
		}
		sd.network = n
	}
	return sites, nil
}

var deviceKinds = []model.DeviceType{
	model.DevRouter, model.DevSwitch, model.DevServer, model.DevWorkstation,
	model.DevPrinter, model.DevCamera, model.DevIoT, model.DevAccessPoint,
	model.DevFirewall, model.DevNAS,
}

func buildHosts(st *store.Store, rng *rand.Rand, sites []*siteData, total int, now time.Time) (int, int, error) {
	perSite := total / len(sites)
	extra := total % len(sites)
	created, metricCount := 0, 0
	for si, sd := range sites {
		n := perSite
		if si < extra {
			n++
		}
		if n == 0 {
			continue
		}
		base := 20 + si
		for i := 0; i < n; i++ {
			gateway := i == 0
			idx := created + 1
			kind := deviceKinds[rng.Intn(len(deviceKinds))]
			if gateway {
				kind = model.DevRouter
			}
			octet3 := 1 + i/250
			octet4 := 10 + i%240
			ip := fmt.Sprintf("10.%d.%d.%d", base, octet3, octet4)
			mac := fmt.Sprintf("02:%02x:%02x:%02x:%02x:%02x",
				si+1, (idx>>16)&0xff, (idx>>8)&0xff, idx&0xff, byte(rng.Intn(256)))
			hostname := fmt.Sprintf("%s-%02d", shortKind(kind), i+1)
			status := model.StatusUp
			if !gateway {
				switch roll := rng.Intn(100); {
				case roll < 6:
					status = model.StatusDown
				case roll < 10:
					status = model.StatusUnknown
				}
			}
			lastSeen := now.Add(-time.Duration(rng.Intn(12)) * time.Hour)
			if status == model.StatusDown {
				lastSeen = now.AddDate(0, 0, -(2 + rng.Intn(20)))
			}
			firstSeen := lastSeen.AddDate(0, 0, -(30 + rng.Intn(300)))
			host := &model.Host{
				IP: ip, MAC: mac, Hostname: hostname,
				Vendor: fakeVendor(kind), Model: fakeModel(kind), OS: fakeOS(kind),
				DeviceType: kind, Confidence: 70 + rng.Intn(30),
				Status: status, IsGateway: gateway,
				LatencyMS: 1 + rng.Float64()*38, TTL: 64,
				FirstSeen: firstSeen, LastSeen: lastSeen,
				NetworkID: sd.network.ID, SiteID: sd.site.ID,
				Ports:    portsFor(kind, hostname, now),
				Evidence: []string{"demo:synthetic"},
			}
			id, _, _, _, err := st.UpsertHost(host)
			if err != nil {
				return 0, 0, err
			}
			host.ID = id
			tags := []string{"demo"}
			if rng.Intn(100) < 5 {
				tags = append(tags, "guest")
			} else if rng.Intn(100) < 3 {
				tags = append(tags, "excluded")
			}
			if err := st.UpdateHostMeta(id, nil, &tags, nil); err != nil {
				return 0, 0, err
			}
			sd.hosts = append(sd.hosts, host)
			if gateway {
				sd.gateway = host
			}
			created++
			metricCount += addMetrics(st, rng, host, now, status)
		}
	}
	for _, sd := range sites {
		if err := linkNeighbors(st, sd); err != nil {
			return 0, 0, err
		}
	}
	return created, metricCount, nil
}

func addMetrics(st *store.Store, rng *rand.Rand, h *model.Host, now time.Time, status model.HostStatus) int {
	samples := make([]model.Metric, 0, 12)
	for i := 11; i >= 0; i-- {
		up := status != model.StatusDown || i > 6
		rtt := h.LatencyMS + rng.Float64()*4 - 2
		if rtt < 0.2 {
			rtt = 0.2
		}
		samples = append(samples, model.Metric{
			HostID: h.ID, TS: now.Add(-time.Duration(i*2) * time.Hour),
			RTTMS: rtt, Up: up, Loss: rng.Float64() * 0.4,
		})
	}
	if err := st.AddMetrics(samples); err != nil {
		return 0
	}
	return len(samples)
}

func linkNeighbors(st *store.Store, sd *siteData) error {
	if sd.gateway == nil || len(sd.hosts) < 2 {
		return nil
	}
	for _, h := range sd.hosts {
		if h.ID == sd.gateway.ID {
			continue
		}
		n := []model.Neighbor{{
			Protocol: "lldp", RemoteName: sd.gateway.Hostname,
			RemoteIP: sd.gateway.IP, RemoteMAC: sd.gateway.MAC,
			RemotePort: "Gi0/1", LocalPort: "Gi0/2",
			RemoteChassis: sd.gateway.MAC, Confidence: 92,
		}}
		if err := st.ReplaceNeighbors(h.ID, n); err != nil {
			return err
		}
	}
	return nil
}

func buildAlerts(st *store.Store, rng *rand.Rand, sites []*siteData, now time.Time) (int, error) {
	count := 0
	for _, sd := range sites {
		for _, h := range sd.hosts {
			var a *model.Alert
			switch {
			case h.Status == model.StatusDown:
				a = &model.Alert{
					RuleName: "host_down", HostID: h.ID, HostIP: h.IP, HostName: h.Hostname,
					Severity: model.SeverityCritical, Title: "Host caído en demo",
					Message: fmt.Sprintf("%s dejó de responder en la sede %s", h.Hostname, sd.site.Name),
				}
			case h.DeviceType == model.DevServer && rng.Intn(100) < 25:
				a = &model.Alert{
					RuleName: "high_latency", HostID: h.ID, HostIP: h.IP, HostName: h.Hostname,
					Severity: model.SeverityWarning, Title: "Latencia alta en demo",
					Message: fmt.Sprintf("RTT medio %.1f ms en %s", h.LatencyMS, h.Hostname),
				}
			case rng.Intn(100) < 4:
				a = &model.Alert{
					RuleName: "new_device", HostID: h.ID, HostIP: h.IP, HostName: h.Hostname,
					Severity: model.SeverityInfo, Title: "Nuevo dispositivo en demo",
					Message: fmt.Sprintf("Se catalogó %s como %s", h.Hostname, h.DeviceType),
				}
			}
			if a == nil {
				continue
			}
			a.Data = map[string]any{"demo": true, "site": sd.site.Name, "at": now.Format(time.RFC3339)}
			dedupe := fmt.Sprintf("demo:%d:%s:%s", h.ID, a.RuleName, now.Format("20060102"))
			if _, created, err := st.CreateAlert(a, dedupe); err != nil {
				return count, err
			} else if created {
				count++
			}
		}
	}
	return count, nil
}

func buildScans(st *store.Store, sites []*siteData, now time.Time) error {
	for _, sd := range sites {
		sc, err := st.CreateScan(&model.Scan{
			SiteID: sd.site.ID, NetworkID: sd.network.ID, CIDR: sd.network.CIDR,
			Profile: "common", Total: len(sd.hosts), Trigger: "demo",
			Options: model.ScanOptions{Profile: "common", TimeoutMS: 800, Concurrency: 32, MaxHosts: 1000},
		})
		if err != nil {
			return err
		}
		if err := st.StartScan(sc.ID, len(sd.hosts)); err != nil {
			return err
		}
		if err := st.UpdateScanProgress(sc.ID, "inventariando", len(sd.hosts), len(sd.hosts), len(sd.hosts), 1); err != nil {
			return err
		}
		if err := st.FinishScan(sc.ID, model.ScanDone, len(sd.hosts), ""); err != nil {
			return err
		}
		if err := st.TouchNetworkScan(sd.network.ID, now); err != nil {
			return err
		}
	}
	return nil
}

func buildSnapshots(st *store.Store, sites []*siteData, now time.Time, alerts int) error {
	for _, sd := range sites {
		data := &model.AuditSnapshotData{
			Schema:     "netdiag/audit/v1",
			SiteID:     sd.site.ID,
			SiteName:   sd.site.Name,
			CapturedAt: now,
			Hosts:      make([]model.AuditHost, 0, len(sd.hosts)),
			Networks: []model.AuditNetwork{{
				ID: sd.network.ID, Name: sd.network.Name, CIDR: sd.network.CIDR,
				Interface: sd.network.Interface, Enabled: true, IsLocal: true,
			}},
			Alerts: model.AuditAlertSummary{Active: alerts, Critical: alerts, Warnings: 0},
			Config: model.AuditConfigSummary{
				MonitorEnabled: true, MonitorInterval: "1h", AlertsEnabled: true,
				MetricsDays: 30, RetentionDays: 7,
			},
		}
		for _, h := range sd.hosts {
			identity := store.IdentityFor(h.IP, h.MAC)
			data.Hosts = append(data.Hosts, model.AuditHost{
				ID: h.ID, Identity: identity, IP: h.IP, MAC: h.MAC, Hostname: h.Hostname,
				Vendor: h.Vendor, Model: h.Model, OS: h.OS, DeviceType: h.DeviceType,
				Status: h.Status, Confidence: h.Confidence, IsGateway: h.IsGateway,
				Tags: h.Tags, Ports: h.Ports, FirstSeen: h.FirstSeen, LastSeen: h.LastSeen,
			})
			data.Topology.Nodes = append(data.Topology.Nodes, model.GraphNode{
				ID: identity, Kind: "host", Label: h.Hostname, HostID: h.ID,
				IP: h.IP, MAC: h.MAC, Vendor: h.Vendor, DeviceType: h.DeviceType,
				Status: h.Status, IsGateway: h.IsGateway, NetworkID: h.NetworkID,
				Network: sd.network.Name, Confidence: h.Confidence,
			})
			if sd.gateway != nil && h.ID != sd.gateway.ID {
				gwIdentity := store.IdentityFor(sd.gateway.IP, sd.gateway.MAC)
				data.Topology.Edges = append(data.Topology.Edges, model.GraphEdge{
					ID: identity + "->" + gwIdentity, Source: identity, Target: gwIdentity,
					Kind: "lldp", Label: "Gi0/1 ↔ Gi0/2", Confidence: 92,
				})
			}
		}
		snap := &model.AuditSnapshot{
			SiteID: sd.site.ID, Name: "Línea base demo",
			Notes: "Datos sintéticos. No proceden de una red real.",
			TS:    now, Data: data,
		}
		if err := st.CreateAuditSnapshot(snap); err != nil {
			return err
		}
	}
	return st.AddAudit(&model.AuditEntry{
		TS: now, Actor: "demo", Action: "demo.generated",
		Target: "dataset", Detail: map[string]any{"demo": true},
		IP: "127.0.0.1", UserAgent: "netdiag-demo",
	})
}

func portsFor(kind model.DeviceType, hostname string, now time.Time) []model.Port {
	defs := map[model.DeviceType][]int{
		model.DevRouter:      {22, 53, 80, 443, 161},
		model.DevSwitch:      {22, 80, 161},
		model.DevServer:      {22, 80, 443, 5432},
		model.DevWorkstation: {135, 445, 3389},
		model.DevPrinter:     {80, 515, 631, 9100},
		model.DevCamera:      {80, 554},
		model.DevIoT:         {80, 1883},
		model.DevAccessPoint: {22, 80, 443},
		model.DevFirewall:    {22, 443},
		model.DevNAS:         {22, 80, 443, 5000},
	}[kind]
	ports := make([]model.Port, 0, len(defs))
	for _, n := range defs {
		p := model.Port{Number: n, Protocol: "tcp", State: "open", Service: serviceName(n)}
		if n == 443 {
			p.Product = "nginx"
			p.Version = "1.27"
			p.TLS = &model.TLSCert{
				Subject: "CN=" + hostname + ".demo.local", Issuer: "CN=Demo CA",
				NotAfter: now.AddDate(0, 2, 0), DNSNames: []string{hostname + ".demo.local"},
			}
		}
		ports = append(ports, p)
	}
	return ports
}

func serviceName(port int) string {
	switch port {
	case 22:
		return "ssh"
	case 53:
		return "domain"
	case 80, 8080, 5000, 9100:
		return "http"
	case 443:
		return "https"
	case 161:
		return "snmp"
	case 135, 445:
		return "microsoft-ds"
	case 3389:
		return "ms-wbt-server"
	case 515, 631:
		return "ipp"
	case 554:
		return "rtsp"
	case 1883:
		return "mqtt"
	case 5432:
		return "postgresql"
	default:
		return "unknown"
	}
}

func shortKind(k model.DeviceType) string {
	if len(k) > 6 {
		return string(k[:6])
	}
	return string(k)
}

func fakeVendor(k model.DeviceType) string {
	switch k {
	case model.DevRouter, model.DevSwitch, model.DevFirewall, model.DevAccessPoint:
		return "DemoNetworks"
	case model.DevServer, model.DevNAS:
		return "DemoSystems"
	case model.DevPrinter:
		return "DemoPrint"
	case model.DevCamera, model.DevIoT:
		return "DemoThings"
	default:
		return "DemoDevices"
	}
}

func fakeModel(k model.DeviceType) string {
	return fmt.Sprintf("%s-%s", shortKind(k), "1000")
}

func fakeOS(k model.DeviceType) string {
	switch k {
	case model.DevRouter, model.DevSwitch, model.DevFirewall:
		return "DemoOS 5.2"
	case model.DevServer:
		return "Demo Linux 6"
	case model.DevWorkstation, model.DevLaptop:
		return "Demo Desktop 11"
	default:
		return ""
	}
}

// ErrNoSites is returned internally when no site could be created.
var ErrNoSites = errors.New("no se pudo crear ninguna sede demo")
