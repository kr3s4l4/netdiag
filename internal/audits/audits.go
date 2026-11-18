// Package audits implements F22 audit mode: explicit, compressed baselines of
// a site and diffs between them. Baselines are redacted: configuration secrets
// are never copied into the stored payload.
package audits

import (
	"sort"
	"time"

	"github.com/kr3s4l4/netdiag/internal/config"
	"github.com/kr3s4l4/netdiag/internal/history"
	"github.com/kr3s4l4/netdiag/internal/model"
	"github.com/kr3s4l4/netdiag/internal/store"
	"github.com/kr3s4l4/netdiag/internal/topology"
)

const snapshotSchema = "netdiag/audit/v1"

// MaxAuditChanges bounds a single diff response.
const MaxAuditChanges = 500

// Capture builds a redacted, deterministic view of the current site state.
func Capture(st *store.Store, cfg *config.Config, siteID int64) (*model.AuditSnapshotData, error) {
	if siteID <= 0 {
		siteID = 1
	}
	site, err := st.GetSite(siteID)
	if err != nil {
		return nil, err
	}
	hosts, err := st.ListAllHostsForSite(siteID, 20000)
	if err != nil {
		return nil, err
	}
	neighbors, err := st.NeighborsForSite(siteID)
	if err != nil {
		neighbors = map[int64][]model.Neighbor{}
	}
	networks, err := st.ListNetworksBySite(siteID)
	if err != nil {
		return nil, err
	}
	rules, err := st.ListRules()
	if err != nil {
		return nil, err
	}
	channels, err := st.ListChannels()
	if err != nil {
		return nil, err
	}
	integrations, err := st.ListIntegrations()
	if err != nil {
		return nil, err
	}
	scans, err := st.ListScans(model.ScanFilter{SiteID: siteID, Page: 1, Limit: 25})
	if err != nil {
		return nil, err
	}
	stats, err := st.AlertStatsForSite(siteID)
	if err != nil {
		stats = model.AlertStats{}
	}

	data := &model.AuditSnapshotData{
		Schema:       snapshotSchema,
		SiteID:       siteID,
		CapturedAt:   time.Now(),
		Hosts:        make([]model.AuditHost, 0, len(hosts)),
		Networks:     make([]model.AuditNetwork, 0, len(networks)),
		Neighbors:    []model.AuditNeighbor{},
		Rules:        make([]model.AuditRule, 0, len(rules)),
		Channels:     make([]model.AuditChannel, 0, len(channels)),
		Integrations: make([]model.AuditIntegration, 0, len(integrations)),
		Topology:     model.AuditTopologyData{Nodes: []model.GraphNode{}, Edges: []model.GraphEdge{}},
		Scans:        make([]model.AuditScan, 0, len(scans.Items)),
		Alerts: model.AuditAlertSummary{
			Active: stats.Active, Critical: stats.Critical, Warnings: stats.Warning,
		},
		Config: configSummary(cfg),
	}
	if site != nil {
		data.SiteName = site.Name
	}
	for _, h := range hosts {
		data.Hosts = append(data.Hosts, auditHost(h))
		for _, n := range neighbors[h.ID] {
			data.Neighbors = append(data.Neighbors, model.AuditNeighbor{
				HostIdentity: h.Identity, Protocol: n.Protocol, RemoteName: n.RemoteName,
				RemoteIP: n.RemoteIP, RemoteMAC: n.RemoteMAC, RemotePort: n.RemotePort,
				LocalPort: n.LocalPort, RemoteChassis: n.RemoteChassis, Confidence: n.Confidence,
			})
		}
	}
	for _, n := range networks {
		data.Networks = append(data.Networks, model.AuditNetwork{
			ID: n.ID, Name: n.Name, CIDR: n.CIDR, Interface: n.Interface,
			Enabled: n.Enabled, IsLocal: n.IsLocal,
		})
	}
	for _, rule := range rules {
		data.Rules = append(data.Rules, model.AuditRule{
			ID: rule.ID, Name: rule.Name, Type: rule.Type, Enabled: rule.Enabled,
			Severity: rule.Severity, ChannelIDs: append([]int64{}, rule.ChannelIDs...),
			CooldownSec: rule.CooldownSec, Builtin: rule.Builtin, ConfigKeys: mapKeys(rule.Config),
		})
	}
	for _, ch := range channels {
		data.Channels = append(data.Channels, model.AuditChannel{
			ID: ch.ID, Name: ch.Name, Type: ch.Type, Enabled: ch.Enabled,
			HasSecret: ch.HasSecret, ConfigKeys: mapKeys(ch.Config),
		})
	}
	for _, it := range integrations {
		data.Integrations = append(data.Integrations, model.AuditIntegration{
			ID: it.ID, Name: it.Name, Type: it.Type, Enabled: it.Enabled,
			HasSecret: it.HasSecret, ConfigKeys: mapKeys(it.Config),
		})
	}
	for _, sc := range scans.Items {
		data.Scans = append(data.Scans, model.AuditScan{
			ID: sc.ID, CIDR: sc.CIDR, Profile: sc.Profile, Status: sc.Status,
			HostsFound: sc.HostsFound, StartedAt: sc.StartedAt, FinishedAt: sc.FinishedAt,
		})
	}

	graph := topology.Build(hosts, neighbors, networks)
	data.Topology.Nodes = graph.Nodes
	data.Topology.Edges = graph.Edges
	nodeIDs := make([]string, 0, len(graph.Nodes))
	edgeIDs := make([]string, 0, len(graph.Edges))
	for _, n := range graph.Nodes {
		nodeIDs = append(nodeIDs, n.ID)
	}
	for _, e := range graph.Edges {
		edgeIDs = append(edgeIDs, e.ID)
	}
	data.Topology.Fingerprint = history.Fingerprint(nodeIDs, edgeIDs)

	sort.Slice(data.Neighbors, func(i, j int) bool {
		if data.Neighbors[i].HostIdentity != data.Neighbors[j].HostIdentity {
			return data.Neighbors[i].HostIdentity < data.Neighbors[j].HostIdentity
		}
		return neighborKey(data.Neighbors[i]) < neighborKey(data.Neighbors[j])
	})
	return data, nil
}

