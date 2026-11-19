package snmp

import (
	"encoding/hex"
	"net"
	"sort"
	"strconv"
	"strings"

	"github.com/gosnmp/gosnmp"
	"github.com/kr3s4l4/netdiag/internal/model"
)

// F46 walks. Every table has a hard row limit: a noisy switch must not be able
// to exhaust memory or stall a scan with an unbounded response.
const (
	oidIPAddrAddr = ".1.3.6.1.2.1.4.20.1.1"
	oidIPAddrIf   = ".1.3.6.1.2.1.4.20.1.2"
	oidIPAddrMask = ".1.3.6.1.2.1.4.20.1.3"

	oidNTMIf   = ".1.3.6.1.2.1.4.22.1.1"
	oidNTMPhys = ".1.3.6.1.2.1.4.22.1.2"
	oidNTMNet  = ".1.3.6.1.2.1.4.22.1.3"
	oidNTMType = ".1.3.6.1.2.1.4.22.1.4"

	oidNTPIf   = ".1.3.6.1.2.1.4.35.1.1"
	oidNTPNet  = ".1.3.6.1.2.1.4.35.1.3"
	oidNTPPhys = ".1.3.6.1.2.1.4.35.1.4"
	oidNTPType = ".1.3.6.1.2.1.4.35.1.6"

	oidQVLANName  = ".1.3.6.1.2.1.17.7.1.4.3.1.1"
	oidQFDBAddr   = ".1.3.6.1.2.1.17.7.1.2.2.1.1"
	oidQFDBPort   = ".1.3.6.1.2.1.17.7.1.2.2.1.2"
	oidQFDBStatus = ".1.3.6.1.2.1.17.7.1.2.2.1.3"

	oidBridgeIf   = ".1.3.6.1.2.1.17.1.4.1.2"
	oidBFDBAddr   = ".1.3.6.1.2.1.17.4.3.1.1"
	oidBFDBPort   = ".1.3.6.1.2.1.17.4.3.1.2"
	oidBFDBStatus = ".1.3.6.1.2.1.17.4.3.1.3"
)

const (
	maxDeepIP   = 512
	maxDeepARP  = 2000
	maxDeepFDB  = 5000
	maxDeepVLAN = 256
)

// maskCommunity keeps a fingerprint of the credential that answered without
// persisting the secret itself. It is safe to show in the UI and in reports.
func maskCommunity(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if len(value) <= 3 {
		return "***"
	}
	return value[:2] + "***" + value[len(value)-2:]
}

// collectDeep walks IP-MIB, Q-BRIDGE-MIB and BRIDGE-MIB. Missing tables simply
// produce no rows; the poll itself remains successful.
func (c *Collector) collectDeep(client *gosnmp.GoSNMP) model.SNMPDeep {
	var out model.SNMPDeep
	out.IPAddresses = parseIPAddrTable(
		walkMap(client, oidIPAddrAddr),
		walkMap(client, oidIPAddrIf),
		walkMap(client, oidIPAddrMask),
	)

	ntmIf := walkMap(client, oidNTMIf)
	out.ARP = append(out.ARP, parseNetToMedia(
		ntmIf,
		walkMACMap(client, oidNTMPhys),
		walkMap(client, oidNTMNet),
		walkMap(client, oidNTMType),
	)...)
	out.ARP = append(out.ARP, parseNetToPhysical(
		walkMap(client, oidNTPIf),
		walkMap(client, oidNTPNet),
		walkMACMap(client, oidNTPPhys),
		walkMap(client, oidNTPType),
	)...)
	if len(out.ARP) > maxDeepARP {
		out.ARP = out.ARP[:maxDeepARP]
	}
	sort.Slice(out.ARP, func(i, j int) bool {
		if out.ARP[i].Table != out.ARP[j].Table {
			return out.ARP[i].Table < out.ARP[j].Table
		}
		return out.ARP[i].IP < out.ARP[j].IP
	})

	bridgeIf := walkMap(client, oidBridgeIf)
	vlans, qFDB := parseQBridge(
		walkMap(client, oidQVLANName),
		walkMap(client, oidQFDBAddr),
		walkMap(client, oidQFDBPort),
		walkMap(client, oidQFDBStatus),
		bridgeIf,
	)
	out.VLANs = vlans
	out.FDB = qFDB
	seen := map[string]bool{}
	for _, row := range qFDB {
		seen[fdbKey(row)] = true
	}
	for _, row := range parseBridgeFDB(
		walkMap(client, oidBFDBAddr),
		walkMap(client, oidBFDBPort),
		walkMap(client, oidBFDBStatus),
		bridgeIf,
	) {
		key := fdbKey(row)
		if seen[key] {
			continue
		}
		seen[key] = true
		out.FDB = append(out.FDB, row)
		if len(out.FDB) >= maxDeepFDB {
			break
		}
	}
	return out
}

