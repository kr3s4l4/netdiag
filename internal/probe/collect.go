package probe

import (
	"context"
	"net"
	"os"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/google/gopacket"
	"github.com/kr3s4l4/netdiag/internal/config"
	"github.com/kr3s4l4/netdiag/internal/model"
	"github.com/kr3s4l4/netdiag/internal/netutil"
	"github.com/kr3s4l4/netdiag/internal/snmp"
	"github.com/kr3s4l4/netdiag/internal/vpn"
	"github.com/kr3s4l4/netdiag/internal/wifi"
)

// Collect gathers local observations. Only ARP is always available; SNMP,
// WiFi and VPN degrade to an explicit available:false status.
func Collect(ctx context.Context, cfg *config.Config, siteID int64) (*Document, error) {
	if cfg == nil {
		return nil, context.Canceled
	}
	d := New(siteID)
	d.Host.Hostname, _ = os.Hostname()
	d.Host.OS = runtime.GOOS

	ifaces, err := netutil.Interfaces()
	if err != nil {
		d.Sources.ARP.Note = "no se pudieron leer las interfaces: " + err.Error()
	} else {
		d.Host.Interfaces = ifaces
		primary := ""
		for _, ifc := range ifaces {
			if ifc.IsPrimary {
				primary = ifc.Name
				break
			}
		}
		gateway := netutil.GatewayFor(primary, ifaces)
		if gateway == "" {
			for _, ifc := range ifaces {
				if ifc.Gateway != "" {
					gateway = ifc.Gateway
					break
				}
			}
		}
		d.Host.Gateway = gateway
	}
	if networks, nerr := netutil.SuggestNetworks(); nerr == nil {
		d.Host.Networks = networks
	}

	collectARP(d)
	collectSNMP(ctx, d, cfg)
	collectWiFi(ctx, d, cfg)
	collectVPN(d)
	sortDocument(d)
	return d, nil
}

func collectARP(d *Document) {
	now := time.Now().UTC()
	gateway := strings.TrimSpace(d.Host.Gateway)
	table := netutil.ARPTable()
	if len(table) == 0 {
		d.Sources.ARP = SourceStatus{Available: false, Note: "tabla ARP del kernel vacía o no legible"}
		return
	}
	for ip, mac := range table {
		mac = strings.ToLower(strings.TrimSpace(mac))
		if _, err := net.ParseMAC(mac); err != nil {
			continue
		}
		d.ARP = append(d.ARP, model.ARPObservation{
			SiteID:     d.SiteID,
			IP:         strings.TrimSpace(ip),
			MAC:        mac,
			IsGateway:  strings.TrimSpace(ip) == gateway,
			ObservedAt: now,
		})
	}
	d.Sources.ARP = SourceStatus{Available: len(d.ARP) > 0, Note: "tabla ARP del kernel; sin raw sockets"}
}

func collectSNMP(ctx context.Context, d *Document, cfg *config.Config) {
	scan := cfg.Scan
	collector := snmp.New(snmp.Config{
		Communities: append([]string(nil), scan.SNMPCommunities...),
		Version:     scan.SNMPVersion,
		Timeout:     scan.SNMPTimeout.Duration(),
		Retries:     1,
		Deep:        cfg.Diagnostics.SNMPDeep,
		V3User:      scan.SNMPv3User,
		V3AuthPass:  scan.SNMPv3Auth,
		V3PrivPass:  scan.SNMPv3Priv,
		V3AuthProto: scan.SNMPv3AuthProto,
		V3PrivProto: scan.SNMPv3PrivProto,
	})
	if !collector.Enabled() {
		d.Sources.SNMP = SourceStatus{Available: false, Note: "SNMP desactivado en la configuración"}
		return
	}
	target := strings.TrimSpace(d.Host.Gateway)
	if target == "" {
		d.Sources.SNMP = SourceStatus{Available: false, Note: "sin gateway local para consultar LLDP/CDP"}
		return
	}
	sum, err := collector.Collect(ctx, target)
	if err != nil || sum == nil {
		note := "sondeo SNMP sin respuesta"
		if err != nil {
			note += ": " + err.Error()
		}
		d.Sources.SNMP = SourceStatus{Available: false, Note: note}
		return
	}
	d.SNMP = append(d.SNMP, SNMPRecord{HostIP: target, Data: *sum})
	d.Neighbors = append(d.Neighbors, sum.Neighbors...)
	if cfg.Diagnostics.SNMPDeep {
		d.SNMPDeep = append(d.SNMPDeep, SNMPDeepRecord{
			HostIP: target,
			Data: model.SNMPDeepSnapshot{
				IPAddresses: sum.Deep.IPAddresses,
				ARP:         sum.Deep.ARP,
				VLANs:       sum.Deep.VLANs,
				FDB:         sum.Deep.FDB,
			},
		})
	}
	d.Sources.SNMP = SourceStatus{Available: true, Note: "LLDP/CDP leídos por SNMP desde el gateway local"}
}

