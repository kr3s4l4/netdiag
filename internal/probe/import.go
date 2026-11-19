package probe

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/kr3s4l4/netdiag/internal/model"
	"github.com/kr3s4l4/netdiag/internal/store"
)

// ImportReport counts the rows written by one manual probe import.
type ImportReport struct {
	ARP         int `json:"arp"`
	Neighbors   int `json:"neighbors"`
	SNMP        int `json:"snmp"`
	SNMPDeep    int `json:"snmp_deep"`
	WiFiAPs     int `json:"wifi_aps"`
	WiFiClients int `json:"wifi_clients"`
	VPN         int `json:"vpn"`
}

// Import writes probe observations into the local store. It is called by the
// CLI only after the optional signature has been verified. Nothing is sent to
// any remote service and no alert is closed automatically.
func Import(st *store.Store, d *Document) (*ImportReport, error) {
	if st == nil || d == nil {
		return nil, errors.New("sonda o almacén vacío")
	}
	if err := d.Validate(); err != nil {
		return nil, err
	}
	if _, err := st.GetSite(d.SiteID); err != nil {
		return nil, fmt.Errorf("la sede %d no existe: %w", d.SiteID, err)
	}
	report := &ImportReport{}
	now := time.Now().UTC()

	arp := make([]model.ARPObservation, 0, len(d.ARP))
	for _, o := range d.ARP {
		if strings.TrimSpace(o.IP) == "" || strings.TrimSpace(o.MAC) == "" {
			continue
		}
		o.SiteID = d.SiteID
		if o.ObservedAt.IsZero() {
			o.ObservedAt = now
		}
		arp = append(arp, o)
	}
	if err := st.SaveARPObservations(arp); err != nil {
		return nil, fmt.Errorf("importar ARP: %w", err)
	}
	report.ARP = len(arp)

	for _, rec := range d.SNMP {
		hostIP := strings.TrimSpace(rec.HostIP)
		if hostIP == "" {
			continue
		}
		hostID, err := ensureHost(st, d.SiteID, hostIP)
		if err != nil {
			return nil, err
		}
		if err := st.ReplaceNeighbors(hostID, rec.Data.Neighbors); err != nil {
			return nil, fmt.Errorf("importar vecinos: %w", err)
		}
		report.Neighbors += len(rec.Data.Neighbors)
		report.SNMP++
	}

	for _, rec := range d.SNMPDeep {
		hostIP := strings.TrimSpace(rec.HostIP)
		if hostIP == "" {
			continue
		}
		deep := model.SNMPDeep{
			IPAddresses: rec.Data.IPAddresses,
			ARP:         rec.Data.ARP,
			VLANs:       rec.Data.VLANs,
			FDB:         rec.Data.FDB,
		}
		if err := st.SaveSNMPDeep(d.SiteID, 0, hostIP, "", now, deep); err != nil {
			return nil, fmt.Errorf("importar SNMP profundo: %w", err)
		}
		report.SNMPDeep++
	}

	for i := range d.WiFi.APs {
		ap := d.WiFi.APs[i]
		if strings.TrimSpace(ap.BSSID) == "" {
			continue
		}
		ap.SiteID = d.SiteID
		if ap.FirstSeen.IsZero() {
			ap.FirstSeen = now
		}
		if ap.LastSeen.IsZero() {
			ap.LastSeen = now
		}
		if err := st.UpsertWifiAP(&ap); err != nil {
			return nil, fmt.Errorf("importar AP WiFi: %w", err)
		}
		report.WiFiAPs++
	}
	for i := range d.WiFi.Clients {
		client := d.WiFi.Clients[i]
		if strings.TrimSpace(client.MAC) == "" {
			continue
		}
		client.SiteID = d.SiteID
		if client.LastSeen.IsZero() {
			client.LastSeen = now
		}
		if err := st.UpsertWifiClient(&client); err != nil {
			return nil, fmt.Errorf("importar cliente WiFi: %w", err)
		}
		report.WiFiClients++
	}

	for i := range d.VPN.Tunnels {
		tunnel := d.VPN.Tunnels[i]
		tunnel.SiteID = d.SiteID
		tunnel.Identity = ""
		if tunnel.FirstSeen.IsZero() {
			tunnel.FirstSeen = now
		}
		if tunnel.LastSeen.IsZero() {
			tunnel.LastSeen = now
		}
		if err := st.UpsertVPNTunnel(&tunnel); err != nil {
			return nil, fmt.Errorf("importar túnel VPN: %w", err)
		}
		report.VPN++
	}
	return report, nil
}

func ensureHost(st *store.Store, siteID int64, ip string) (int64, error) {
	host, err := st.GetHostByIP(ip)
	if err == nil {
		return host.ID, nil
	}
	if !errors.Is(err, store.ErrNotFound) {
		return 0, fmt.Errorf("buscar host %s: %w", ip, err)
	}
	h := &model.Host{IP: ip, SiteID: siteID}
	id, _, _, _, err := st.UpsertHost(h)
	if err != nil {
		return 0, fmt.Errorf("crear host %s: %w", ip, err)
	}
	return id, nil
}
