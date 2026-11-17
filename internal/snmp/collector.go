// Package snmp collects device identity and L2 topology data over SNMP.
package snmp

import (
	"context"
	"encoding/hex"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/gosnmp/gosnmp"
	"github.com/kr3s4l4/netdiag/internal/model"
)

// Config holds SNMP credentials.
type Config struct {
	Communities []string
	Version     string // 1 | 2c | 3
	Timeout     time.Duration
	Retries     int
	V3User      string
	V3AuthPass  string
	V3PrivPass  string
	V3AuthProto string
	V3PrivProto string
}

// Collector performs SNMP polls.
type Collector struct {
	cfg Config
}

// New creates a collector.
func New(cfg Config) *Collector {
	if cfg.Timeout <= 0 {
		cfg.Timeout = time.Second
	}
	return &Collector{cfg: cfg}
}

// Enabled reports whether credentials are configured.
func (c *Collector) Enabled() bool {
	if strings.EqualFold(c.cfg.Version, "3") {
		return c.cfg.V3User != ""
	}
	return len(c.cfg.Communities) > 0
}

var sysOIDs = map[string]string{
	"descr":    ".1.3.6.1.2.1.1.1.0",
	"object":   ".1.3.6.1.2.1.1.2.0",
	"uptime":   ".1.3.6.1.2.1.1.3.0",
	"contact":  ".1.3.6.1.2.1.1.4.0",
	"name":     ".1.3.6.1.2.1.1.5.0",
	"location": ".1.3.6.1.2.1.1.6.0",
}

