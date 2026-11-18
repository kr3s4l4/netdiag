// Package vpn derives VPN tunnel observations from local kernel tables and
// the scan inventory. It never decrypts traffic: findings are heuristics with
// explicit evidence and confidence, in the same spirit as F4 diagnostics.
package vpn

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Observation is one piece of evidence for a tunnel.
type Observation struct {
	Type       string
	LocalIP    string
	LocalPort  int
	RemoteIP   string
	RemotePort int
	BytesSent  int64
	BytesRecv  int64
	State      string
	Evidence   []string
	Confidence int
}

// Identity returns a stable key for upserting observations.
func (o Observation) Identity() string {
	return fmt.Sprintf("%s|%s:%d|%s:%d", o.Type, o.LocalIP, o.LocalPort, o.RemoteIP, o.RemotePort)
}

// Detector reads VPN evidence from a procfs root ("/proc" by default). The
// root is injectable so the parser can be tested with fixtures.
type Detector struct {
	Root string
}

func (d *Detector) root() string {
	if d.Root == "" {
		return "/proc"
	}
	return d.Root
}

// Observe collects all current observations. Duplicate identities are merged
// by keeping the highest confidence and summing byte counters.
func (d *Detector) Observe() []Observation {
	merged := map[string]Observation{}
	add := func(o Observation) {
		if prev, ok := merged[o.Identity()]; ok {
			merged[o.Identity()] = mergeObservations(prev, o)
		} else {
			merged[o.Identity()] = o
		}
	}
	for _, o := range d.sockets() {
		add(o)
	}
	for _, o := range d.interfaces() {
		// A local interface and its control socket describe the same tunnel.
		// When there is exactly one socket of this type, fold them together
		// so the UI shows one row with both pieces of evidence.
		if o.RemoteIP == "" && o.LocalPort == 0 {
			var match *Observation
			count := 0
			for _, prev := range merged {
				if prev.Type == o.Type {
					match = &prev
					count++
				}
			}
			if count == 1 {
				o.LocalIP = match.LocalIP
				o.LocalPort = match.LocalPort
				o.RemoteIP = match.RemoteIP
				o.RemotePort = match.RemotePort
			}
		}
		add(o)
	}
	out := make([]Observation, 0, len(merged))
	for _, o := range merged {
		if o.State == "" {
			o.State = "observed"
		}
		out = append(out, o)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Confidence != out[j].Confidence {
			return out[i].Confidence > out[j].Confidence
		}
		return out[i].Identity() < out[j].Identity()
	})
	return out
}

func mergeObservations(a, b Observation) Observation {
	a.BytesSent += b.BytesSent
	a.BytesRecv += b.BytesRecv
	a.Evidence = append(a.Evidence, b.Evidence...)
	if b.Confidence > a.Confidence {
		a.Confidence = b.Confidence
		a.State = b.State
	}
	sort.Strings(a.Evidence)
	return a
}

// vpnPort maps a well-known port to a tunnel type and baseline confidence.
func vpnPort(port int, network string) (string, int) {
	switch port {
	case 51820:
		return "wireguard", 92
	case 1194:
		return "openvpn", 88
	case 500, 4500:
		return "ipsec", 82
	case 1723:
		return "pptp", 70
	case 443:
		if network == "tcp" {
			return "openvpn", 35 // HTTPS is far more common; needs more evidence.
		}
	}
	return "", 0
}

