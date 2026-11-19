package diagnostics

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/kr3s4l4/netdiag/internal/config"
	"github.com/kr3s4l4/netdiag/internal/model"
)

// DetectPassive turns metadata-only capture records into security findings.
// No packet payload is read, so a finding is a lead for the operator, not a
// protocol-level confirmation of an attack.
func DetectPassive(obs []model.PassiveObservation, arp []model.ARPObservation,
	cfg config.Diagnostics) []model.DiagnosticAnomaly {
	out := []model.DiagnosticAnomaly{}
	minConf := cfg.MinConfidence
	if minConf < 1 {
		minConf = 60
	}
	maxOut := cfg.MaxAnomalies
	if maxOut < 1 {
		maxOut = 200
	}

	gatewayIPs := map[string]bool{}
	for _, o := range arp {
		if o.IsGateway {
			gatewayIPs[strings.TrimSpace(o.IP)] = true
		}
	}
	authorized := map[string]bool{}
	for _, ip := range cfg.AuthorizedDHCPServers {
		authorized[strings.ToLower(strings.TrimSpace(ip))] = true
	}

	add := func(kind, severity, title, detail, recommendation string, confidence int, ips, evidence []string) {
		if confidence < minConf || len(out) >= maxOut {
			return
		}
		id := "passive:" + kind + ":" + strings.Join(ips, ",")
		if len(evidence) > 0 {
			id += ":" + evidence[0]
		}
		out = append(out, model.DiagnosticAnomaly{
			ID:             id,
			Type:           kind,
			Category:       "capture",
			Severity:       severity,
			Title:          title,
			Detail:         detail,
			HostIPs:        ips,
			Confidence:     confidence,
			Evidence:       evidence,
			Recommendation: recommendation,
			DetectedAt:     time.Now(),
		})
	}

	byKind := map[string][]model.PassiveObservation{}
	for _, o := range obs {
		o.Kind = strings.TrimSpace(o.Kind)
		if o.Kind == "" {
			continue
		}
		byKind[o.Kind] = append(byKind[o.Kind], o)
	}

	// 1. Repeated gratuitous ARP can be a legitimate failover, but three or
	// more announcements from one IP in one window deserve a look.
	garps := map[string][]model.PassiveObservation{}
	for _, o := range byKind["garp"] {
		garps[firstNonEmpty(o.SrcIP, o.SrcMAC)] = append(garps[firstNonEmpty(o.SrcIP, o.SrcMAC)], o)
	}
	for source, rows := range garps {
		if len(rows) < 3 {
			continue
		}
		add("passive_gratuitous_arp", "warning", "ARP gratuito repetido",
			fmt.Sprintf("Se vieron %d anuncios ARP gratuitos desde %s en la ventana capturada.", len(rows), source),
			"Comprueba si hay un cambio de IP legítimo, un clúster en failover o un equipo suplantando una dirección.",
			75, []string{source}, []string{rows[0].Detail, "interfaz " + rows[0].Interface})
	}

	// 2. Any ICMP redirect is a routing instruction from a peer on the segment.
	for _, o := range byKind["icmp_redirect"] {
		add("passive_icmp_redirect", "critical", "ICMP redirect observado",
			fmt.Sprintf("El equipo %s envió un ICMP redirect a %s durante la captura.", o.SrcIP, o.DstIP),
			"Verifica que el emisor sea el router legítimo y desactiva el accept_redirects del host si no hace falta.",
			85, []string{o.SrcIP, o.DstIP}, []string{o.Detail, "interfaz " + o.Interface})
	}

	// 3. DHCP replies heard on the wire are stronger evidence than the active
	// UDP probe because they are real offers to real clients.
	seenDHCP := map[string]bool{}
	for _, o := range byKind["dhcp_server"] {
		ip := strings.ToLower(strings.TrimSpace(o.SrcIP))
		if ip == "" || seenDHCP[ip] {
			continue
		}
		seenDHCP[ip] = true
		if authorized[ip] || gatewayIPs[ip] {
			continue
		}
		add("passive_dhcp_server", "critical", "Servidor DHCP no autorizado capturado",
			fmt.Sprintf("Se capturó %s del servidor %s (%s) sin que figure en la lista de confianza.", o.Detail, ip, o.SrcMAC),
			"Localiza el puerto de acceso del servidor y compara su MAC con el inventario antes de autorizarlo.",
			88, []string{ip}, []string{o.Detail, "MAC " + o.SrcMAC, "interfaz " + o.Interface})
	}

	// 4. Many distinct destination ports from one source to one host look like
	// a TCP port scan. BPF already limits this to bare SYN packets.
	ports := map[string]map[string]bool{}
	for _, o := range byKind["inbound_scan"] {
		key := o.SrcIP + "|" + o.DstIP
		if ports[key] == nil {
			ports[key] = map[string]bool{}
		}
		ports[key][o.Detail] = true
	}
	for key, seen := range ports {
		if len(seen) < 10 {
			continue
		}
		parts := strings.Split(key, "|")
		add("passive_inbound_scan", "warning", "Posible escaneo de puertos entrante",
			fmt.Sprintf("%s envió SYN a %d puertos distintos de %s en la ventana capturada.", parts[0], len(seen), parts[1]),
			"Comprueba si el origen es un escáner autorizado o un equipo comprometido y revisa las reglas de firewall.",
			80, []string{parts[0], parts[1]}, []string{sortedPorts(seen)})
	}

	// 5. DNS tunnelling: very long high-entropy labels, or an unusual number of
	// distinct domains from one source in a short window.
	domains := map[string]map[string]bool{}
	emitted := map[string]bool{}
	for _, o := range byKind["dns_tunnel"] {
		source := firstNonEmpty(o.SrcIP, o.SrcMAC)
		if domains[source] == nil {
			domains[source] = map[string]bool{}
		}
		name := strings.ToLower(strings.TrimSpace(o.Detail))
		domains[source][name] = true
		label := longestLabel(name)
		if len(label) < 20 || shannon(label) < 3.2 {
			continue
		}
		key := source + "|" + name
		if emitted[key] {
			continue
		}
		emitted[key] = true
		add("passive_dns_tunnel", "warning", "Posible túnel DNS",
			fmt.Sprintf("%s consultó el dominio de alta entropía %s.", source, name),
			"Comprueba el resolvedor y el proceso que genera la consulta; los dominios de túnel suelen ser largos y aleatorios.",
			82, []string{source}, []string{"dominio " + name, "etiqueta " + label})
	}
	for source, names := range domains {
		if len(names) < 12 || emitted[source+"|volume"] {
			continue
		}
		emitted[source+"|volume"] = true
		add("passive_dns_tunnel", "warning", "Volumen de consultas DNS sospechoso",
			fmt.Sprintf("%s consultó %d dominios distintos en una ventana corta.", source, len(names)),
			"Compara con el comportamiento habitual del equipo y revisa si hay software generando nombres aleatorios.",
			80, []string{source}, []string{"primer dominio " + passiveSortedKeys(names)[0]})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Severity != out[j].Severity {
			return severityValue(out[i].Severity) > severityValue(out[j].Severity)
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func longestLabel(domain string) string {
	best := ""
	for _, label := range strings.Split(domain, ".") {
		if len(label) > len(best) {
			best = label
		}
	}
	return best
}

// shannon measures label randomness in bits per character. Base64-like tunnel
// labels score above ~3.2; ordinary words and hostnames stay below it.
func shannon(value string) float64 {
	if value == "" {
		return 0
	}
	freq := map[rune]int{}
	for _, r := range value {
		freq[r]++
	}
	n := float64(len([]rune(value)))
	entropy := 0.0
	for _, count := range freq {
		p := float64(count) / n
		entropy -= p * math.Log2(p)
	}
	return entropy
}

func sortedPorts(values map[string]bool) string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, strings.TrimPrefix(value, "SYN:"))
	}
	sort.Strings(out)
	return strings.Join(out, ", ")
}

func passiveSortedKeys(values map[string]bool) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func severityValue(severity string) int {
	switch severity {
	case model.SeverityCritical:
		return 2
	case model.SeverityWarning:
		return 1
	default:
		return 0
	}
}
