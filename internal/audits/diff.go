package audits

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/kr3s4l4/netdiag/internal/model"
)

type fieldPair struct {
	from any
	to   any
}

// Diff compares two audit baselines (from -> to). Only inventory,
// configuration and topology facts are compared: volatile latency samples and
// timestamps are intentionally excluded.
func Diff(from, to *model.AuditSnapshot) *model.AuditDiff {
	out := &model.AuditDiff{
		Changes: []model.AuditChange{},
		Summary: map[string]int{},
	}
	if from != nil {
		out.FromID, out.FromName, out.FromTS = from.ID, from.Name, from.TS
	}
	if to != nil {
		out.ToID, out.ToName, out.ToTS = to.ID, to.Name, to.TS
	}
	if from == nil || to == nil || from.Data == nil || to.Data == nil {
		out.Limitations = append(out.Limitations,
			"Uno de los snapshots no tiene datos disponibles; el diff está incompleto.")
		return out
	}
	a, b := from.Data, to.Data

	hostsA, hostsB := map[string]model.AuditHost{}, map[string]model.AuditHost{}
	for _, h := range a.Hosts {
		hostsA[hostKey(h)] = h
	}
	for _, h := range b.Hosts {
		hostsB[hostKey(h)] = h
	}
	for key, h := range hostsB {
		prev, ok := hostsA[key]
		if !ok {
			add(out, model.AuditChange{Type: "host.added", Entity: "host", ID: key,
				Label: hostLabel(h), To: string(h.Status),
				Evidence: []string{fmt.Sprintf("presente en %s", out.ToName)}})
			continue
		}
		label := hostLabel(h)
		diffFields(out, "host", key, label, map[string]fieldPair{
			"ip": {prev.IP, h.IP}, "mac": {prev.MAC, h.MAC},
			"hostname": {prev.Hostname, h.Hostname}, "vendor": {prev.Vendor, h.Vendor},
			"model": {prev.Model, h.Model}, "os": {prev.OS, h.OS},
			"device_type": {string(prev.DeviceType), string(h.DeviceType)},
			"status":      {string(prev.Status), string(h.Status)},
			"confidence":  {prev.Confidence, h.Confidence},
			"gateway":     {prev.IsGateway, h.IsGateway}, "favorite": {prev.Favorite, h.Favorite},
			"notes": {prev.Notes, h.Notes}, "tags": {prev.Tags, h.Tags},
		})
		diffPorts(out, key, label, prev.Ports, h.Ports)
	}
	for key, h := range hostsA {
		if _, ok := hostsB[key]; !ok {
			add(out, model.AuditChange{Type: "host.removed", Entity: "host", ID: key,
				Label: hostLabel(h), From: string(h.Status),
				Evidence: []string{fmt.Sprintf("presente en %s", out.FromName)}})
		}
	}

	diffEntities(out, "network", a.Networks, b.Networks,
		func(n model.AuditNetwork) string { return strconv.FormatInt(n.ID, 10) },
		func(n model.AuditNetwork) string {
			if n.Name != "" {
				return n.Name + " (" + n.CIDR + ")"
			}
			return n.CIDR
		},
		func(x, y model.AuditNetwork) map[string]fieldPair {
			return map[string]fieldPair{
				"name": {x.Name, y.Name}, "cidr": {x.CIDR, y.CIDR},
				"interface": {x.Interface, y.Interface}, "enabled": {x.Enabled, y.Enabled},
				"local": {x.IsLocal, y.IsLocal},
			}
		})

	diffEntities(out, "neighbor", a.Neighbors, b.Neighbors,
		func(n model.AuditNeighbor) string { return n.HostIdentity + "|" + neighborKey(n) },
		func(n model.AuditNeighbor) string {
			return n.Protocol + " " + firstNonEmpty(n.RemoteName, n.RemoteIP, n.RemoteMAC)
		},
		func(x, y model.AuditNeighbor) map[string]fieldPair {
			return map[string]fieldPair{
				"remote_name":    {x.RemoteName, y.RemoteName},
				"remote_chassis": {x.RemoteChassis, y.RemoteChassis},
				"confidence":     {x.Confidence, y.Confidence},
			}
		})

	diffEntities(out, "rule", a.Rules, b.Rules,
		func(r model.AuditRule) string { return strconv.FormatInt(r.ID, 10) },
		func(r model.AuditRule) string { return r.Name },
		func(x, y model.AuditRule) map[string]fieldPair {
			return map[string]fieldPair{
				"name": {x.Name, y.Name}, "type": {x.Type, y.Type},
				"enabled": {x.Enabled, y.Enabled}, "severity": {x.Severity, y.Severity},
				"channels": {x.ChannelIDs, y.ChannelIDs}, "cooldown": {x.CooldownSec, y.CooldownSec},
				"builtin": {x.Builtin, y.Builtin}, "config_keys": {x.ConfigKeys, y.ConfigKeys},
			}
		})

	diffEntities(out, "channel", a.Channels, b.Channels,
		func(c model.AuditChannel) string { return strconv.FormatInt(c.ID, 10) },
		func(c model.AuditChannel) string { return c.Name },
		func(x, y model.AuditChannel) map[string]fieldPair {
			return map[string]fieldPair{
				"name": {x.Name, y.Name}, "type": {x.Type, y.Type},
				"enabled": {x.Enabled, y.Enabled}, "secret": {x.HasSecret, y.HasSecret},
				"config_keys": {x.ConfigKeys, y.ConfigKeys},
			}
		})

	diffEntities(out, "integration", a.Integrations, b.Integrations,
		func(it model.AuditIntegration) string { return strconv.FormatInt(it.ID, 10) },
		func(it model.AuditIntegration) string { return it.Name },
		func(x, y model.AuditIntegration) map[string]fieldPair {
			return map[string]fieldPair{
				"name": {x.Name, y.Name}, "type": {x.Type, y.Type},
				"enabled": {x.Enabled, y.Enabled}, "secret": {x.HasSecret, y.HasSecret},
				"config_keys": {x.ConfigKeys, y.ConfigKeys},
			}
		})

	diffEntities(out, "scan", a.Scans, b.Scans,
		func(sc model.AuditScan) string { return strconv.FormatInt(sc.ID, 10) },
		func(sc model.AuditScan) string { return sc.CIDR },
		func(x, y model.AuditScan) map[string]fieldPair {
			return map[string]fieldPair{
				"cidr": {x.CIDR, y.CIDR}, "profile": {x.Profile, y.Profile},
				"status": {x.Status, y.Status}, "hosts_found": {x.HostsFound, y.HostsFound},
			}
		})

	diffTopology(out, a.Topology, b.Topology)
	diffFields(out, "alerts", "summary", "Resumen de alertas", map[string]fieldPair{
		"active":   {a.Alerts.Active, b.Alerts.Active},
		"critical": {a.Alerts.Critical, b.Alerts.Critical},
		"warnings": {a.Alerts.Warnings, b.Alerts.Warnings},
	})
	diffFields(out, "config", "summary", "Configuración", map[string]fieldPair{
		"monitor_enabled":      {a.Config.MonitorEnabled, b.Config.MonitorEnabled},
		"monitor_interval":     {a.Config.MonitorInterval, b.Config.MonitorInterval},
		"enterprise_mode":      {a.Config.EnterpriseMode, b.Config.EnterpriseMode},
		"alerts_enabled":       {a.Config.AlertsEnabled, b.Config.AlertsEnabled},
		"integrations_enabled": {a.Config.IntegrationsEnabled, b.Config.IntegrationsEnabled},
		"metrics_days":         {a.Config.MetricsDays, b.Config.MetricsDays},
		"retention_days":       {a.Config.RetentionDays, b.Config.RetentionDays},
	})

	sort.SliceStable(out.Changes, func(i, j int) bool {
		x, y := out.Changes[i], out.Changes[j]
		if x.Type != y.Type {
			return x.Type < y.Type
		}
		if x.Entity != y.Entity {
			return x.Entity < y.Entity
		}
		if x.ID != y.ID {
			return x.ID < y.ID
		}
		return x.Field < y.Field
	})
	if len(out.Changes) > MaxAuditChanges {
		out.Changes = out.Changes[:MaxAuditChanges]
		out.Truncated = true
	}
	out.Summary["total"] = len(out.Changes)
	for _, c := range out.Changes {
		out.Summary[c.Type]++
	}
	return out
}

