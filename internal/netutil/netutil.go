// Package netutil provides the network primitives used by discovery:
// interface enumeration, per-interface gateways, CIDR iteration, ICMP echo,
// ARP cache reading and small address helpers.
package netutil

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kr3s4l4/netdiag/internal/model"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

// ErrNoICMP means neither raw nor unprivileged ICMP sockets are available.
var ErrNoICMP = errors.New("no hay soporte ICMP (faltan capabilities o ping_group_range)")

// Interfaces enumerates local NICs with addresses, primary flag and gateway.
func Interfaces() ([]model.SystemInterface, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	gateways := gatewaysByIface()
	primary := primaryInterface()
	var out []model.SystemInterface
	for _, ifc := range ifaces {
		si := model.SystemInterface{
			Name:      ifc.Name,
			Index:     ifc.Index,
			MAC:       normalizeHardware(ifc.HardwareAddr.String()),
			MTU:       ifc.MTU,
			Up:        ifc.Flags&net.FlagUp != 0,
			Loopback:  ifc.Flags&net.FlagLoopback != 0,
			IsPrimary: ifc.Name == primary,
			Gateway:   gateways[ifc.Name],
			Addrs:     []model.InterfaceAddr{},
		}
		addrs, err := ifc.Addrs()
		if err == nil {
			for _, a := range addrs {
				ipn, ok := a.(*net.IPNet)
				if !ok {
					continue
				}
				ip := ipn.IP
				if ip.IsLinkLocalUnicast() && ip.To4() == nil {
					continue // skip IPv6 link-local noise
				}
				family := "ipv6"
				if ip.To4() != nil {
					family = "ipv4"
				}
				si.Addrs = append(si.Addrs, model.InterfaceAddr{
					IP:      ip.String(),
					CIDR:    ipn.String(),
					Family:  family,
					Private: IsPrivate(ip),
				})
			}
		}
		if len(si.Addrs) == 0 {
			continue
		}
		out = append(out, si)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].IsPrimary != out[j].IsPrimary {
			return out[i].IsPrimary
		}
		if out[i].Up != out[j].Up {
			return out[i].Up
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

func normalizeHardware(s string) string {
	if s == "" {
		return ""
	}
	hw, err := net.ParseMAC(s)
	if err != nil {
		return s
	}
	return strings.ToLower(hw.String())
}

func primaryInterface() string {
	f, err := os.Open("/proc/net/route")
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Scan() // header
	best := ""
	bestMetric := int64(1 << 30)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 8 || fields[1] != "00000000" {
			continue
		}
		flags, _ := strconv.ParseInt(fields[3], 16, 64)
		if flags&0x2 == 0 { // RTF_GATEWAY
			continue
		}
		metric, _ := strconv.ParseInt(fields[6], 10, 64)
		if metric < bestMetric {
			best, bestMetric = fields[0], metric
		}
	}
	return best
}

// gatewaysByIface reads the IPv4 default route per interface.
func gatewaysByIface() map[string]string {
	out := map[string]string{}
	f, err := os.Open("/proc/net/route")
	if err != nil {
		return out
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Scan()
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 8 || fields[1] != "00000000" {
			continue
		}
		flags, _ := strconv.ParseInt(fields[3], 16, 64)
		if flags&0x2 == 0 {
			continue
		}
		gw := parseHexIPv4(fields[2])
		if gw != "" {
			out[fields[0]] = gw
		}
	}
	return out
}

func parseHexIPv4(s string) string {
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return ""
	}
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, uint32(v))
	return net.IP(b).String()
}

// GatewayFor returns the IPv4 default gateway for a named interface, or the
// interface's own IP when the interface has no default route (bridged/container
// networks). This avoids attaching nodes to the physical NIC's gateway.
func GatewayFor(ifaceName string, ifaces []model.SystemInterface) string {
	for _, i := range ifaces {
		if i.Name != ifaceName {
			continue
		}
		if i.Gateway != "" {
			return i.Gateway
		}
		for _, a := range i.Addrs {
			if a.Family == "ipv4" && a.Private {
				return a.IP
			}
		}
	}
	return ""
}

// SuggestNetworks derives scan targets from private IPv4 interface addresses.
// Prefixes wider than /24 are reduced to /24 to keep sweeps fast and useful.
func SuggestNetworks() ([]model.Network, error) {
	ifaces, err := Interfaces()
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var out []model.Network
	for _, ifc := range ifaces {
		if ifc.Loopback || !ifc.Up {
			continue
		}
		for _, a := range ifc.Addrs {
			if a.Family != "ipv4" || !a.Private {
				continue
			}
			p, err := netip.ParsePrefix(a.CIDR)
			if err != nil {
				continue
			}
			if p.Bits() < 24 {
				p = netip.PrefixFrom(p.Masked().Addr(), 24)
			}
			cidr := p.String()
			if seen[cidr] {
				continue
			}
			seen[cidr] = true
			out = append(out, model.Network{
				Name:      ifc.Name + " · " + cidr,
				CIDR:      cidr,
				Interface: ifc.Name,
				Enabled:   true,
				IsLocal:   true,
			})
		}
	}
	return out, nil
}