// Collect probes a device and returns its system info, interfaces and
// neighbours. It tries each configured community until one answers.
func (c *Collector) Collect(ctx context.Context, ip string) (*model.SNMPSummary, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("SNMP no configurado")
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	communities := c.cfg.Communities
	if strings.EqualFold(c.cfg.Version, "3") {
		communities = []string{""}
	}
	var lastErr error
	for _, community := range communities {
		sum, err := c.collectWith(ctx, ip, community)
		if err == nil {
			return sum, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("sin respuesta SNMP")
	}
	return nil, lastErr
}

func (c *Collector) newClient(ip, community string) *gosnmp.GoSNMP {
	timeout := c.cfg.Timeout
	if timeout <= 0 {
		timeout = time.Second
	}
	g := &gosnmp.GoSNMP{
		Target:    ip,
		Port:      161,
		Community: community,
		Timeout:   timeout,
		Retries:   c.cfg.Retries,
		MaxOids:   20,
		Transport: "udp",
	}
	switch strings.ToLower(c.cfg.Version) {
	case "1":
		g.Version = gosnmp.Version1
	case "3":
		g.Version = gosnmp.Version3
		g.SecurityModel = gosnmp.UserSecurityModel
		g.MsgFlags = gosnmp.AuthNoPriv
		sec := &gosnmp.UsmSecurityParameters{
			UserName:                 c.cfg.V3User,
			AuthenticationProtocol:   authProto(c.cfg.V3AuthProto),
			AuthenticationPassphrase: c.cfg.V3AuthPass,
		}
		if c.cfg.V3PrivPass != "" {
			g.MsgFlags = gosnmp.AuthPriv
			sec.PrivacyProtocol = privProto(c.cfg.V3PrivProto)
			sec.PrivacyPassphrase = c.cfg.V3PrivPass
		}
		g.SecurityParameters = sec
	default:
		g.Version = gosnmp.Version2c
	}
	return g
}

func authProto(s string) gosnmp.SnmpV3AuthProtocol {
	switch strings.ToLower(s) {
	case "md5":
		return gosnmp.MD5
	case "sha224":
		return gosnmp.SHA224
	case "sha256":
		return gosnmp.SHA256
	case "sha384":
		return gosnmp.SHA384
	case "sha512":
		return gosnmp.SHA512
	default:
		return gosnmp.SHA
	}
}

func privProto(s string) gosnmp.SnmpV3PrivProtocol {
	switch strings.ToLower(s) {
	case "des":
		return gosnmp.DES
	case "aes192":
		return gosnmp.AES192
	case "aes256":
		return gosnmp.AES256
	default:
		return gosnmp.AES
	}
}

func (c *Collector) collectWith(ctx context.Context, ip, community string) (*model.SNMPSummary, error) {
	client := c.newClient(ip, community)
	if err := client.Connect(); err != nil {
		return nil, err
	}
	defer client.Conn.Close()

	packet, err := client.Get([]string{sysOIDs["descr"], sysOIDs["object"], sysOIDs["uptime"],
		sysOIDs["name"], sysOIDs["location"], sysOIDs["contact"]})
	if err != nil {
		return nil, err
	}
	if packet == nil || len(packet.Variables) == 0 || packet.Variables[0].Type == gosnmp.NoSuchObject {
		return nil, fmt.Errorf("sin datos SNMP")
	}
	sum := &model.SNMPSummary{Version: strings.ToLower(c.cfg.Version)}
	for _, v := range packet.Variables {
		switch v.Name {
		case sysOIDs["descr"]:
			sum.SysDescr = pduString(v)
		case sysOIDs["object"]:
			sum.SysObjectID = pduString(v)
		case sysOIDs["name"]:
			sum.SysName = pduString(v)
		case sysOIDs["location"]:
			sum.SysLocation = pduString(v)
		case sysOIDs["contact"]:
			sum.SysContact = pduString(v)
		case sysOIDs["uptime"]:
			sum.UptimeSecs = pduUint32(v)
		}
	}
	if sum.SysDescr == "" && sum.SysName == "" {
		return nil, fmt.Errorf("respuesta SNMP vacía")
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	sum.Interfaces = c.interfaces(client)
	sum.Neighbors = append(c.lldpNeighbors(client), c.cdpNeighbors(client)...)
	sum.Neighbors = append(sum.Neighbors, c.bridgeNeighbors(client, sum.Interfaces)...)
	return sum, nil
}

func (c *Collector) interfaces(client *gosnmp.GoSNMP) []model.SNMPInterface {
	descr := walkMap(client, ".1.3.6.1.2.1.2.2.1.2")
	names := walkMap(client, ".1.3.6.1.2.1.31.1.1.1.1")
	macs := walkMap(client, ".1.3.6.1.2.1.2.2.1.6")
	status := walkMap(client, ".1.3.6.1.2.1.2.2.1.8")
	speeds := walkMap(client, ".1.3.6.1.2.1.2.2.1.5")
	aliases := walkMap(client, ".1.3.6.1.2.1.31.1.1.1.18")

	indexes := map[int]bool{}
	for i := range descr {
		if n, err := strconv.Atoi(i); err == nil {
			indexes[n] = true
		}
	}
	for i := range names {
		if n, err := strconv.Atoi(i); err == nil {
			indexes[n] = true
		}
	}
	var out []model.SNMPInterface
	for i := range indexes {
		if len(out) >= 512 {
			break
		}
		key := strconv.Itoa(i)
		si := model.SNMPInterface{
			Index: i,
			Descr: descr[key],
			Name:  names[key],
			MAC:   normalizeMAC(macs[key]),
			Alias: aliases[key],
		}
		if si.Name == "" {
			si.Name = si.Descr
		}
		switch status[key] {
		case "1":
			si.Status = "up"
		case "2":
			si.Status = "down"
		default:
			si.Status = "unknown"
		}
		if n, err := strconv.ParseInt(speeds[key], 10, 64); err == nil && n > 0 {
			si.SpeedMbps = n / 1_000_000
		}
		out = append(out, si)
	}
	sortInterfaces(out)
	return out
}

func sortInterfaces(s []model.SNMPInterface) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1].Index > s[j].Index; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}

