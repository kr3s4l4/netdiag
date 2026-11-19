package capture

import (
	"net"
	"strings"
	"testing"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

func buildPacket(t *testing.T, parts ...gopacket.SerializableLayer) gopacket.Packet {
	t.Helper()
	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{FixLengths: true}
	if err := gopacket.SerializeLayers(buf, opts, parts...); err != nil {
		t.Fatal(err)
	}
	return gopacket.NewPacket(buf.Bytes(), layers.LayerTypeEthernet, gopacket.Default)
}

func ether() *layers.Ethernet {
	return &layers.Ethernet{
		SrcMAC:       net.HardwareAddr{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0x01},
		DstMAC:       net.HardwareAddr{0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
		EthernetType: layers.EthernetTypeIPv4,
	}
}

func TestObserveGratuitousARP(t *testing.T) {
	ip := net.IPv4(192, 168, 0, 50).To4()
	pkt := buildPacket(t,
		&layers.Ethernet{
			SrcMAC:       net.HardwareAddr{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0x01},
			DstMAC:       net.HardwareAddr{0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
			EthernetType: layers.EthernetTypeARP,
		},
		&layers.ARP{
			AddrType: layers.LinkTypeEthernet, Protocol: layers.EthernetTypeIPv4,
			HwAddressSize: 6, ProtAddressSize: 4, Operation: layers.ARPRequest,
			SourceHwAddress:   net.HardwareAddr{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0x01},
			SourceProtAddress: ip, DstHwAddress: make([]byte, 6), DstProtAddress: ip,
		},
	)
	obs := Observe(pkt, "wlan0")
	if len(obs) != 1 || obs[0].Kind != "garp" || obs[0].SrcIP != "192.168.0.50" {
		t.Fatalf("GARP inesperado: %+v", obs)
	}
}

func TestObserveICMPRedirect(t *testing.T) {
	ip4 := &layers.IPv4{Version: 4, TTL: 64, Protocol: layers.IPProtocolICMPv4,
		SrcIP: net.IPv4(192, 168, 0, 1), DstIP: net.IPv4(192, 168, 0, 50)}
	pkt := buildPacket(t, ether(), ip4, &layers.ICMPv4{
		TypeCode: layers.CreateICMPv4TypeCode(layers.ICMPv4TypeRedirect, 0),
	})
	obs := Observe(pkt, "wlan0")
	if len(obs) != 1 || obs[0].Kind != "icmp_redirect" || obs[0].SrcIP != "192.168.0.1" {
		t.Fatalf("redirect inesperado: %+v", obs)
	}
}

func TestObserveDHCPOffer(t *testing.T) {
	ip4 := &layers.IPv4{Version: 4, TTL: 64, Protocol: layers.IPProtocolUDP,
		SrcIP: net.IPv4(192, 168, 0, 51), DstIP: net.IPv4(255, 255, 255, 255)}
	udp := &layers.UDP{SrcPort: 67, DstPort: 68}
	// Serialize the DHCP header as raw bytes: gopacket's DHCPv4 SerializeTo
	// needs a preallocated padded buffer, while UDP ports 67/68 decode it back.
	dhcpBytes := make([]byte, 240)
	dhcpBytes[0] = byte(layers.DHCPOpReply)
	copy(dhcpBytes[236:240], []byte{0x63, 0x82, 0x53, 0x63})
	dhcpBytes = append(dhcpBytes,
		byte(layers.DHCPOptMessageType), 1, byte(layers.DHCPMsgTypeOffer),
		byte(layers.DHCPOptServerID), 4, 192, 168, 0, 51,
		byte(layers.DHCPOptEnd))
	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{FixLengths: true}
	if err := gopacket.SerializeLayers(buf, opts, ether(), ip4, udp, gopacket.Payload(dhcpBytes)); err != nil {
		t.Fatal(err)
	}
	pkt := gopacket.NewPacket(buf.Bytes(), layers.LayerTypeEthernet, gopacket.Default)
	obs := Observe(pkt, "wlan0")
	if len(obs) != 1 || obs[0].Kind != "dhcp_server" || obs[0].SrcIP != "192.168.0.51" || !strings.Contains(obs[0].Detail, "OFFER") {
		t.Fatalf("DHCP inesperado: %+v", obs)
	}
}

func TestObserveTCPSYNAndDNS(t *testing.T) {
	ip4 := &layers.IPv4{Version: 4, TTL: 64, Protocol: layers.IPProtocolTCP,
		SrcIP: net.IPv4(10, 0, 0, 9), DstIP: net.IPv4(192, 168, 0, 50)}
	tcp := &layers.TCP{SrcPort: 40000, DstPort: 445, SYN: true}
	pkt := buildPacket(t, ether(), ip4, tcp)
	obs := Observe(pkt, "wlan0")
	if len(obs) != 1 || obs[0].Kind != "inbound_scan" || obs[0].Detail != "SYN:445" {
		t.Fatalf("SYN inesperado: %+v", obs)
	}

	ip4 = &layers.IPv4{Version: 4, TTL: 64, Protocol: layers.IPProtocolUDP,
		SrcIP: net.IPv4(10, 0, 0, 9), DstIP: net.IPv4(8, 8, 8, 8)}
	udp := &layers.UDP{SrcPort: 41000, DstPort: 53}
	dns := &layers.DNS{Questions: []layers.DNSQuestion{
		{Name: []byte("abcdefghijklmnopqrstuvwxyz012345.example.com"), Type: layers.DNSTypeA, Class: layers.DNSClassIN},
	}}
	buf := gopacket.NewSerializeBuffer()
	if err := gopacket.SerializeLayers(buf, optsForTest(), ether(), ip4, udp, dns); err != nil {
		t.Fatal(err)
	}
	pkt = gopacket.NewPacket(buf.Bytes(), layers.LayerTypeEthernet, gopacket.Default)
	obs = Observe(pkt, "wlan0")
	if len(obs) != 1 || obs[0].Kind != "dns_tunnel" || !strings.Contains(obs[0].Detail, "example.com") {
		t.Fatalf("DNS inesperado: %+v", obs)
	}
}

func optsForTest() gopacket.SerializeOptions {
	return gopacket.SerializeOptions{FixLengths: true}
}

func TestConfigNormalizeIsBounded(t *testing.T) {
	cfg := Config{}
	cfg.Normalize()
	if cfg.Duration <= 0 || cfg.MaxPackets < 100 || cfg.BPFFilter == "" {
		t.Fatalf("normalización inesperada: %+v", cfg)
	}
	if !strings.Contains(cfg.BPFFilter, "tcp") || !strings.Contains(cfg.BPFFilter, "arp") {
		t.Fatalf("BPF demasiado amplio: %s", cfg.BPFFilter)
	}
}
