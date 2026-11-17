package discover

import (
	"context"
	"net"
	"net/netip"
	"time"

	"github.com/kr3s4l4/netdiag/internal/netutil"
)

// NudgeARP populates the kernel neighbour table by sending one UDP datagram to
// every address on the local segment. It is unprivileged and typically resolves
// a /24 in under a second.
func NudgeARP(ctx context.Context, iface string, ips []netip.Addr, wait time.Duration) map[string]string {
	if wait <= 0 {
		wait = 400 * time.Millisecond
	}
	var localIP net.IP
	ifaces, _ := net.Interfaces()
	for _, i := range ifaces {
		if iface != "" && i.Name != iface {
			continue
		}
		if i.Flags&net.FlagUp == 0 || i.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := i.Addrs()
		for _, a := range addrs {
			if ipn, ok := a.(*net.IPNet); ok && ipn.IP.To4() != nil && netutil.IsPrivate(ipn.IP) {
				localIP = ipn.IP.To4()
				break
			}
		}
		if localIP != nil {
			break
		}
	}
	if localIP == nil {
		localIP = net.IPv4zero
	}
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: localIP, Port: 0})
	if err != nil {
		return netutil.ARPTable()
	}
	defer conn.Close()
	payload := []byte{0}
	for i, addr := range ips {
		if ctx.Err() != nil {
			break
		}
		ip := net.IP(addr.AsSlice())
		_, _ = conn.WriteToUDP(payload, &net.UDPAddr{IP: ip, Port: 9})
		if i%256 == 255 {
			time.Sleep(10 * time.Millisecond)
		}
	}
	select {
	case <-time.After(wait):
	case <-ctx.Done():
	}
	return netutil.ARPTable()
}

// filterARP keeps entries whose IP belongs to the target set.
func filterARP(table map[string]string, ips []netip.Addr) map[string]string {
	if len(table) == 0 {
		return table
	}
	want := make(map[string]bool, len(ips))
	for _, a := range ips {
		want[a.String()] = true
	}
	out := make(map[string]string)
	for ip, mac := range table {
		if want[ip] {
			out[ip] = mac
		}
	}
	return out
}
