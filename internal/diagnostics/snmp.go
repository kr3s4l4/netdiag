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

// snmpBatch groups rows collected in the same poll. Change detection needs at
// least two batches; a single snapshot can only reveal internal contradictions.
type snmpBatch[T any] struct {
	key  string
	at   time.Time
	rows []T
}

func snmpBatches[T any](rows []T, keyOf func(T) string, atOf func(T) time.Time) []*snmpBatch[T] {
	byKey := map[string]*snmpBatch[T]{}
	for _, row := range rows {
		key := keyOf(row)
		b := byKey[key]
		if b == nil {
			b = &snmpBatch[T]{key: key, at: atOf(row)}
			byKey[key] = b
		}
		if at := atOf(row); at.After(b.at) {
			b.at = at
		}
		b.rows = append(b.rows, row)
	}
	keys := make([]string, 0, len(byKey))
	for key := range byKey {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, b := byKey[keys[i]], byKey[keys[j]]
		if !a.at.Equal(b.at) {
			return a.at.Before(b.at)
		}
		return a.key < b.key
	})
	out := make([]*snmpBatch[T], 0, len(keys))
	for _, key := range keys {
		out = append(out, byKey[key])
	}
	return out
}

func snmpKey(scanID int64, at time.Time) string {
	if scanID > 0 {
		return "scan:" + strconv.FormatInt(scanID, 10)
	}
	return "time:" + strconv.FormatInt(at.Unix(), 10)
}

