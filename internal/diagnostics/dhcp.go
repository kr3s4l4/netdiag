package diagnostics

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/kr3s4l4/netdiag/internal/config"
	"github.com/kr3s4l4/netdiag/internal/model"
)

type dhcpBatch struct {
	key  string
	at   time.Time
	rows []model.DHCPObservation
}

type gatewayBinding struct {
	mac string
	at  time.Time
}

// DetectDHCP turns raw DHCP replies into security findings. It works with the
// unprivileged UDP probe only: a server is trusted when it is explicitly
// authorized or when its IP and MAC match the gateway learned from ARP.
func DetectDHCP(obs []model.DHCPObservation, arp []model.ARPObservation, cfg config.Diagnostics) []model.DiagnosticAnomaly {
	out := []model.DiagnosticAnomaly{}
	valid := make([]model.DHCPObservation, 0, len(obs))
	for _, o := range obs {
		o.IP = strings.TrimSpace(o.IP)
		o.ServerID = strings.ToLower(strings.TrimSpace(o.ServerID))
		o.MAC = strings.ToLower(strings.TrimSpace(o.MAC))
		o.MessageType = strings.ToLower(strings.TrimSpace(o.MessageType))
		if o.ServerID == "" {
			o.ServerID = o.IP
		}
		if o.IP == "" {
			continue
		}
		valid = append(valid, o)
	}
	if len(valid) == 0 {
		return out
	}

	minConf := cfg.MinConfidence
	if minConf < 1 {
		minConf = 60
	}
	maxOut := cfg.MaxAnomalies
	if maxOut < 1 {
		maxOut = 200
	}

	batches := map[string]*dhcpBatch{}
	for _, o := range valid {
		key := "scan:" + strconv.FormatInt(o.ScanID, 10)
		if o.ScanID <= 0 {
			key = "time:" + strconv.FormatInt(o.ObservedAt.Unix(), 10)
		}
		b := batches[key]
		if b == nil {
			b = &dhcpBatch{key: key, at: o.ObservedAt}
			batches[key] = b
		}
		if o.ObservedAt.After(b.at) {
			b.at = o.ObservedAt
		}
		b.rows = append(b.rows, o)
	}
	keys := make([]string, 0, len(batches))
	for k := range batches {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, b := batches[keys[i]], batches[keys[j]]
		if !a.at.Equal(b.at) {
			return a.at.Before(b.at)
		}
		return a.key < b.key
	})
	current := batches[keys[len(keys)-1]]

	authorized := map[string]bool{}
	for _, value := range cfg.AuthorizedDHCPServers {
		if value = strings.ToLower(strings.TrimSpace(value)); value != "" {
			authorized[value] = true
		}
	}
	gateways := map[string]gatewayBinding{}
	for _, o := range arp {
		ip := strings.TrimSpace(o.IP)
		if ip == "" || !o.IsGateway {
			continue
		}
		mac := strings.ToLower(strings.TrimSpace(o.MAC))
		prev := gateways[ip]
		if mac != "" && (prev.mac == "" || o.ObservedAt.After(prev.at)) {
			gateways[ip] = gatewayBinding{mac: mac, at: o.ObservedAt}
		}
	}

	added := map[string]bool{}
	add := func(kind, severity, title, detail, recommendation string, confidence int, ips, evidence []string) {
		if confidence < minConf || len(evidence) == 0 || added[kind+":"+evidence[0]] {
			return
		}
		added[kind+":"+evidence[0]] = true
		id := fmt.Sprintf("dhcp:%s:%s", kind, strings.Join(ips, ","))
		out = append(out, model.DiagnosticAnomaly{
			ID:             id,
			Type:           kind,
			Category:       "dhcp",
			Severity:       severity,
			Title:          title,
			Detail:         detail,
			HostIPs:        ips,
			Confidence:     confidence,
			Evidence:       evidence,
			Recommendation: recommendation,
			DetectedAt:     current.at,
		})
	}

	servers := map[string]bool{}
	byServer := map[string][]model.DHCPObservation{}
	for _, o := range current.rows {
		servers[o.ServerID] = true
		byServer[o.ServerID] = append(byServer[o.ServerID], o)
	}
	if len(servers) > 1 {
		ids := sortedKeys(servers)
		add("dhcp_multiple_servers", model.SeverityCritical, "Varios servidores DHCP responden en la red",
			fmt.Sprintf("El último sondeo obtuvo respuestas de %d servidores: %s.", len(ids), strings.Join(ids, ", ")),
			"Identifica el servidor DHCP legítimo y desactiva el otro antes de que reparta direcciones o rutas.",
			90, ids, []string{"Múltiples respuestas DHCP en el mismo escaneo", strings.Join(ids, ", ")})
	}

	for _, serverID := range sortedKeys(servers) {
		rows := byServer[serverID]
		macs := map[string]bool{}
		sourceIPs := map[string]bool{}
		nak := false
		for _, o := range rows {
			if o.MAC != "" {
				macs[o.MAC] = true
			}
			sourceIPs[o.IP] = true
			if o.MessageType == "nak" {
				nak = true
			}
		}
		macList := sortedKeys(macs)
		ips := append([]string{serverID}, sortedKeys(sourceIPs)...)
		ips = uniqueStrings(ips)
		if len(macList) > 1 {
			add("dhcp_server_mac_changed", model.SeverityCritical, "El servidor DHCP cambió de MAC",
				fmt.Sprintf("El servidor %s respondió desde las MAC %s en el mismo escaneo.", serverID, strings.Join(macList, ", ")),
				"Comprueba si hay suplantación del servidor DHCP o un relay con varias interfaces.",
				92, ips, []string{"Servidor " + serverID, "MACs " + strings.Join(macList, ", ")})
		}

		trusted := authorized[serverID]
		for _, mac := range macList {
			if authorized[mac] {
				trusted = true
			}
		}
		gw, isGateway := gateways[serverID]
		if isGateway {
			matches := false
			for _, mac := range macList {
				if mac == gw.mac {
					matches = true
				}
			}
			if len(macList) > 0 && !matches {
				add("dhcp_gateway_spoof", model.SeverityCritical, "Alguien suplanta al servidor DHCP del gateway",
					fmt.Sprintf("El gateway %s responde DHCP desde %s, pero su MAC ARP es %s.", serverID, strings.Join(macList, ", "), gw.mac),
					"Verifica la MAC del router por consola y busca otro equipo enviando OFFER/ACK en el segmento.",
					95, []string{serverID}, []string{"Gateway " + serverID, "MAC DHCP " + strings.Join(macList, ", "), "MAC ARP " + gw.mac})
				trusted = true
			} else if matches {
				trusted = true
			}
		}
		if trusted || (len(authorized) == 0 && len(gateways) == 0) {
			continue
		}
		severity, confidence := model.SeverityWarning, 80
		if len(authorized) > 0 {
			confidence = 85
		}
		if nak {
			severity, confidence = model.SeverityCritical, 90
		}
		add("dhcp_unauthorized_server", severity, "Servidor DHCP no autorizado",
			fmt.Sprintf("El servidor %s (%s) respondió DHCP sin estar en la lista de servidores autorizados ni coincidir con el gateway.",
				serverID, strings.Join(macList, ", ")),
			"Añade el servidor a diagnostics.authorized_dhcp_servers si es legítimo; si no, localiza el equipo por su MAC.",
			confidence, ips, []string{"Servidor " + serverID, "Fuentes " + strings.Join(sortedKeys(sourceIPs), ", "), "Sin coincidencia con gateway o lista autorizada"})
	}

	// A server changing its L2 address between scans is a stronger signal than
	// a single unknown server and does not need an authorized list.
	timeline := map[string][]gatewayBinding{}
	for _, key := range keys {
		b := batches[key]
		for _, o := range b.rows {
			if o.MAC == "" {
				continue
			}
			found := false
			for i := range timeline[o.ServerID] {
				if timeline[o.ServerID][i].mac == o.MAC {
					found = true
					break
				}
			}
			if !found {
				timeline[o.ServerID] = append(timeline[o.ServerID], gatewayBinding{mac: o.MAC, at: b.at})
			}
		}
	}
	for _, serverID := range sortedKeys(servers) {
		seen := timeline[serverID]
		if len(seen) < 2 {
			continue
		}
		macs := make([]string, 0, len(seen))
		for _, s := range seen {
			macs = append(macs, s.mac+" @ "+s.at.Format("2006-01-02 15:04"))
		}
		add("dhcp_server_mac_changed", model.SeverityCritical, "El servidor DHCP cambió de MAC",
			fmt.Sprintf("El servidor %s apareció con %d MAC en el histórico reciente.", serverID, len(seen)),
			"Investiga un posible DHCP rogue antes de renovar las direcciones de los clientes.",
			90, []string{serverID}, append([]string{"Servidor " + serverID}, macs...))
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Severity != out[j].Severity {
			return severityRank(out[i].Severity) > severityRank(out[j].Severity)
		}
		if out[i].Confidence != out[j].Confidence {
			return out[i].Confidence > out[j].Confidence
		}
		return out[i].ID < out[j].ID
	})
	if len(out) > maxOut {
		out = out[:maxOut]
	}
	return out
}

func uniqueStrings(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, v := range in {
		if v != "" && !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}