func (c *Collector) lldpNeighbors(client *gosnmp.GoSNMP) []model.Neighbor {
	names := walkMap(client, ".1.0.8802.1.1.2.1.4.1.1.9")
	ports := walkMap(client, ".1.0.8802.1.1.2.1.4.1.1.7")
	portDesc := walkMap(client, ".1.0.8802.1.1.2.1.4.1.1.8")
	chassis := walkMap(client, ".1.0.8802.1.1.2.1.4.1.1.5")
	sysDesc := walkMap(client, ".1.0.8802.1.1.2.1.4.1.1.10")
	base := ".1.0.8802.1.1.2.1.4.1.1.9."
	var out []model.Neighbor
	for suffix, name := range names {
		if len(out) >= 512 {
			break
		}
		localPort := lldpLocalPort(base + suffix)
		port := ports[suffix]
		if port == "" {
			port = portDesc[suffix]
		}
		n := model.Neighbor{
			Protocol:      "lldp",
			RemoteName:    name,
			RemotePort:    port,
			LocalPort:     localPort,
			RemoteChassis: chassis[suffix],
			RemoteSysDesc: sysDesc[suffix],
			Confidence:    95,
		}
		if n.RemoteName == "" && n.RemoteChassis == "" {
			continue
		}
		out = append(out, n)
	}
	return out
}

func lldpLocalPort(oid string) string {
	// ...9.<timeMark>.<localPortNum>.<remIndex>
	parts := strings.Split(strings.TrimPrefix(oid, ".1.0.8802.1.1.2.1.4.1.1.9."), ".")
	if len(parts) >= 2 {
		return parts[1]
	}
	return ""
}

func (c *Collector) cdpNeighbors(client *gosnmp.GoSNMP) []model.Neighbor {
	ids := walkMap(client, ".1.3.6.1.4.1.9.9.23.1.2.1.1.6")
	ports := walkMap(client, ".1.3.6.1.4.1.9.9.23.1.2.1.1.7")
	platforms := walkMap(client, ".1.3.6.1.4.1.9.9.23.1.2.1.1.8")
	addrs := walkMap(client, ".1.3.6.1.4.1.9.9.23.1.2.1.1.4")
	var out []model.Neighbor
	for idx, id := range ids {
		if len(out) >= 256 || id == "" {
			continue
		}
		n := model.Neighbor{
			Protocol:      "cdp",
			RemoteName:    id,
			RemotePort:    ports[idx],
			RemoteSysDesc: platforms[idx],
			Confidence:    90,
		}
		if parts := strings.Split(idx, "."); len(parts) > 0 {
			n.LocalPort = parts[0]
		}
		if raw := addrs[idx]; raw != "" {
			if ip := decodeCDPAddress(raw); ip != "" {
				n.RemoteIP = ip
			}
		}
		out = append(out, n)
	}
	return out
}

func decodeCDPAddress(raw string) string {
	// gosnmp renders octet strings as decimal text in some versions; try
	// both hex and raw forms.
	clean := strings.NewReplacer(" ", "", ":", "", "-", "").Replace(raw)
	if b, err := hex.DecodeString(clean); err == nil && len(b) >= 4 {
		ip := net.IP(b[len(b)-4:])
		if v4 := ip.To4(); v4 != nil {
			return v4.String()
		}
	}
	parts := strings.Split(raw, ".")
	if len(parts) == 4 {
		return raw
	}
	return ""
}

func (c *Collector) bridgeNeighbors(client *gosnmp.GoSNMP, ifaces []model.SNMPInterface) []model.Neighbor {
	bridgeToIf := walkMap(client, ".1.3.6.1.2.1.17.1.4.1.2")
	fdbPort := walkMap(client, ".1.3.6.1.2.1.17.4.3.1.2")
	fdbStatus := walkMap(client, ".1.3.6.1.2.1.17.4.3.1.3")
	if len(fdbPort) == 0 {
		return nil
	}
	ifName := map[int]string{}
	for _, i := range ifaces {
		ifName[i.Index] = i.Name
	}
	ifIndex := map[string]int{}
	for bridgePort, ifIdxStr := range bridgeToIf {
		if n, err := strconv.Atoi(ifIdxStr); err == nil {
			ifIndex[bridgePort] = n
		}
	}
	var out []model.Neighbor
	for suffix, port := range fdbPort {
		if len(out) >= 512 {
			break
		}
		if fdbStatus[suffix] != "3" { // learned
			continue
		}
		mac := macFromOIDSuffix(suffix)
		if mac == "" || strings.HasPrefix(mac, "01:00:5e") || mac == "ff:ff:ff:ff:ff:ff" {
			continue
		}
		local := ""
		if bpidx, err := strconv.Atoi(port); err == nil {
			if idx, ok := ifIndex[port]; ok {
				local = ifName[idx]
			} else if idx, ok := ifIndex[strconv.Itoa(bpidx)]; ok {
				local = ifName[idx]
			}
		}
		out = append(out, model.Neighbor{
			Protocol:   "bridge",
			RemoteMAC:  mac,
			LocalPort:  local,
			Confidence: 60,
		})
	}
	return out
}