// sockets inspects /proc/net/{udp,udp6,tcp,tcp6} for VPN ports. Established
// TCP and connected UDP sockets carry a remote endpoint.
func (d *Detector) sockets() []Observation {
	var out []Observation
	for _, spec := range []struct{ file, network string }{
		{"udp", "udp"}, {"udp6", "udp6"}, {"tcp", "tcp"}, {"tcp6", "tcp6"},
	} {
		data, err := os.ReadFile(filepath.Join(d.root(), "net", spec.file))
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(strings.NewReader(string(data)))
		scanner.Buffer(make([]byte, 0, 64<<10), 1<<20)
		header := true
		for scanner.Scan() {
			if header {
				header = false
				continue
			}
			fields := strings.Fields(scanner.Text())
			if len(fields) < 4 {
				continue
			}
			localIP, localPort, ok := parseProcAddr(fields[1], spec.network == "udp6" || spec.network == "tcp6")
			if !ok {
				continue
			}
			remoteIP, remotePort, okRemote := parseProcAddr(fields[2], spec.network == "udp6" || spec.network == "tcp6")
			typ, confidence := vpnPort(localPort, spec.network)
			if typ == "" {
				typ, confidence = vpnPort(remotePort, spec.network)
			}
			if typ == "" {
				continue
			}
			state := "observed"
			if spec.network == "tcp" || spec.network == "tcp6" {
				if fields[3] == "01" { // TCP_ESTABLISHED
					state = "active"
				} else {
					continue
				}
			} else if okRemote && remoteIP != "" && !isZeroIP(remoteIP) {
				state = "active"
			}
			o := Observation{
				Type: typ, LocalIP: localIP, LocalPort: localPort,
				RemoteIP: remoteIP, RemotePort: remotePort,
				State: state, Confidence: confidence,
				Evidence: []string{fmt.Sprintf("socket %s local=%s:%d rem=%s:%d state=%s",
					spec.network, localIP, localPort, remoteIP, remotePort, fields[3])},
			}
			if typ == "openvpn" && localPort == 443 {
				o.Evidence = append(o.Evidence, "TCP/443 es ambiguo: verificar con el servidor VPN")
			}
			out = append(out, o)
		}
	}
	return out
}

// interfaces inspects /proc/net/dev and the sysfs network directory for
// WireGuard (wg*) and tun/tap devices, attaching cumulative byte counters.
func (d *Detector) interfaces() []Observation {
	var out []Observation
	data, err := os.ReadFile(filepath.Join(d.root(), "net", "dev"))
	if err != nil {
		return out
	}
	for _, line := range strings.Split(string(data), "\n")[2:] {
		fields := strings.Fields(line)
		if len(fields) < 11 {
			continue
		}
		name := strings.TrimSuffix(fields[0], ":")
		rx, errRX := strconv.ParseInt(fields[1], 10, 64)
		tx, errTX := strconv.ParseInt(fields[9], 10, 64)
		if errRX != nil || errTX != nil {
			continue
		}
		typ, confidence := "", 0
		switch {
		case strings.HasPrefix(name, "wg"):
			typ, confidence = "wireguard", 95
		case strings.HasPrefix(name, "tun"):
			typ, confidence = "openvpn", 72
		case strings.HasPrefix(name, "tap"):
			typ, confidence = "openvpn", 68
		}
		if typ == "" {
			continue
		}
		operstate := ""
		if b, err := os.ReadFile(filepath.Join(d.root(), "sys", "class", "net", name, "operstate")); err == nil {
			operstate = strings.TrimSpace(string(b))
		}
		state := "observed"
		if operstate == "up" || rx > 0 || tx > 0 {
			state = "active"
		}
		localIP := interfaceIP(name)
		out = append(out, Observation{
			Type: typ, LocalIP: localIP, State: state, Confidence: confidence,
			BytesRecv: rx, BytesSent: tx,
			Evidence: []string{fmt.Sprintf("interfaz %s operstate=%s rx=%d tx=%d (contadores acumulados)", name, operstate, rx, tx)},
		})
	}
	return out
}

func interfaceIP(name string) string {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return ""
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return ""
	}
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && ipnet.IP.To4() != nil {
			return ipnet.IP.String()
		}
	}
	return ""
}

// parseProcAddr decodes the "0100007F:0035" (IPv4) or long IPv6 hex form
// used by /proc/net/udp* and /proc/net/tcp*.
func parseProcAddr(value string, ipv6 bool) (string, int, bool) {
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return "", 0, false
	}
	port, err := strconv.ParseUint(parts[1], 16, 16)
	if err != nil {
		return "", 0, false
	}
	raw, err := hex.DecodeString(parts[0])
	if err != nil {
		return "", 0, false
	}
	if ipv6 {
		if len(raw) != 16 {
			return "", 0, false
		}
		// Each 4-byte group is stored little-endian.
		ip := make([]byte, 16)
		for i := 0; i < 4; i++ {
			copy(ip[i*4:i*4+4], raw[i*4:i*4+4])
			reverse(ip[i*4 : i*4+4])
		}
		return net.IP(ip).String(), int(port), true
	}
	if len(raw) != 4 {
		return "", 0, false
	}
	reverse(raw)
	return net.IP(raw).String(), int(port), true
}

func reverse(b []byte) {
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}
}

func isZeroIP(ip string) bool {
	p := net.ParseIP(ip)
	return p == nil || p.IsUnspecified()
}