// IsPrivate reports whether ip is loopback, RFC1918, CGNAT or link-local.
func IsPrivate(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}
	if v4 := ip.To4(); v4 != nil {
		// 100.64.0.0/10 (CGNAT)
		return v4[0] == 100 && v4[1] >= 64 && v4[1] <= 127
	}
	return false
}

// IsPublicUnicast reports whether the address is a global unicast address.
func IsPublicUnicast(ip net.IP) bool {
	return ip != nil && !IsPrivate(ip) && !ip.IsUnspecified() && !ip.IsMulticast()
}

// HostsInCIDR expands a CIDR into scannable IPv4 addresses. Network and
// broadcast addresses are skipped for prefixes <= /30. It enforces limit.
func HostsInCIDR(cidr string, limit int) ([]netip.Addr, error) {
	p, err := netip.ParsePrefix(strings.TrimSpace(cidr))
	if err != nil {
		return nil, fmt.Errorf("CIDR inválido %q: %w", cidr, err)
	}
	if !p.Addr().Is4() {
		return nil, fmt.Errorf("solo se admite IPv4: %s", cidr)
	}
	p = p.Masked()
	count := 1 << (32 - p.Bits())
	if count > 1<<20 {
		return nil, fmt.Errorf("CIDR demasiado grande: %s", cidr)
	}
	skipEdges := p.Bits() <= 30
	out := make([]netip.Addr, 0, count)
	addr := p.Addr()
	for i := 0; i < count; i++ {
		if !(skipEdges && (i == 0 || i == count-1)) {
			out = append(out, addr)
		}
		if len(out) > limit {
			return nil, fmt.Errorf("el CIDR %s excede el máximo de %d hosts", cidr, limit)
		}
		addr = addr.Next()
	}
	return out, nil
}

// CountHosts returns the number of usable hosts in a CIDR.
func CountHosts(cidr string) (int, error) {
	p, err := netip.ParsePrefix(strings.TrimSpace(cidr))
	if err != nil {
		return 0, err
	}
	if !p.Addr().Is4() {
		return 0, fmt.Errorf("solo IPv4")
	}
	bits := 32 - p.Bits()
	if bits > 20 {
		return 0, fmt.Errorf("CIDR demasiado grande")
	}
	n := 1 << bits
	if p.Bits() <= 30 && n >= 2 {
		n -= 2
	}
	return n, nil
}

// ARPEntry is one neighbour cache row.
type ARPEntry struct {
	IP  string
	MAC string
}

// ARPTable reads the kernel IPv4 neighbour cache.
func ARPTable() map[string]string {
	out := map[string]string{}
	f, err := os.Open("/proc/net/arp")
	if err != nil {
		return out
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Scan()
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 4 {
			continue
		}
		mac := strings.ToLower(fields[3])
		if mac == "00:00:00:00:00:00" || mac == "" {
			continue
		}
		out[fields[0]] = mac
	}
	return out
}

// LookupName resolves a PTR with a hard timeout.
func LookupName(ctx context.Context, ip string, timeout time.Duration) string {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	names, err := net.DefaultResolver.LookupAddr(ctx, ip)
	if err != nil || len(names) == 0 {
		return ""
	}
	return strings.TrimSuffix(names[0], ".")
}

// LocalHostname returns the system hostname.
func LocalHostname() string {
	h, _ := os.Hostname()
	return h
}

// OSName reads /etc/os-release PRETTY_NAME.
func OSName() string {
	f, err := os.Open("/etc/os-release")
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if strings.HasPrefix(sc.Text(), "PRETTY_NAME=") {
			return strings.Trim(strings.TrimPrefix(sc.Text(), "PRETTY_NAME="), `"`)
		}
	}
	return ""
}

// Uptime returns the system uptime.
func Uptime() time.Duration {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return 0
	}
	secs, _ := strconv.ParseFloat(fields[0], 64)
	return time.Duration(secs * float64(time.Second))
}

// --- ICMP ---

type pingWaiter struct {
	ch chan pingReply
}

type pingReply struct {
	rtt time.Duration
	ttl int
	err error
}

// ICMP is a concurrent echo requester sharing one socket.
type ICMP struct {
	pc      *icmp.PacketConn
	v4      *ipv4.PacketConn
	id      int
	seq     atomic.Uint32
	mu      sync.Mutex
	pending map[int]*pingWaiter
	closed  atomic.Bool
	mode    string
}