func macFromOIDSuffix(suffix string) string {
	parts := strings.Split(suffix, ".")
	if len(parts) < 6 {
		return ""
	}
	var b []string
	for _, p := range parts[len(parts)-6:] {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 || n > 255 {
			return ""
		}
		b = append(b, fmt.Sprintf("%02x", n))
	}
	return strings.Join(b, ":")
}

func walkMap(client *gosnmp.GoSNMP, root string) map[string]string {
	out := map[string]string{}
	pdus, err := client.BulkWalkAll(root)
	if err != nil || len(pdus) == 0 {
		return out
	}
	base := root + "."
	for _, pdu := range pdus {
		suffix := strings.TrimPrefix(pdu.Name, base)
		out[suffix] = pduString(pdu)
	}
	return out
}

func pduString(pdu gosnmp.SnmpPDU) string {
	switch v := pdu.Value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	case []byte:
		return strings.TrimSpace(string(v))
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case uint:
		return strconv.FormatUint(uint64(v), 10)
	case uint32:
		return strconv.FormatUint(uint64(v), 10)
	case uint64:
		return strconv.FormatUint(v, 10)
	case time.Duration:
		return strconv.FormatInt(int64(v/time.Second), 10)
	case net.IP:
		return v.String()
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func pduUint32(pdu gosnmp.SnmpPDU) uint32 {
	s := pduString(pdu)
	n, _ := strconv.ParseUint(s, 10, 32)
	return uint32(n)
}

func normalizeMAC(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	clean := strings.NewReplacer(":", "", "-", "", ".", "", " ", "").Replace(raw)
	if len(clean) == 12 {
		if _, err := hex.DecodeString(clean); err == nil {
			clean = strings.ToLower(clean)
			return strings.Join([]string{clean[0:2], clean[2:4], clean[4:6], clean[6:8], clean[8:10], clean[10:12]}, ":")
		}
	}
	if b, err := hex.DecodeString(clean); err == nil && len(b) == 6 {
		return strings.ToLower(net.HardwareAddr(b).String())
	}
	// Some devices return comma/space separated decimals.
	fields := strings.FieldsFunc(raw, func(r rune) bool { return r == ' ' || r == ',' || r == '.' })
	if len(fields) == 6 {
		var parts []string
		for _, f := range fields {
			n, err := strconv.Atoi(f)
			if err != nil || n < 0 || n > 255 {
				return ""
			}
			parts = append(parts, fmt.Sprintf("%02x", n))
		}
		return strings.Join(parts, ":")
	}
	return ""
}

// SupportsBridge reports whether an interface list looks like a switch.
func SupportsBridge(ifaces []model.SNMPInterface) bool {
	return len(ifaces) > 2
}

// VendorFromSysObjectID returns a coarse vendor hint from sysObjectID.
func VendorFromSysObjectID(oid string) string {
	switch {
	case strings.HasPrefix(oid, ".1.3.6.1.4.1.9."):
		return "Cisco"
	case strings.HasPrefix(oid, ".1.3.6.1.4.1.2636."):
		return "Juniper"
	case strings.HasPrefix(oid, ".1.3.6.1.4.1.14988."):
		return "MikroTik"
	case strings.HasPrefix(oid, ".1.3.6.1.4.1.41112."):
		return "Ubiquiti"
	case strings.HasPrefix(oid, ".1.3.6.1.4.1.6574."):
		return "Synology"
	case strings.HasPrefix(oid, ".1.3.6.1.4.1.311."):
		return "Microsoft"
	case strings.HasPrefix(oid, ".1.3.6.1.4.1.2021."):
		return "Net-SNMP"
	case strings.HasPrefix(oid, ".1.3.6.1.4.1.4526."):
		return "Netgear"
	case strings.HasPrefix(oid, ".1.3.6.1.4.1.11863."):
		return "TP-Link"
	}
	return ""
}
