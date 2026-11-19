// Package capture implements the optional F47 passive listener. It records
// metadata only: no payload, credential or DNS answer is ever persisted. In
// portable builds every entry point degrades to an explicit "unavailable"
// result so the rest of NetDiag keeps working without raw-socket privileges.
package capture

import (
	"errors"
	"time"

	"github.com/kr3s4l4/netdiag/internal/model"
)

// ErrUnavailable is returned by portable builds and by hosts without libpcap.
var ErrUnavailable = errors.New("captura pasiva no disponible: compila con -tags pcap y concede CAP_NET_RAW")

// Config controls one short, bounded capture window.
type Config struct {
	Interface   string
	Duration    time.Duration
	MaxPackets  int
	Promiscuous bool
	BPFFilter   string
}

// Result summarises a capture attempt. Error is descriptive; callers should
// never fail a scan because passive capture was unavailable or rate limited.
type Result struct {
	Available    bool                       `json:"available"`
	Packets      int                        `json:"packets"`
	Observed     int                        `json:"observed"`
	Duration     string                     `json:"duration,omitempty"`
	BPFFilter    string                     `json:"bpf_filter,omitempty"`
	Error        string                     `json:"error,omitempty"`
	Observations []model.PassiveObservation `json:"-"`
}

// DefaultFilter keeps the listener narrow: ARP, ICMP, DHCP, DNS and TCP SYNs.
// It never selects TCP payload traffic or full application streams.
const DefaultFilter = "arp or icmp or (udp and (port 67 or port 68 or port 53)) or (tcp and tcp[tcpflags] & tcp-syn != 0)"

// Normalize fills safe bounds for one capture window.
func (c *Config) Normalize() {
	if c.Duration <= 0 {
		c.Duration = 10 * time.Second
	}
	if c.Duration > 5*time.Minute {
		c.Duration = 5 * time.Minute
	}
	if c.MaxPackets <= 0 {
		c.MaxPackets = 2000
	}
	if c.MaxPackets > 100000 {
		c.MaxPackets = 100000
	}
	if c.BPFFilter == "" {
		c.BPFFilter = DefaultFilter
	}
}
