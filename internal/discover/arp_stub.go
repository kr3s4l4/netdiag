//go:build !pcap

package discover

import (
	"context"
	"net/netip"
	"time"
)

// ARPAvailable reports whether active L2 ARP scanning is compiled in.
func ARPAvailable() bool { return false }

// ARPScan is a no-op in portable builds. The kernel neighbour table populated
// by NudgeARP is used instead.
func ARPScan(ctx context.Context, iface string, ips []netip.Addr, timeout time.Duration) map[string]string {
	return nil
}