// walkMACMap decodes OCTET STRING MAC values before converting them to text.
// A raw six-byte address is not valid UTF-8, so pduString alone loses it.
func walkMACMap(client *gosnmp.GoSNMP, root string) map[string]string {
	out := map[string]string{}
	pdus, err := client.BulkWalkAll(root)
	if err != nil || len(pdus) == 0 {
		return out
	}
	base := root + "."
	for _, pdu := range pdus {
		suffix := strings.TrimPrefix(pdu.Name, base)
		out[suffix] = pduMAC(pdu)
	}
	return out
}

func pduMAC(pdu gosnmp.SnmpPDU) string {
	if raw, ok := pdu.Value.([]byte); ok {
		return normalizeMAC(hex.EncodeToString(raw))
	}
	return normalizeMAC(pduString(pdu))
}

func parseIPAddrTable(addr, ifIndex, mask map[string]string) []model.SNMPIPAddress {
	out := []model.SNMPIPAddress{}
	for ip := range addr {
		if len(out) >= maxDeepIP {
			break
		}
		ip = strings.TrimSpace(ip)
		if net.ParseIP(ip) == nil {
			continue
		}
		out = append(out, model.SNMPIPAddress{
			IP:      ip,
			IfIndex: intOrZero(ifIndex[ip]),
			Mask:    strings.TrimSpace(mask[ip]),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].IP < out[j].IP })
	return out
}

// parseNetToMedia reads the RFC 1213 ARP table, indexed by ifIndex and IP.
func parseNetToMedia(ifIndex, phys, netAddr, typ map[string]string) []model.SNMPARPRow {
	out := []model.SNMPARPRow{}
	for suffix, rawIP := range netAddr {
		if len(out) >= maxDeepARP {
			break
		}
		ip := strings.TrimSpace(rawIP)
		if net.ParseIP(ip) == nil {
			continue
		}
		mac := normalizeMAC(phys[suffix])
		if mac == "" {
			continue
		}
		idx, ok := rowIfIndex(ifIndex, suffix)
		if !ok {
			idx, ok = firstIndex(suffix)
		}
		if !ok || idx <= 0 {
			continue
		}
		kind := arpType(typ[suffix])
		if kind == "" {
			continue
		}
		out = append(out, model.SNMPARPRow{
			IP: ip, MAC: mac, IfIndex: idx, Type: kind, Table: "net_to_media",
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].IP < out[j].IP })
	return out
}

// parseNetToPhysical reads the RFC 4293 ARP table, indexed by ifIndex,
// address type, address length and address bytes.
func parseNetToPhysical(ifIndex, netAddr, phys, typ map[string]string) []model.SNMPARPRow {
	out := []model.SNMPARPRow{}
	for suffix, rawIP := range netAddr {
		if len(out) >= maxDeepARP {
			break
		}
		ip := strings.TrimSpace(rawIP)
		parts := strings.Split(suffix, ".")
		if len(parts) >= 4 && net.ParseIP(ip) == nil {
			ip = strings.Join(parts[3:], ".")
		}
		if net.ParseIP(ip) == nil {
			continue
		}
		mac := normalizeMAC(phys[suffix])
		if mac == "" {
			continue
		}
		idx, ok := rowIfIndex(ifIndex, suffix)
		if !ok {
			if len(parts) == 0 {
				continue
			}
			idx = intOrZero(parts[0])
		}
		if idx <= 0 {
			continue
		}
		kind := arpType(typ[suffix])
		if kind == "" {
			continue
		}
		out = append(out, model.SNMPARPRow{
			IP: ip, MAC: mac, IfIndex: idx, Type: kind, Table: "net_to_physical",
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].IP < out[j].IP })
	return out
}