func diffEntities[T any](out *model.AuditDiff, entity string, from, to []T,
	keyFn func(T) string, labelFn func(T) string, fieldsFn func(T, T) map[string]fieldPair) {
	a, b := map[string]T{}, map[string]T{}
	for _, item := range from {
		a[keyFn(item)] = item
	}
	for _, item := range to {
		b[keyFn(item)] = item
	}
	for key, item := range b {
		prev, ok := a[key]
		if !ok {
			add(out, model.AuditChange{Type: entity + ".added", Entity: entity, ID: key,
				Label: labelFn(item), To: labelFn(item),
				Evidence: []string{fmt.Sprintf("presente en %s", out.ToName)}})
			continue
		}
		diffFields(out, entity, key, labelFn(item), fieldsFn(prev, item))
	}
	for key, item := range a {
		if _, ok := b[key]; !ok {
			add(out, model.AuditChange{Type: entity + ".removed", Entity: entity, ID: key,
				Label: labelFn(item), From: labelFn(item),
				Evidence: []string{fmt.Sprintf("presente en %s", out.FromName)}})
		}
	}
}

func diffFields(out *model.AuditDiff, entity, id, label string, fields map[string]fieldPair) {
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		p := fields[k]
		if sameValue(p.from, p.to) {
			continue
		}
		add(out, model.AuditChange{
			Type: entity + ".changed", Entity: entity, ID: id, Label: label, Field: k,
			From: p.from, To: p.to,
			Evidence: []string{fmt.Sprintf("%s: %s → %s", k, display(p.from), display(p.to))},
		})
	}
}

