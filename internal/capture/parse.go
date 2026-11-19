package capture

import (
	"net"
	"strings"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/kr3s4l4/netdiag/internal/model"
)

// Observe turns one decoded frame into zero or more metadata-only records.
// It is deliberately free of pcap calls so parsers are testable in portable
// builds and in the pcap build alike.
func Observe(pkt gopacket.Packet, iface string) []model.PassiveObservation {
	if pkt == nil {
		return nil
	}
	srcIP, dstIP := ipAddrs(pkt)
	srcMAC := ""
	if eth, ok := pkt.Layer(layers.LayerTypeEthernet).(*layers.Ethernet); ok {
		srcMAC = eth.SrcMAC.String()
	}
	at := pkt.Metadata().Timestamp
	if at.IsZero() {
		at = pkt.Metadata().CaptureInfo.Timestamp
	}
	base := model.PassiveObservation{
		Interface: iface, SrcIP: srcIP, DstIP: dstIP, SrcMAC: srcMAC, ObservedAt: at,
	}
	var out []model.PassiveObservation

	if arp, ok := pkt.Layer(layers.LayerTypeARP).(*layers.ARP); ok {
		requestIP := net.IP(arp.SourceProtAddress).String()
		targetIP := net.IP(arp.DstProtAddress).String()
		broadcast := false
		if eth, ok := pkt.Layer(layers.LayerTypeEthernet).(*layers.Ethernet); ok {
			broadcast = eth.DstMAC.String() == "ff:ff:ff:ff:ff:ff"
		}
		if (arp.Operation == layers.ARPRequest && requestIP == targetIP && requestIP != "") ||
			(arp.Operation == layers.ARPReply && broadcast) {
			o := base
			o.Kind = "garp"
			o.Protocol = "arp"
			o.SrcIP = requestIP
			o.DstIP = targetIP
			o.Detail = "GARP " + requestIP + " → " + net.HardwareAddr(arp.SourceHwAddress).String()
			out = append(out, o)
		}
	}

	if icmp, ok := pkt.Layer(layers.LayerTypeICMPv4).(*layers.ICMPv4); ok {
		if icmp.TypeCode.Type() == layers.ICMPv4TypeRedirect {
			o := base
			o.Kind = "icmp_redirect"
			o.Protocol = "icmp"
			o.Detail = "ICMP redirect de " + srcIP + " a " + dstIP
			out = append(out, o)
		}
	}

	if dhcp, ok := pkt.Layer(layers.LayerTypeDHCPv4).(*layers.DHCPv4); ok {
		kind := dhcpMessage(dhcp)
		if kind != "" {
			o := base
			o.Kind = "dhcp_server"
			o.Protocol = "udp"
			o.SrcIP = firstNonEmpty(serverIdentifier(dhcp), srcIP)
			o.Detail = "DHCP " + kind
			out = append(out, o)
		}
	}

	if tcp, ok := pkt.Layer(layers.LayerTypeTCP).(*layers.TCP); ok && tcp.SYN && !tcp.ACK {
		o := base
		o.Kind = "inbound_scan"
		o.Protocol = "tcp"
		o.Detail = "SYN:" + itoa(int(tcp.DstPort))
		out = append(out, o)
	}

	if dns, ok := pkt.Layer(layers.LayerTypeDNS).(*layers.DNS); ok && !dns.QR {
		for _, q := range dns.Questions {
			name := strings.TrimSuffix(string(q.Name), ".")
			if name == "" || len(name) > 253 {
				continue
			}
			o := base
			o.Kind = "dns_tunnel"
			o.Protocol = "udp"
			o.Detail = name
			out = append(out, o)
			if len(out) >= 8 {
				return out
			}
		}
	}
	return out
}

func ipAddrs(pkt gopacket.Packet) (string, string) {
	if ip, ok := pkt.Layer(layers.LayerTypeIPv4).(*layers.IPv4); ok {
		return ip.SrcIP.String(), ip.DstIP.String()
	}
	if ip, ok := pkt.Layer(layers.LayerTypeIPv6).(*layers.IPv6); ok {
		return ip.SrcIP.String(), ip.DstIP.String()
	}
	return "", ""
}

func dhcpMessage(dhcp *layers.DHCPv4) string {
	for _, opt := range dhcp.Options {
		if opt.Type != layers.DHCPOptMessageType || len(opt.Data) == 0 {
			continue
		}
		switch layers.DHCPMsgType(opt.Data[0]) {
		case layers.DHCPMsgTypeOffer:
			return "OFFER"
		case layers.DHCPMsgTypeAck:
			return "ACK"
		}
	}
	return ""
}

func serverIdentifier(dhcp *layers.DHCPv4) string {
	for _, opt := range dhcp.Options {
		if opt.Type == layers.DHCPOptServerID && len(opt.Data) >= 4 {
			return net.IP(opt.Data[:4]).String()
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" && value != "<nil>" && value != "0.0.0.0" {
			return value
		}
	}
	return ""
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := [20]byte{}
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
