package history

import (
	"time"

	"github.com/kr3s4l4/netdiag/internal/model"
	"github.com/kr3s4l4/netdiag/internal/store"
	"github.com/kr3s4l4/netdiag/internal/topology"
)

// Capture persists a compact view of the current inventory and topology.
// Snapshots are taken after scans and periodically so a temporal diff can
// compare moments even when only the current inventory is retained.
func Capture(st *store.Store, siteID int64) error {
	if siteID <= 0 {
		siteID = 1
	}
	hosts, err := st.ListAllHostsForSite(siteID, 20000)
	if err != nil {
		return err
	}
	neighbors, err := st.AllNeighbors()
	if err != nil {
		neighbors = map[int64][]model.Neighbor{}
	}
	var networks []model.Network
	if siteID > 0 {
		networks, err = st.ListNetworksBySite(siteID)
	} else {
		networks, err = st.ListNetworks()
	}
	if err != nil {
		return err
	}
	graph := topology.Build(hosts, neighbors, networks)

	nodeIDs := make([]string, 0, len(graph.Nodes))
	for _, n := range graph.Nodes {
		nodeIDs = append(nodeIDs, n.ID)
	}
	edgeIDs := make([]string, 0, len(graph.Edges))
	for _, e := range graph.Edges {
		edgeIDs = append(edgeIDs, e.ID)
	}
	snap := &model.HistorySnapshot{
		SiteID: siteID,
		TS:     time.Now(),
		Hosts:  make([]model.HistoryHost, 0, len(hosts)),
		Topology: model.HistoryTopology{
			Nodes: len(graph.Nodes), Edges: len(graph.Edges),
			ByType: graph.Stats.ByType, NodeIDs: nodeIDs, EdgeIDs: edgeIDs,
			Fingerprint: Fingerprint(nodeIDs, edgeIDs),
		},
	}
	for _, h := range hosts {
		snap.Hosts = append(snap.Hosts, model.HistoryHost{
			ID: h.ID, IP: h.IP, Hostname: h.Hostname, MAC: h.MAC,
			DeviceType: h.DeviceType, Status: h.Status,
			FirstSeen: h.FirstSeen, LastSeen: h.LastSeen,
		})
	}
	return st.SaveHistorySnapshot(snap)
}

// BuildReport collects both periods from storage and compares them.
func BuildReport(st *store.Store, siteID int64, a, b model.HistoryPeriod) (*model.DiffReport, error) {
	pa, err := st.BuildHistoryPeriod(siteID, a)
	if err != nil {
		return nil, err
	}
	pb, err := st.BuildHistoryPeriod(siteID, b)
	if err != nil {
		return nil, err
	}
	report := Compare(pa, pb)
	return &report, nil
}
