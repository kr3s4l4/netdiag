package diagnostics

import (
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/kr3s4l4/netdiag/internal/config"
	"github.com/kr3s4l4/netdiag/internal/model"
)

// arpSeen is one IP→MAC binding from an earlier scan batch.
type arpSeen struct {
	mac      string
	at       time.Time
	hostname string
	gateway  bool
}

// arpBatch groups the observations collected in one scan.
type arpBatch struct {
	key  string
	at   time.Time
	rows []model.ARPObservation
}

// DetectARP compares IP→MAC bindings across scan batches and returns findings
// backed only by inventory evidence. It never reads live frames, so it works
// without raw sockets, promiscuous mode or external binaries.
func DetectARP(obs []model.ARPObservation, cfg config.Diagnostics) []model.DiagnosticAnomaly {
	out := []model.DiagnosticAnomaly{}
	valid := make([]model.ARPObservation, 0, len(obs))
	for _, o := range obs {
		o.IP = strings.TrimSpace(o.IP)
		o.MAC = strings.ToLower(strings.TrimSpace(o.MAC))
		if o.IP == "" || !validARPMAC(o.MAC) {
			continue
		}
		valid = append(valid, o)
	}
	if len(valid) == 0 {
		return out
	}

	limit := cfg.MACIPLimit
	if limit < 2 {
		limit = 5
	}
	minConf := cfg.MinConfidence
	if minConf < 1 {
		minConf = 60
	}
	maxOut := cfg.MaxAnomalies
	if maxOut < 1 {
		maxOut = 200
	}

	batches := map[string]*arpBatch{}
	for _, o := range valid {
		key := "scan:" + strconv.FormatInt(o.ScanID, 10)
		if o.ScanID <= 0 {
			key = "time:" + strconv.FormatInt(o.ObservedAt.Unix(), 10)
		}
		b := batches[key]
		if b == nil {
			b = &arpBatch{key: key, at: o.ObservedAt}
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
	if len(keys) < 2 {
		return out
	}
	current := batches[keys[len(keys)-1]]
	history := map[string][]arpSeen{}
	for _, k := range keys[:len(keys)-1] {
		b := batches[k]
		for _, o := range b.rows {
			history[o.IP] = append(history[o.IP], arpSeen{
				mac: o.MAC, at: b.at, hostname: o.Hostname, gateway: o.IsGateway,
			})
		}
	}

	add := func(kind, severity, title, detail, recommendation string, confidence int, ips []string, evidence []string) {
		if confidence < minConf {
			return
		}
		id := fmt.Sprintf("arp:%s:%s", kind, strings.Join(ips, ","))
		if len(evidence) > 0 {
			id += ":" + evidence[0]
		}
		out = append(out, model.DiagnosticAnomaly{
			ID:             id,
			Type:           kind,
			Category:       "arp",
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

	// An IP that changed MAC between scans is the core MITM signal.
	for _, o := range current.rows {
		prev := previousDifferentMAC(history[o.IP], o.MAC)
		if prev == nil {
			continue
		}
		evidence := []string{
			fmt.Sprintf("MAC anterior %s · %s", prev.mac, prev.at.Format("2006-01-02 15:04")),
			fmt.Sprintf("MAC actual %s · %s", o.MAC, current.at.Format("2006-01-02 15:04")),
			"Fuente: histórico de escaneos; sin captura de paquetes",
		}
		if o.IsGateway || prev.gateway {
			add("arp_gateway_mac_changed", "critical", "Posible MITM: cambió la MAC del gateway",
				fmt.Sprintf("El gateway %s pasó de %s a %s.", o.IP, prev.mac, o.MAC),
				"Comprueba la MAC del gateway en el router y descarta otro servidor DHCP o ARP spoofing en la red.",
				95, []string{o.IP}, evidence)
		} else {
			add("arp_ip_mac_changed", "warning", "Cambió la MAC de un dispositivo",
				fmt.Sprintf("La IP %s pasó de %s a %s.", o.IP, prev.mac, o.MAC),
				"Confirma si el equipo cambió de tarjeta de red antes de descartar una suplantación.",
				75, []string{o.IP}, evidence)
		}
	}

	macIPs := map[string]map[string]bool{}
	ipMACs := map[string]map[string]bool{}
	for _, o := range current.rows {
		if macIPs[o.MAC] == nil {
			macIPs[o.MAC] = map[string]bool{}
		}
		macIPs[o.MAC][o.IP] = true
		if ipMACs[o.IP] == nil {
			ipMACs[o.IP] = map[string]bool{}
		}
		ipMACs[o.IP][o.MAC] = true
	}

	for mac, ips := range macIPs {
		if len(ips) <= limit {
			continue
		}
		list := sortedKeys(ips)
		add("arp_mac_multi_ip", "warning", "Una MAC reclama muchas IPs",
			fmt.Sprintf("La MAC %s aparece en %d direcciones: %s.", mac, len(list), strings.Join(list, ", ")),
			"Investiga una suplantación de MAC o un equipo con identidades múltiples.",
			80, list, []string{fmt.Sprintf("MAC %s · %d IPs en el último escaneo", mac, len(list))})
	}

	for ip, macs := range ipMACs {
		if len(macs) < 2 {
			continue
		}
		list := sortedKeys(macs)
		add("arp_ip_multi_mac", "warning", "Una IP tiene varias MAC",
			fmt.Sprintf("La IP %s aparece con %d MAC: %s.", ip, len(list), strings.Join(list, ", ")),
			"Busca un conflicto de IP o respuestas ARP duplicadas en el segmento.",
			85, []string{ip}, []string{fmt.Sprintf("IP %s · MACs %s", ip, strings.Join(list, ", "))})
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

func previousDifferentMAC(seen []arpSeen, current string) *arpSeen {
	var found *arpSeen
	for i := range seen {
		if seen[i].mac == current {
			continue
		}
		if found == nil || seen[i].at.After(found.at) {
			found = &seen[i]
		}
	}
	return found
}

func severityRank(s string) int {
	switch strings.ToLower(s) {
	case "critical":
		return 3
	case "warning":
		return 2
	default:
		return 1
	}
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// validARPMAC rejects empty, broadcast, multicast and all-zero addresses.
// Locally administered addresses are kept: device randomisation is legitimate.
func validARPMAC(mac string) bool {
	parts := strings.Split(mac, ":")
	if len(parts) != 6 {
		return false
	}
	b, err := hex.DecodeString(strings.Join(parts, ""))
	if err != nil || len(b) != 6 {
		return false
	}
	allZero, allFF := true, true
	for _, v := range b {
		if v != 0 {
			allZero = false
		}
		if v != 0xff {
			allFF = false
		}
	}
	return !allZero && !allFF && b[0]&1 == 0
}