// parseQBridge returns VLAN names and the VLAN-aware FDB. The FDB OID contains
// the VLAN ID before the six MAC octets.
func parseQBridge(vlanNames, fdbAddr, fdbPort, fdbStatus map[string]string, bridgeIf map[string]string) ([]model.SNMPVLAN, []model.SNMPFDBRow) {
	vlans := []model.SNMPVLAN{}
	for suffix, name := range vlanNames {
		if len(vlans) >= maxDeepVLAN {
			break
		}
		id, err := strconv.Atoi(strings.TrimSpace(suffix))
		if err != nil || id <= 0 {
			continue
		}
		vlans = append(vlans, model.SNMPVLAN{VLAN: id, Name: strings.TrimSpace(name)})
	}
	sort.Slice(vlans, func(i, j int) bool { return vlans[i].VLAN < vlans[j].VLAN })

	fdb := []model.SNMPFDBRow{}
	for suffix := range fdbAddr {
		if len(fdb) >= maxDeepFDB {
			break
		}
		parts := strings.Split(suffix, ".")
		if len(parts) < 7 {
			continue
		}
		vlan, err := strconv.Atoi(parts[0])
		if err != nil || vlan <= 0 {
			continue
		}
		mac := macFromOIDSuffix(suffix)
		if mac == "" {
			continue
		}
		port := intOrZero(fdbPort[suffix])
		if port <= 0 {
			continue
		}
		status, ok := fdbStatusName(fdbStatus[suffix])
		if !ok {
			continue
		}
		fdb = append(fdb, model.SNMPFDBRow{
			MAC: mac, Port: port, IfIndex: bridgeIfIfIndex(bridgeIf, port),
			VLAN: vlan, Status: status, Table: "q_bridge",
		})
	}
	sort.Slice(fdb, func(i, j int) bool {
		if fdb[i].VLAN != fdb[j].VLAN {
			return fdb[i].VLAN < fdb[j].VLAN
		}
		return fdb[i].MAC < fdb[j].MAC
	})
	return vlans, fdb
}

// parseBridgeFDB reads the flat BRIDGE-MIB forwarding table.
func parseBridgeFDB(fdbAddr, fdbPort, fdbStatus map[string]string, bridgeIf map[string]string) []model.SNMPFDBRow {
	out := []model.SNMPFDBRow{}
	for suffix := range fdbAddr {
		if len(out) >= maxDeepFDB {
			break
		}
		mac := macFromOIDSuffix(suffix)
		if mac == "" {
			continue
		}
		port := intOrZero(fdbPort[suffix])
		if port <= 0 {
			continue
		}
		status, ok := fdbStatusName(fdbStatus[suffix])
		if !ok {
			continue
		}
		out = append(out, model.SNMPFDBRow{
			MAC: mac, Port: port, IfIndex: bridgeIfIfIndex(bridgeIf, port),
			Status: status, Table: "bridge",
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].MAC < out[j].MAC })
	return out
}

func fdbKey(row model.SNMPFDBRow) string {
	return row.MAC + "|" + strconv.Itoa(row.Port) + "|" + strconv.Itoa(row.VLAN)
}

func bridgeIfIfIndex(bridgeIf map[string]string, port int) int {
	return intOrZero(bridgeIf[strconv.Itoa(port)])
}

func fdbStatusName(raw string) (string, bool) {
	switch strings.TrimSpace(raw) {
	case "3":
		return "learned", true
	case "4":
		return "static", true
	case "5":
		return "management", true
	case "1", "2":
		return "", false
	default:
		return "other", true
	}
}

func arpType(raw string) string {
	switch strings.TrimSpace(raw) {
	case "3":
		return "dynamic"
	case "4":
		return "static"
	case "2":
		return ""
	case "1":
		return "other"
	default:
		return "other"
	}
}

func rowIfIndex(values map[string]string, suffix string) (int, bool) {
	raw, ok := values[suffix]
	if !ok {
		return 0, false
	}
	n := intOrZero(raw)
	return n, n > 0
}

func firstIndex(suffix string) (int, bool) {
	parts := strings.Split(suffix, ".")
	if len(parts) < 5 {
		return 0, false
	}
	n, err := strconv.Atoi(parts[0])
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}

func intOrZero(raw string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(raw))
	return n
}
