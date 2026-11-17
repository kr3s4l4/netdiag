// Package fingerprint turns raw scan observations into device identity:
// vendor (OUI), device type, OS, services and banners.
package fingerprint

import (
	"fmt"
	"sort"
	"strings"

	"github.com/kr3s4l4/netdiag/internal/model"
)

// QuickPorts is a fast, high-signal set for everyday sweeps.
var QuickPorts = []int{
	22, 53, 80, 135, 139, 443, 445, 515, 548, 554, 631, 1883, 3389,
	5060, 8000, 8080, 8443, 8888, 9100, 32400, 62078,
}

// CommonPorts adds the most frequent service ports. ~120 probes per host.
var CommonPorts = mergePorts(QuickPorts, []int{
	21, 23, 25, 49, 67, 69, 110, 111, 123, 143, 161, 162, 179, 389, 427,
	465, 500, 587, 636, 873, 993, 995, 1080, 1194, 1433, 1521, 1723, 1900,
	2049, 2181, 2375, 2376, 3000, 3128, 3260, 3306, 3478, 3690, 4000, 4369,
	5000, 5001, 5432, 5601, 5672, 5900, 5985, 5986, 6379, 6443, 7000, 7001,
	8008, 8009, 8060, 8081, 8086, 8090, 8123, 8200, 8500, 8880, 9000, 9001,
	9042, 9090, 9200, 9300, 9443, 10000, 11211, 15672, 27017, 27018, 49152,
})

// FullPorts scans all privileged ports plus the common set. Complete range
// scanning is available through an explicit custom port list.
var FullPorts = func() []int {
	out := make([]int, 0, 1100)
	for p := 1; p <= 1024; p++ {
		out = append(out, p)
	}
	return mergePorts(out, CommonPorts)
}()

func mergePorts(base []int, extra []int) []int {
	seen := map[int]bool{}
	var out []int
	for _, p := range append(append([]int{}, base...), extra...) {
		if p > 0 && p <= 65535 && !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	sort.Ints(out)
	return out
}

// ProfilePorts resolves a scan profile to a port list.
func ProfilePorts(profile string, custom []int) []int {
	switch strings.ToLower(profile) {
	case "quick":
		return QuickPorts
	case "full":
		return FullPorts
	case "custom":
		if len(custom) > 0 {
			out := make([]int, 0, len(custom))
			seen := map[int]bool{}
			for _, p := range custom {
				if p > 0 && p <= 65535 && !seen[p] {
					seen[p] = true
					out = append(out, p)
				}
			}
			sort.Ints(out)
			return out
		}
		return CommonPorts
	default:
		return CommonPorts
	}
}

// ParsePorts parses "22,80,8000-8100" into a sorted list.
func ParsePorts(spec string) ([]int, error) {
	var out []int
	seen := map[int]bool{}
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.Contains(part, "-") {
			var lo, hi int
			if _, err := fmt.Sscanf(part, "%d-%d", &lo, &hi); err != nil {
				return nil, err
			}
			if lo < 1 || hi > 65535 || lo > hi {
				return nil, errInvalidPort(part)
			}
			for p := lo; p <= hi; p++ {
				if !seen[p] {
					seen[p] = true
					out = append(out, p)
				}
			}
			continue
		}
		var p int
		if _, err := fmt.Sscanf(part, "%d", &p); err != nil || p < 1 || p > 65535 {
			return nil, errInvalidPort(part)
		}
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	sort.Ints(out)
	return out, nil
}

func errInvalidPort(s string) error { return &portError{s: s} }

type portError struct{ s string }

func (e *portError) Error() string { return "puerto o rango inválido: " + e.s }

// ServiceName returns the conventional service for a port.
func ServiceName(port int) string {
	if s, ok := serviceNames[port]; ok {
		return s
	}
	return ""
}

var serviceNames = map[int]string{
	7: "echo", 9: "discard", 20: "ftp-data", 21: "ftp", 22: "ssh", 23: "telnet",
	25: "smtp", 37: "time", 43: "whois", 49: "tacacs", 53: "dns", 67: "dhcp",
	68: "dhcp", 69: "tftp", 80: "http", 88: "kerberos", 110: "pop3", 111: "rpcbind",
	123: "ntp", 135: "msrpc", 137: "netbios-ns", 138: "netbios-dgm", 139: "netbios-ssn",
	143: "imap", 161: "snmp", 162: "snmptrap", 179: "bgp", 389: "ldap", 427: "svrloc",
	443: "https", 445: "smb", 465: "smtps", 500: "isakmp", 512: "exec", 513: "login",
	514: "syslog", 515: "lpd", 548: "afp", 554: "rtsp", 587: "submission", 631: "ipp",
	636: "ldaps", 873: "rsync", 993: "imaps", 995: "pop3s", 1080: "socks", 1194: "openvpn",
	1433: "mssql", 1521: "oracle", 1723: "pptp", 1883: "mqtt", 1900: "ssdp",
	2049: "nfs", 2181: "zookeeper", 2375: "docker", 2376: "docker-tls", 3000: "http-alt",
	3128: "squid", 3260: "iscsi", 3306: "mysql", 3389: "rdp", 3478: "stun",
	3690: "svn", 4000: "http-alt", 4369: "epmd", 5000: "http-alt", 5001: "https-alt",
	5060: "sip", 5061: "sips", 5222: "xmpp", 5353: "mdns", 5432: "postgresql",
	5601: "kibana", 5672: "amqp", 5900: "vnc", 5985: "winrm", 5986: "winrm-tls",
	6379: "redis", 6443: "kubernetes", 7000: "http-alt", 7001: "https-alt",
	8000: "http-alt", 8008: "http-alt", 8009: "ajp", 8060: "roku", 8080: "http-proxy",
	8081: "http-alt", 8086: "influxdb", 8090: "http-alt", 8123: "home-assistant",
	8200: "http-alt", 8443: "https-alt", 8500: "consul", 8880: "http-alt",
	8888: "http-alt", 9000: "http-alt", 9001: "tor", 9042: "cassandra",
	9090: "prometheus", 9100: "printer", 9200: "elasticsearch", 9300: "elasticsearch",
	9443: "https-alt", 10000: "webmin", 11211: "memcached", 15672: "rabbitmq",
	27017: "mongodb", 27018: "mongodb", 32400: "plex", 37777: "dahua",
	49152: "upnp", 5228: "android-gcm", 62078: "iphone-sync",
}

// ProfileDescription returns a human label and probe count.
func ProfileDescription(profile string) (string, int) {
	switch strings.ToLower(profile) {
	case "quick":
		return "Rápido", len(QuickPorts)
	case "full":
		return "Completo", len(FullPorts)
	case "custom":
		return "Personalizado", 0
	default:
		return "Estándar", len(CommonPorts)
	}
}

// OpenPorts extracts open ports from a host.
func OpenPorts(h model.Host) []model.Port {
	var out []model.Port
	for _, p := range h.Ports {
		if p.State == "open" || p.State == "" {
			out = append(out, p)
		}
	}
	return out
}
