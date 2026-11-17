// Package topology builds a rendered graph from inventory, SNMP neighbours and
// configured networks. The graph is intentionally explicit: every edge carries
// the evidence that produced it and a confidence value.
package topology

import (
	"fmt"
	"net"
	"sort"
	"strings"

	"github.com/kr3s4l4/netdiag/internal/model"
)

// Build assembles the topology graph.
//
// Rules:
//   - every host is a node;
//   - every network gets a logical node, shown as a gateway when a host is
//     marked as gateway;
//   - hosts are attached to their network node;
//   - LLDP/CDP/bridge neighbours create edges when the remote side can be
//     resolved to a known host (by IP, MAC or name). Unresolved neighbours are
//     not invented as nodes.
func Build(hosts []model.Host, neighbors map[int64][]model.Neighbor, networks []model.Network) model.Graph {
	g := model.Graph{Nodes: []model.GraphNode{}, Edges: []model.GraphEdge{}, Stats: model.GraphStats{ByType: map[string]int{}}}

	networkName := map[int64]string{}
	for _, n := range networks {
		networkName[n.ID] = n.Name
	}

	hostByID := map[int64]*model.Host{}
	hostByIP := map[string]*model.Host{}
	hostByMAC := map[string]*model.Host{}
	nameIndex := map[string]*model.Host{}
	for i := range hosts {
		h := &hosts[i]
		hostByID[h.ID] = h
		if h.IP != "" {
			hostByIP[h.IP] = h
		}
		if h.MAC != "" {
			hostByMAC[strings.ToLower(h.MAC)] = h
		}
		if h.Hostname != "" {
			nameIndex[strings.ToLower(h.Hostname)] = h
		}
		g.Stats.ByType[string(h.DeviceType)]++
	}

	gatewayByNetwork := map[int64]*model.Host{}
	var gateways []*model.Host
	for i := range hosts {
		h := &hosts[i]
		if !h.IsGateway {
			continue
		}
		gateways = append(gateways, h)
		if h.NetworkID != 0 {
			if _, ok := gatewayByNetwork[h.NetworkID]; !ok {
				gatewayByNetwork[h.NetworkID] = h
			}
		}
	}

	seenNodes := map[string]bool{}
	addNode := func(n model.GraphNode) {
		if seenNodes[n.ID] {
			return
		}
		seenNodes[n.ID] = true
		g.Nodes = append(g.Nodes, n)
	}

	// Network anchor nodes (gateway or logical subnet).
	networkNodeID := map[int64]string{}
	for _, nw := range networks {
		nodeID := fmt.Sprintf("net:%d", nw.ID)
		kind := "network"
		label := nw.Name
		if label == "" {
			label = nw.CIDR
		}
		var gw *model.Host
		if h, ok := gatewayByNetwork[nw.ID]; ok {
			gw = h
			kind = "gateway"
			if h.Hostname != "" {
				label = h.Hostname
			} else if h.IP != "" {
				label = h.IP
			}
		}
		node := model.GraphNode{
			ID:         nodeID,
			Kind:       kind,
			HostID:     0,
			Label:      label,
			IP:         nw.CIDR,
			NetworkID:  nw.ID,
			Network:    nw.CIDR,
			IsGateway:  gw != nil,
			DeviceType: model.DevRouter,
		}
		if gw != nil {
			node.HostID = gw.ID
			node.IP = gw.IP
			node.MAC = gw.MAC
			node.Vendor = gw.Vendor
			node.Status = gw.Status
			node.Confidence = gw.Confidence
		}
		addNode(node)
		networkNodeID[nw.ID] = nodeID
		g.Stats.Nodes++
	}

	// Host nodes.
	for i := range hosts {
		h := &hosts[i]
		id := hostNodeID(h.ID)
		label := h.Hostname
		if label == "" {
			label = h.IP
		}
		if label == "" {
			label = h.MAC
		}
		kind := "host"
		switch h.DeviceType {
		case model.DevSwitch:
			kind = "switch"
		case model.DevRouter:
			kind = "router"
		case model.DevFirewall:
			kind = "firewall"
		}
		addNode(model.GraphNode{
			ID:         id,
			Kind:       kind,
			HostID:     h.ID,
			Label:      label,
			IP:         h.IP,
			MAC:        h.MAC,
			Vendor:     h.Vendor,
			DeviceType: h.DeviceType,
			Status:     h.Status,
			IsGateway:  h.IsGateway,
			NetworkID:  h.NetworkID,
			Network:    networkName[h.NetworkID],
			Confidence: h.Confidence,
		})
		g.Stats.Nodes++
	}

	seenEdges := map[string]bool{}
	addEdge := func(e model.GraphEdge) {
		if e.Source == "" || e.Target == "" || e.Source == e.Target {
			return
		}
		key := e.Source + "->" + e.Target + ":" + e.Kind
		rev := e.Target + "->" + e.Source + ":" + e.Kind
		if seenEdges[key] || seenEdges[rev] {
			return
		}
		seenEdges[key] = true
		if e.Confidence <= 0 {
			e.Confidence = 50
		}
		g.Edges = append(g.Edges, e)
	}

	// Attach hosts to their network anchor.
	for i := range hosts {
		h := &hosts[i]
		if h.IsGateway {
			continue // the gateway is the anchor itself when present
		}
		anchor := networkNodeID[h.NetworkID]
		if anchor == "" && len(networks) == 1 {
			anchor = networkNodeID[networks[0].ID]
		}
		if anchor == "" {
			continue
		}
		addEdge(model.GraphEdge{
			ID:         edgeID(anchor, hostNodeID(h.ID), "subnet"),
			Source:     anchor,
			Target:     hostNodeID(h.ID),
			Kind:       "subnet",
			Label:      "subred",
			Confidence: 70,
		})
	}

	// SNMP / L2 adjacency.
	resolve := func(n model.Neighbor) *model.Host {
		if n.RemoteIP != "" {
			if h, ok := hostByIP[n.RemoteIP]; ok {
				return h
			}
		}
		if n.RemoteMAC != "" {
			if h, ok := hostByMAC[strings.ToLower(n.RemoteMAC)]; ok {
				return h
			}
		}
		if n.RemoteName != "" {
			if h, ok := nameIndex[strings.ToLower(n.RemoteName)]; ok {
				return h
			}
		}
		return nil
	}
	for localID, list := range neighbors {
		local, ok := hostByID[localID]
		if !ok {
			continue
		}
		for _, n := range list {
			remote := resolve(n)
			if remote == nil || remote.ID == localID {
				continue
			}
			src, dst := hostNodeID(local.ID), hostNodeID(remote.ID)
			if remote.IsGateway {
				src, dst = hostNodeID(local.ID), hostNodeID(remote.ID)
			}
			addEdge(model.GraphEdge{
				ID:         edgeID(src, dst, n.Protocol),
				Source:     src,
				Target:     dst,
				Kind:       n.Protocol,
				Label:      neighborLabel(n),
				Confidence: n.Confidence,
			})
		}
	}

	// Gateway links for hosts on networks without a discovered gateway host.
	for i := range hosts {
		h := &hosts[i]
		if h.IsGateway || h.NetworkID == 0 {
			continue
		}
		if _, ok := gatewayByNetwork[h.NetworkID]; ok {
			continue
		}
	}

	// Count isolated nodes (no edges).
	degree := map[string]int{}
	for _, e := range g.Edges {
		degree[e.Source]++
		degree[e.Target]++
	}
	for _, n := range g.Nodes {
		if degree[n.ID] == 0 {
			g.Stats.Isolated++
		}
	}
	g.Stats.Edges = len(g.Edges)
	g.Stats.Nodes = len(g.Nodes)

	sort.Slice(g.Nodes, func(i, j int) bool {
		if g.Nodes[i].Kind != g.Nodes[j].Kind {
			return g.Nodes[i].Kind < g.Nodes[j].Kind
		}
		if g.Nodes[i].IP != g.Nodes[j].IP {
			return ipSort(g.Nodes[i].IP) < ipSort(g.Nodes[j].IP)
		}
		return g.Nodes[i].ID < g.Nodes[j].ID
	})
	sort.Slice(g.Edges, func(i, j int) bool { return g.Edges[i].ID < g.Edges[j].ID })
	return g
}

func hostNodeID(id int64) string { return fmt.Sprintf("host:%d", id) }

func edgeID(a, b, kind string) string {
	if a > b {
		a, b = b, a
	}
	return a + "|" + b + "|" + kind
}

func neighborLabel(n model.Neighbor) string {
	parts := []string{}
	if n.LocalPort != "" {
		parts = append(parts, "local "+n.LocalPort)
	}
	if n.RemotePort != "" {
		parts = append(parts, "remoto "+n.RemotePort)
	}
	if n.RemoteName != "" && len(parts) == 0 {
		parts = append(parts, n.RemoteName)
	}
	return strings.Join(parts, " · ")
}

func ipSort(s string) uint32 {
	ip := net.ParseIP(s)
	if ip == nil {
		return 0
	}
	v4 := ip.To4()
	if v4 == nil {
		return 0
	}
	return uint32(v4[0])<<24 | uint32(v4[1])<<16 | uint32(v4[2])<<8 | uint32(v4[3])
}
