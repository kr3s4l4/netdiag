//go:build pcap

package discover

import (
	"context"
	"net"
	"net/netip"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
)

// ARPAvailable reports whether active L2 ARP scanning is compiled in.
func ARPAvailable() bool { return true }

// ARPScan sends ARP requests on a local interface and collects replies. It
// needs CAP_NET_RAW/CAP_NET_ADMIN (see scripts/install.sh).
func ARPScan(ctx context.Context, iface string, ips []netip.Addr, timeout time.Duration) map[string]string {
	out := map[string]string{}
	if iface == "" || len(ips) == 0 {
		return out
	}
	ifc, err := net.InterfaceByName(iface)
	if err != nil || len(ifc.HardwareAddr) != 6 {
		return out
	}
	var srcIP net.IP
	addrs, _ := ifc.Addrs()
	for _, a := range addrs {
		if ipn, ok := a.(*net.IPNet); ok && ipn.IP.To4() != nil {
			srcIP = ipn.IP.To4()
			break
		}
	}
	if srcIP == nil {
		return out
	}
	handle, err := pcap.OpenLive(iface, 65536, false, 100*time.Millisecond)
	if err != nil {
		return out
	}
	defer handle.Close()
	if err := handle.SetBPFFilter("arp"); err != nil {
		return out
	}
	if timeout <= 0 {
		timeout = 2 * time.Second
	}

	srcMAC := net.HardwareAddr(ifc.HardwareAddr)
	eth := &layers.Ethernet{SrcMAC: srcMAC, DstMAC: net.HardwareAddr{0xff, 0xff, 0xff, 0xff, 0xff, 0xff}, EthernetType: layers.EthernetTypeARP}
	arp := &layers.ARP{
		AddrType:          layers.LinkTypeEthernet,
		Protocol:          layers.EthernetTypeIPv4,
		HwAddressSize:     6,
		ProtAddressSize:   4,
		Operation:         layers.ARPRequest,
		SourceHwAddress:   srcMAC,
		SourceProtAddress: srcIP,
		DstHwAddress:      make([]byte, 6),
		DstProtAddress:    make([]byte, 4),
	}
	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{FixLengths: true}
	var frame []byte
	for i, addr := range ips {
		if ctx.Err() != nil {
			break
		}
		if !addr.Is4() {
			continue
		}
		arp.DstProtAddress = addr.AsSlice()
		buf.Clear()
		if err := gopacket.SerializeLayers(buf, opts, eth, arp); err != nil {
			continue
		}
		frame = buf.Bytes()
		_ = handle.WritePacketData(frame)
		if i%64 == 63 {
			time.Sleep(3 * time.Millisecond)
		}
	}

	deadline := time.Now().Add(timeout)
	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	packetSource.NoCopy = true
	ch := packetSource.Packets()
	for {
		if ctx.Err() != nil || time.Now().After(deadline) {
			break
		}
		select {
		case pkt, ok := <-ch:
			if !ok {
				return out
			}
			arpLayer, ok := pkt.Layer(layers.LayerTypeARP).(*layers.ARP)
			if !ok || arpLayer.Operation != layers.ARPReply {
				continue
			}
			ip := net.IP(arpLayer.SourceProtAddress).String()
			mac := net.HardwareAddr(arpLayer.SourceHwAddress).String()
			if mac != "" && mac != "00:00:00:00:00:00" {
				out[ip] = mac
			}
		case <-time.After(100 * time.Millisecond):
		}
	}
	return out
}