func auditHost(h model.Host) model.AuditHost {
	tags := append([]string{}, h.Tags...)
	sort.Strings(tags)
	ports := append([]model.Port{}, h.Ports...)
	sort.Slice(ports, func(i, j int) bool {
		if ports[i].Protocol != ports[j].Protocol {
			return ports[i].Protocol < ports[j].Protocol
		}
		return ports[i].Number < ports[j].Number
	})
	return model.AuditHost{
		ID: h.ID, Identity: h.Identity, IP: h.IP, MAC: h.MAC, Hostname: h.Hostname,
		Vendor: h.Vendor, Model: h.Model, OS: h.OS, DeviceType: h.DeviceType,
		Status: h.Status, Confidence: h.Confidence, IsGateway: h.IsGateway,
		Favorite: h.Favorite, Notes: h.Notes, Tags: tags, Ports: ports,
		FirstSeen: h.FirstSeen, LastSeen: h.LastSeen,
	}
}

func mapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func configSummary(cfg *config.Config) model.AuditConfigSummary {
	if cfg == nil {
		return model.AuditConfigSummary{}
	}
	return model.AuditConfigSummary{
		MonitorEnabled:      cfg.Monitor.Enabled,
		MonitorInterval:     cfg.Monitor.Interval.Duration().String(),
		EnterpriseMode:      cfg.Diagnostics.EnterpriseMode,
		AlertsEnabled:       cfg.Alerts.Enabled,
		IntegrationsEnabled: cfg.Integrations.Enabled,
		MetricsDays:         cfg.Storage.MetricsDays,
		RetentionDays:       cfg.Storage.RetentionDays,
	}
}

func neighborKey(n model.AuditNeighbor) string {
	return n.Protocol + "|" + n.RemoteMAC + "|" + n.RemoteIP + "|" + n.RemotePort + "|" + n.LocalPort
}