func diffPorts(out *model.AuditDiff, hostID, label string, from, to []model.Port) {
	a, b := map[string]model.Port{}, map[string]model.Port{}
	for _, p := range from {
		a[portKey(p)] = p
	}
	for _, p := range to {
		b[portKey(p)] = p
	}
	for key, p := range b {
		id := hostID + "/" + key
		prev, ok := a[key]
		if !ok {
			add(out, model.AuditChange{Type: "host.port_added", Entity: "host", ID: id,
				Label: label, Field: "port", To: portLabel(p),
				Evidence: []string{fmt.Sprintf("puerto %s abierto en %s", key, out.ToName)}})
			continue
		}
		if portLabel(prev) != portLabel(p) {
			add(out, model.AuditChange{Type: "host.port_changed", Entity: "host", ID: id,
				Label: label, Field: "port", From: portLabel(prev), To: portLabel(p),
				Evidence: []string{fmt.Sprintf("puerto %s: %s → %s", key, portLabel(prev), portLabel(p))}})
		}
	}
	for key, p := range a {
		if _, ok := b[key]; !ok {
			add(out, model.AuditChange{Type: "host.port_removed", Entity: "host", ID: hostID + "/" + key,
				Label: label, Field: "port", From: portLabel(p),
				Evidence: []string{fmt.Sprintf("puerto %s cerrado en %s", key, out.ToName)}})
		}
	}
}