// OpenICMP tries a raw socket first, then unprivileged ICMP.
func OpenICMP() (*ICMP, error) {
	var (
		pc   *icmp.PacketConn
		mode string
		err  error
	)
	for _, candidate := range []string{"ip4:icmp", "udp4"} {
		pc, err = icmp.ListenPacket(candidate, "0.0.0.0")
		if err == nil {
			mode = candidate
			break
		}
	}
	if pc == nil {
		return nil, fmt.Errorf("%w: %v", ErrNoICMP, err)
	}
	p := &ICMP{
		pc:      pc,
		id:      os.Getpid() & 0xffff,
		pending: map[int]*pingWaiter{},
		mode:    mode,
	}
	if v4 := pc.IPv4PacketConn(); v4 != nil {
		_ = v4.SetControlMessage(ipv4.FlagTTL, true)
		p.v4 = v4
	}
	go p.readLoop()
	return p, nil
}

// Mode returns the socket mode ("ip4:icmp" or "udp4").
func (p *ICMP) Mode() string { return p.mode }

// Close stops the reader.
func (p *ICMP) Close() error {
	p.closed.Store(true)
	return p.pc.Close()
}

func (p *ICMP) readLoop() {
	buf := make([]byte, 1500)
	for {
		if p.closed.Load() {
			return
		}
		_ = p.pc.SetReadDeadline(time.Now().Add(2 * time.Second))
		var (
			n   int
			ttl int
			err error
		)
		if p.v4 != nil {
			var cm *ipv4.ControlMessage
			n, cm, _, err = p.v4.ReadFrom(buf)
			if cm != nil {
				ttl = cm.TTL
			}
		} else {
			n, _, err = p.pc.ReadFrom(buf)
		}
		if err != nil {
			var ne net.Error
			if errors.As(err, &ne) && ne.Timeout() {
				continue
			}
			if p.closed.Load() {
				return
			}
			continue
		}
		msg, err := icmp.ParseMessage(1, buf[:n])
		if err != nil {
			continue
		}
		echo, ok := msg.Body.(*icmp.Echo)
		if !ok {
			continue
		}
		seq := int(echo.Seq)
		p.mu.Lock()
		w, ok := p.pending[seq]
		if ok {
			delete(p.pending, seq)
		}
		p.mu.Unlock()
		if ok && msg.Type == ipv4.ICMPTypeEchoReply {
			// RTT is measured by the waiting goroutine via sentAt.
			select {
			case w.ch <- pingReply{ttl: ttl}:
			default:
			}
		}
	}
}

// Ping sends one echo request. It returns RTT and TTL (0 when unknown).
func (p *ICMP) Ping(ctx context.Context, ip net.IP, timeout time.Duration) (time.Duration, int, error) {
	if ip == nil || ip.To4() == nil {
		return 0, 0, errors.New("ICMP requiere IPv4")
	}
	if timeout <= 0 {
		timeout = time.Second
	}
	seq := int(p.seq.Add(1) & 0x7fff)
	w := &pingWaiter{ch: make(chan pingReply, 1)}
	p.mu.Lock()
	p.pending[seq] = w
	p.mu.Unlock()
	defer func() {
		p.mu.Lock()
		delete(p.pending, seq)
		p.mu.Unlock()
	}()

	msg := icmp.Message{
		Type: ipv4.ICMPTypeEcho,
		Code: 0,
		Body: &icmp.Echo{
			ID:   p.id,
			Seq:  seq,
			Data: []byte("netdiag-probe-0123456789"),
		},
	}
	wire, err := msg.Marshal(nil)
	if err != nil {
		return 0, 0, err
	}
	start := time.Now()
	var dst net.Addr = &net.IPAddr{IP: ip.To4()}
	if p.mode == "udp4" {
		dst = &net.UDPAddr{IP: ip.To4()}
	}
	if _, err := p.pc.WriteTo(wire, dst); err != nil {
		return 0, 0, err
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return 0, 0, ctx.Err()
	case <-timer.C:
		return 0, 0, context.DeadlineExceeded
	case r := <-w.ch:
		return time.Since(start), r.ttl, nil
	}
}

// TCPPing reports whether a TCP handshake can be established on any of ports.
// Closed ports that answer RST also prove the host is alive (reset == true).
func TCPPing(ctx context.Context, ip string, ports []int, timeout time.Duration) (bool, bool) {
	d := net.Dialer{Timeout: timeout}
	for _, port := range ports {
		conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(ip, strconv.Itoa(port)))
		if err == nil {
			conn.Close()
			return true, true
		}
		var opErr *net.OpError
		if errors.As(err, &opErr) {
			if strings.Contains(strings.ToLower(opErr.Err.Error()), "refused") {
				return true, true
			}
		}
		if ctx.Err() != nil {
			return false, false
		}
	}
	return false, false
}

// Dialer returns a dialer with a bounded timeout.
func Dialer(timeout time.Duration) *net.Dialer {
	if timeout <= 0 {
		timeout = 900 * time.Millisecond
	}
	return &net.Dialer{Timeout: timeout, KeepAlive: -1}
}