// DetectSNMP turns raw IP-MIB/Q-BRIDGE rows into security findings. It only
// reads polled tables: no raw frames, no promiscuous mode and no new packets.
func DetectSNMP(snap *model.SNMPDeepSnapshot, arp []model.ARPObservation, hosts []model.Host,
	neighbors map[int64][]model.Neighbor, cfg config.Diagnostics) []model.DiagnosticAnomaly {
	out := []model.DiagnosticAnomaly{}
	if snap == nil {
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

	hostByIP := map[string]model.Host{}
	hostByID := map[int64]model.Host{}
	for _, h := range hosts {
		hostByIP[h.IP] = h
		hostByID[h.ID] = h
	}
	gatewayIPs := map[string]bool{}
	for _, o := range arp {
		if o.IsGateway {
			gatewayIPs[strings.TrimSpace(o.IP)] = true
		}
	}

	add := func(kind, severity, title, detail, recommendation string, confidence int, ips, evidence []string) {
		if confidence < minConf || len(out) >= maxOut {
			return
		}
		id := "snmp:" + kind + ":" + strings.Join(ips, ",")
		if len(evidence) > 0 {
			id += ":" + evidence[0]
		}
		out = append(out, model.DiagnosticAnomaly{
			ID:             id,
			Type:           kind,
			Category:       "snmp",
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

	// 1. The switch ARP table must agree with the inventory ARP history.
	arpBatches := snmpBatches(snap.ARP,
		func(r model.SNMPARPRow) string { return snmpKey(r.ScanID, r.ObservedAt) },
		func(r model.SNMPARPRow) time.Time { return r.ObservedAt })
	if len(arpBatches) > 0 {
		current := arpBatches[len(arpBatches)-1]
		previous := map[string]string{}
		for _, b := range arpBatches[:len(arpBatches)-1] {
			for _, row := range b.rows {
				previous[row.IP] = strings.ToLower(row.MAC)
			}
		}
		for _, row := range current.rows {
			old, ok := previous[row.IP]
			if !ok || old == strings.ToLower(row.MAC) {
				continue
			}
			critical := gatewayIPs[row.IP]
			severity, confidence := "warning", 85
			title := "Cambió la MAC en la tabla ARP del switch"
			if critical {
				severity, confidence = "critical", 90
				title = "Posible MITM: cambió la MAC del gateway en el switch"
			}
			add(kindFor(critical, "snmp_arp_conflict", "snmp_gateway_arp_conflict"),
				severity, title,
				fmt.Sprintf("El switch %s vio %s pasar de %s a %s.", row.HostIP, row.IP, old, row.MAC),
				"Compara la tabla ARP del switch con el router y revisa si hay otro equipo respondiendo por esa IP.",
				confidence, []string{row.IP}, []string{
					fmt.Sprintf("Equipo sondeado %s · comunidad %s", row.HostIP, communityLabel(row.Community)),
					fmt.Sprintf("Tabla %s · ifIndex %d", row.Table, row.IfIndex),
					fmt.Sprintf("MAC anterior %s · MAC actual %s", old, row.MAC),
				})
		}
	}

	// 2. FDB contradictions inside one poll and between consecutive polls.
	fdbBatches := snmpBatches(snap.FDB,
		func(r model.SNMPFDBRow) string { return snmpKey(r.ScanID, r.ObservedAt) },
		func(r model.SNMPFDBRow) time.Time { return r.ObservedAt })
	if len(fdbBatches) > 0 {
		current := fdbBatches[len(fdbBatches)-1]
		previousPort := map[string]string{}
		for _, b := range fdbBatches[:len(fdbBatches)-1] {
			for _, row := range b.rows {
				previousPort[row.HostIP+"|"+row.MAC] = fmt.Sprintf("puerto %d / VLAN %d", row.Port, row.VLAN)
			}
		}
		ports := map[string]map[string]bool{}
		vlans := map[string]map[int]bool{}
		for _, row := range current.rows {
			mac := strings.ToLower(row.MAC)
			if ports[mac] == nil {
				ports[mac] = map[string]bool{}
			}
			ports[mac][fmt.Sprintf("%s|%d|%d", row.HostIP, row.Port, row.VLAN)] = true
			if vlans[mac] == nil {
				vlans[mac] = map[int]bool{}
			}
			vlans[mac][row.VLAN] = true

			if before, ok := previousPort[row.HostIP+"|"+mac]; ok && before != fmt.Sprintf("puerto %d / VLAN %d", row.Port, row.VLAN) {
				add("snmp_fdb_mac_moved", "warning", "Una MAC cambió de puerto en el switch",
					fmt.Sprintf("La MAC %s estaba en %s y ahora está en puerto %d / VLAN %d.", mac, before, row.Port, row.VLAN),
					"Descarta un cable movido o una suplantación de MAC en el puerto nuevo.",
					88, []string{row.HostIP}, []string{
						fmt.Sprintf("Equipo sondeado %s · comunidad %s", row.HostIP, communityLabel(row.Community)),
						fmt.Sprintf("Tabla %s · estado %s", row.Table, row.Status),
						fmt.Sprintf("Antes %s · ahora puerto %d", before, row.Port),
					})
			}
		}
		for mac, seen := range ports {
			if len(seen) < 2 {
				continue
			}
			add("snmp_fdb_port_conflict", "warning", "Una MAC aparece en varios puertos",
				fmt.Sprintf("La MAC %s está aprendida en %d puertos o VLAN distintas.", mac, len(seen)),
				"Busca un lazo de capa 2, dos switches sin coordinar o una suplantación de MAC.",
				82, []string{mac}, []string{sortedSet(seen)})
		}
		for mac, ids := range vlans {
			if len(ids) < 2 {
				continue
			}
			add("snmp_fdb_vlan_conflict", "warning", "Una MAC aparece en varias VLAN",
				fmt.Sprintf("La MAC %s está en %d VLAN del mismo switch.", mac, len(ids)),
				"Comprueba el trunk y el etiquetado: puede haber fuga de VLAN o bridging no autorizado.",
				85, []string{mac}, []string{fmt.Sprintf("VLANes %s", sortedInts(ids))})
		}

		// 3. Correlate LLDP/CDP with the forwarding database.
		ifNames := map[string]map[int]string{}
		for ip, h := range hostByIP {
			if h.SNMP == nil {
				continue
			}
			names := map[int]string{}
			for _, i := range h.SNMP.Interfaces {
				names[i.Index] = i.Name
			}
			ifNames[ip] = names
		}
		neighborPortMAC := map[string]map[string]string{}
		for id, ns := range neighbors {
			h, ok := hostByID[id]
			if !ok {
				continue
			}
			for _, n := range ns {
				if (n.Protocol != "lldp" && n.Protocol != "cdp") || n.RemoteMAC == "" || n.LocalPort == "" {
					continue
				}
				if neighborPortMAC[h.IP] == nil {
					neighborPortMAC[h.IP] = map[string]string{}
				}
				neighborPortMAC[h.IP][strings.ToLower(n.LocalPort)] = strings.ToLower(n.RemoteMAC)
			}
		}
		for _, row := range current.rows {
			portName := ifNames[row.HostIP][row.IfIndex]
			if portName == "" {
				continue
			}
			for port, mac := range neighborPortMAC[row.HostIP] {
				if mac != strings.ToLower(row.MAC) || strings.EqualFold(port, portName) {
					continue
				}
				add("snmp_lldp_fdb_disagree", "warning", "LLDP y FDB no coinciden en el puerto",
					fmt.Sprintf("El vecino %s anuncia la MAC %s en %s, pero el switch la aprendió en %s.",
						row.HostIP, row.MAC, port, portName),
					"Revisa el cableado y la tabla de reenvío antes de descartar un movimiento de puerto.",
					88, []string{row.HostIP}, []string{
						fmt.Sprintf("LLDP local %s · MAC %s", port, mac),
						fmt.Sprintf("FDB ifIndex %d · %s", row.IfIndex, portName),
					})
				break
			}
		}

		// 4. Unknown VLANs only matter on devices that publish VLAN names.
		if len(snap.VLANs) > 0 {
			known := map[int]bool{}
			for _, v := range snap.VLANs {
				known[v.VLAN] = true
			}
			seenUnknown := map[int]bool{}
			for _, row := range current.rows {
				if row.VLAN <= 0 || known[row.VLAN] || seenUnknown[row.VLAN] {
					continue
				}
				seenUnknown[row.VLAN] = true
				add("snmp_vlan_unknown", "info", "FDB referencia una VLAN sin nombre",
					fmt.Sprintf("El switch %s aprende direcciones en la VLAN %d, que no aparece en dot1qVlanStaticTable.", row.HostIP, row.VLAN),
					"Comprueba si la VLAN existe o si el equipo limita el número de nombres devueltos.",
					60, []string{row.HostIP}, []string{
						fmt.Sprintf("VLAN %d · MAC %s · puerto %d", row.VLAN, row.MAC, row.Port),
					})
			}
		}
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

func kindFor(critical bool, normal, criticalKind string) string {
	if critical {
		return criticalKind
	}
	return normal
}

func communityLabel(value string) string {
	if value == "" {
		return "desconocida"
	}
	return value
}

func sortedSet(values map[string]bool) string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return strings.Join(out, " · ")
}

func sortedInts(values map[int]bool) string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, strconv.Itoa(value))
	}
	sort.Strings(out)
	return strings.Join(out, ", ")
}