func diffTopology(out *model.AuditDiff, from, to model.AuditTopologyData) {
	nodesA, nodesB := map[string]model.GraphNode{}, map[string]model.GraphNode{}
	for _, n := range from.Nodes {
		nodesA[n.ID] = n
	}
	for _, n := range to.Nodes {
		nodesB[n.ID] = n
	}
	for id, n := range nodesB {
		prev, ok := nodesA[id]
		if !ok {
			add(out, model.AuditChange{Type: "topology.node_added", Entity: "topology", ID: id,
				Label:    n.Label,
				Evidence: []string{fmt.Sprintf("nodo presente en %s", out.ToName)}})
			continue
		}
		before := len(out.Changes)
		diffFields(out, "topology", id, firstNonEmpty(n.Label, id), map[string]fieldPair{
			"kind": {prev.Kind, n.Kind}, "label": {prev.Label, n.Label},
			"ip": {prev.IP, n.IP}, "status": {string(prev.Status), string(n.Status)},
			"device_type": {string(prev.DeviceType), string(n.DeviceType)},
			"confidence":  {prev.Confidence, n.Confidence},
		})
		for i := before; i < len(out.Changes); i++ {
			out.Changes[i].Type = "topology.node_changed"
		}
	}
	for id, n := range nodesA {
		if _, ok := nodesB[id]; !ok {
			add(out, model.AuditChange{Type: "topology.node_removed", Entity: "topology", ID: id,
				Label:    n.Label,
				Evidence: []string{fmt.Sprintf("nodo ausente en %s", out.ToName)}})
		}
	}

	edgesA, edgesB := map[string]model.GraphEdge{}, map[string]model.GraphEdge{}
	for _, e := range from.Edges {
		edgesA[e.ID] = e
	}
	for _, e := range to.Edges {
		edgesB[e.ID] = e
	}
	for id, e := range edgesB {
		prev, ok := edgesA[id]
		if !ok {
			add(out, model.AuditChange{Type: "topology.edge_added", Entity: "topology", ID: id,
				Label:    e.Label,
				Evidence: []string{fmt.Sprintf("enlace presente en %s", out.ToName)}})
			continue
		}
		before := len(out.Changes)
		diffFields(out, "topology", id, firstNonEmpty(e.Label, id), map[string]fieldPair{
			"source": {prev.Source, e.Source}, "target": {prev.Target, e.Target},
			"kind": {prev.Kind, e.Kind}, "label": {prev.Label, e.Label},
			"confidence": {prev.Confidence, e.Confidence},
		})
		for i := before; i < len(out.Changes); i++ {
			out.Changes[i].Type = "topology.edge_changed"
		}
	}
	for id, e := range edgesA {
		if _, ok := edgesB[id]; !ok {
			add(out, model.AuditChange{Type: "topology.edge_removed", Entity: "topology", ID: id,
				Label:    e.Label,
				Evidence: []string{fmt.Sprintf("enlace ausente en %s", out.ToName)}})
		}
	}
}

func add(out *model.AuditDiff, c model.AuditChange) {
	if c.Evidence == nil {
		c.Evidence = []string{}
	}
	out.Changes = append(out.Changes, c)
}

func hostKey(h model.AuditHost) string {
	if h.Identity != "" {
		return h.Identity
	}
	if h.MAC != "" {
		return "mac:" + strings.ToLower(h.MAC)
	}
	return "ip:" + h.IP
}

func hostLabel(h model.AuditHost) string {
	if h.Hostname != "" {
		return h.Hostname + " (" + h.IP + ")"
	}
	return h.IP
}

func portKey(p model.Port) string {
	return p.Protocol + "/" + strconv.Itoa(p.Number)
}

func portLabel(p model.Port) string {
	parts := []string{p.State}
	for _, v := range []string{p.Service, p.Product, p.Version} {
		if v != "" {
			parts = append(parts, v)
		}
	}
	return strings.Join(parts, " ")
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func sameValue(a, b any) bool { return display(a) == display(b) }

func display(v any) string {
	switch x := v.(type) {
	case nil:
		return "—"
	case string:
		return x
	case bool:
		return strconv.FormatBool(x)
	default:
		return fmt.Sprint(v)
	}
}
