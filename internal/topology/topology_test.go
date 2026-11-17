package topology

import (
	"testing"

	"github.com/kr3s4l4/netdiag/internal/model"
)

func TestBuildGraph(t *testing.T) {
	networks := []model.Network{
		{ID: 1, Name: "LAN", CIDR: "192.168.1.0/24"},
		{ID: 2, Name: "DMZ", CIDR: "10.10.0.0/24"},
	}
	hosts := []model.Host{
		{ID: 1, IP: "192.168.1.1", MAC: "aa:bb:cc:00:00:01", Hostname: "gw", DeviceType: model.DevRouter, IsGateway: true, NetworkID: 1, Status: model.StatusUp},
		{ID: 2, IP: "192.168.1.20", MAC: "aa:bb:cc:00:00:02", Hostname: "pc-1", DeviceType: model.DevWorkstation, NetworkID: 1, Status: model.StatusUp},
		{ID: 3, IP: "10.10.0.5", MAC: "aa:bb:cc:00:00:03", Hostname: "srv", DeviceType: model.DevServer, NetworkID: 2, Status: model.StatusUp},
		{ID: 4, IP: "10.10.0.6", MAC: "aa:bb:cc:00:00:04", DeviceType: model.DevUnknown, NetworkID: 2, Status: model.StatusDown},
	}
	neighbors := map[int64][]model.Neighbor{
		1: {
			{Protocol: "lldp", RemoteName: "pc-1", LocalPort: "Gi0/1", RemotePort: "eth0", Confidence: 90},
			{Protocol: "lldp", RemoteIP: "10.10.0.5", Confidence: 80},
			{Protocol: "arp", RemoteMAC: "aa:bb:cc:00:00:01", Confidence: 100}, // self, debe ignorarse
		},
	}
	g := Build(hosts, neighbors, networks)

	if g.Stats.Nodes != 6 {
		t.Fatalf("nodos inesperados: %d (%+v)", g.Stats.Nodes, g.Nodes)
	}
	if g.Stats.Edges == 0 {
		t.Fatal("se esperaban enlaces")
	}
	if g.Stats.ByType[string(model.DevRouter)] != 1 || g.Stats.ByType[string(model.DevServer)] != 1 {
		t.Fatalf("resumen por tipo inesperado: %+v", g.Stats.ByType)
	}

	ids := map[string]model.GraphNode{}
	for _, n := range g.Nodes {
		ids[n.ID] = n
	}
	gw, ok := ids["net:1"]
	if !ok {
		t.Fatalf("falta el nodo de red/gateway: %+v", ids)
	}
	if !gw.IsGateway || gw.Kind != "gateway" || gw.HostID != 1 {
		t.Fatalf("el ancla de la red no es la puerta de enlace: %+v", gw)
	}
	if _, ok := ids["net:2"]; !ok {
		t.Fatal("falta el ancla de la DMZ")
	}

	kinds := map[string]int{}
	for _, e := range g.Edges {
		if e.Source == e.Target {
			t.Fatalf("enlace reflexivo: %+v", e)
		}
		kinds[e.Kind]++
	}
	if kinds["subnet"] == 0 {
		t.Fatal("faltan enlaces de subred")
	}
	if kinds["lldp"] == 0 {
		t.Fatal("faltan enlaces LLDP")
	}
	if kinds["arp"] != 0 {
		t.Fatal("un vecino de sí mismo no debe generar enlace ARP")
	}
}

func TestBuildEmpty(t *testing.T) {
	g := Build(nil, nil, nil)
	if len(g.Nodes) != 0 || len(g.Edges) != 0 {
		t.Fatalf("grafo vacío inesperado: %+v", g)
	}
	if g.Stats.ByType == nil {
		t.Fatal("ByType debe inicializarse")
	}
}