func collectWiFi(ctx context.Context, d *Document, cfg *config.Config) {
	d.WiFi = WiFiData{APs: []model.WifiAP{}, Clients: []model.WifiClient{}}
	if !wifi.Available() {
		d.Sources.WiFi = SourceStatus{Available: false, Note: "compilado sin -tags pcap"}
		return
	}
	iface := strings.TrimSpace(cfg.WiFi.Interface)
	if iface == "" {
		devs, err := wifi.Interfaces()
		if err == nil && len(devs) > 0 {
			iface = devs[0]
		}
	}
	if iface == "" {
		d.Sources.WiFi = SourceStatus{Available: false, Note: "sin interfaz en modo monitor"}
		return
	}
	duration := time.Duration(cfg.WiFi.ScanSeconds) * time.Second
	if duration < time.Second {
		duration = 2 * time.Second
	}
	aps := map[string]*model.WifiAP{}
	clients := map[string]*model.WifiClient{}
	err := wifi.Capture(ctx, iface, duration, cfg.WiFi.Channels, func(pkt gopacket.Packet) {
		apObs, clientObs := wifi.ParsePacket(pkt)
		if apObs != nil && apObs.BSSID != "" {
			ap, ok := aps[apObs.BSSID]
			if !ok {
				ap = &model.WifiAP{SiteID: d.SiteID, BSSID: apObs.BSSID}
				aps[apObs.BSSID] = ap
			}
			if apObs.SSID != "(oculta)" && (ap.SSID == "" || ap.SSID == "(oculta)") {
				ap.SSID = apObs.SSID
			}
			if apObs.SignalDBM > ap.SignalDBM {
				ap.SignalDBM = apObs.SignalDBM
			}
			ap.Channel = apObs.Channel
			ap.Band = apObs.Band
			ap.Encryption = apObs.Encryption
			ap.Beacons += apObs.Beacons
		}
		if clientObs != nil && clientObs.MAC != "" {
			client, ok := clients[clientObs.MAC]
			if !ok {
				client = &model.WifiClient{SiteID: d.SiteID, MAC: clientObs.MAC}
				clients[clientObs.MAC] = client
			}
			if clientObs.SignalDBM > client.SignalDBM {
				client.SignalDBM = clientObs.SignalDBM
			}
			client.APBSSID = clientObs.APBSSID
		}
	})
	if err != nil {
		d.Sources.WiFi = SourceStatus{Available: false, Note: "captura no disponible: " + err.Error()}
		return
	}
	now := time.Now().UTC()
	perAP := map[string]int{}
	for _, c := range clients {
		perAP[c.APBSSID]++
		c.LastSeen = now
		d.WiFi.Clients = append(d.WiFi.Clients, *c)
	}
	for _, ap := range aps {
		ap.Clients = perAP[ap.BSSID]
		ap.FirstSeen = now
		ap.LastSeen = now
		d.WiFi.APs = append(d.WiFi.APs, *ap)
	}
	d.Sources.WiFi = SourceStatus{Available: true, Note: "captura pasiva sin payloads; requiere modo monitor"}
}

func collectVPN(d *Document) {
	d.VPN = VPNData{Tunnels: []model.VPNTunnel{}}
	now := time.Now().UTC()
	detector := vpn.Detector{}
	for _, o := range detector.Observe() {
		d.VPN.Tunnels = append(d.VPN.Tunnels, model.VPNTunnel{
			Type:       o.Type,
			LocalIP:    o.LocalIP,
			LocalPort:  o.LocalPort,
			RemoteIP:   o.RemoteIP,
			RemotePort: o.RemotePort,
			BytesSent:  o.BytesSent,
			BytesRecv:  o.BytesRecv,
			State:      o.State,
			Evidence:   o.Evidence,
			Confidence: o.Confidence,
			FirstSeen:  now,
			LastSeen:   now,
		})
	}
	d.Sources.VPN = SourceStatus{Available: true, Note: "lectura pasiva de tablas del kernel; sin privilegios"}
}

func sortDocument(d *Document) {
	sort.Slice(d.ARP, func(i, j int) bool {
		if d.ARP[i].IP == d.ARP[j].IP {
			return d.ARP[i].MAC < d.ARP[j].MAC
		}
		return d.ARP[i].IP < d.ARP[j].IP
	})
	sort.Slice(d.Neighbors, func(i, j int) bool {
		a, b := d.Neighbors[i], d.Neighbors[j]
		if a.Protocol != b.Protocol {
			return a.Protocol < b.Protocol
		}
		if a.RemoteMAC != b.RemoteMAC {
			return a.RemoteMAC < b.RemoteMAC
		}
		return a.RemoteIP < b.RemoteIP
	})
	sort.Slice(d.WiFi.APs, func(i, j int) bool { return d.WiFi.APs[i].BSSID < d.WiFi.APs[j].BSSID })
	sort.Slice(d.WiFi.Clients, func(i, j int) bool { return d.WiFi.Clients[i].MAC < d.WiFi.Clients[j].MAC })
	sort.Slice(d.VPN.Tunnels, func(i, j int) bool {
		a, b := d.VPN.Tunnels[i], d.VPN.Tunnels[j]
		if a.Type != b.Type {
			return a.Type < b.Type
		}
		return a.LocalIP < b.LocalIP
	})
}
