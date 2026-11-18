// Package neighbors discovers private networks reachable through local
// gateways. It uses unprivileged UDP traceroutes, so it needs no root and no
// external binaries. Only RFC1918 candidate addresses are probed.
package neighbors

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/kr3s4l4/netdiag/internal/netutil"
	"github.com/kr3s4l4/netdiag/internal/trace"
)

// Network is one confirmed neighboring subnet.
type Network struct {
	CIDR           string    `json:"cidr"`
	RouterIP       string    `json:"router_ip"`
	RouterHostname string    `json:"router_hostname,omitempty"`
	Path           []string  `json:"path"`
	Evidence       string    `json:"evidence"`
	Confidence     int       `json:"confidence"`
	DetectedAt     time.Time `json:"detected_at"`
}

// Result is the outcome of one discovery pass.
type Result struct {
	Networks      []Network `json:"networks"`
	LocalNetworks []string  `json:"local_networks"`
	Candidates    int       `json:"candidates"`
	DurationMS    float64   `json:"duration_ms"`
	RunAt         time.Time `json:"run_at"`
}

// Discover probes likely gateway addresses outside the local subnets and
// returns any private network revealed by the traced path.
func Discover(ctx context.Context, localCIDRs, seedIPs []string) *Result {
	started := time.Now()
	local := normalizeCIDRs(localCIDRs)
	seeds := cleanIPs(seedIPs)
	candidates := candidateGateways(local, seeds)

	res := &Result{
		Networks:      []Network{},
		LocalNetworks: sortedKeys(local),
		Candidates:    len(candidates),
		RunAt:         started,
	}

	var (
		mu       sync.Mutex
		wg       sync.WaitGroup
		sem      = make(chan struct{}, 8)
		byCIDR   = map[string]Network{}
		localPfx = localPrefixes(local)
	)
	for _, candidate := range candidates {
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		go func(ip string) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			found := inspectCandidate(ctx, ip, local, localPfx)
			if len(found) == 0 {
				return
			}
			mu.Lock()
			defer mu.Unlock()
			for _, n := range found {
				existing, ok := byCIDR[n.CIDR]
				if !ok || n.Confidence > existing.Confidence {
					byCIDR[n.CIDR] = n
				}
			}
		}(candidate)
	}
	wg.Wait()

	for _, n := range byCIDR {
		res.Networks = append(res.Networks, n)
	}
	sort.Slice(res.Networks, func(i, j int) bool {
		return res.Networks[i].CIDR < res.Networks[j].CIDR
	})
	res.DurationMS = float64(time.Since(started).Microseconds()) / 1000
	return res
}

// inspectCandidate traces one address and extracts foreign private subnets
// visible in the path.
func inspectCandidate(ctx context.Context, candidate string, local map[string]bool, localPfx []netip.Prefix) []Network {
	ip := net.ParseIP(candidate)
	if ip == nil || !netutil.IsPrivate(ip) {
		return nil
	}
	if belongsTo(ip, localPfx) {
		return nil
	}
	callCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	r, err := trace.Run(callCtx, candidate, trace.Options{
		MaxHops: 4,
		Probes:  1,
		Timeout: 450 * time.Millisecond,
		Resolve: true,
	})
	if err != nil || r == nil {
		return nil
	}

	out := []Network{}
	seen := map[string]bool{}
	for _, h := range r.Hops {
		if h.IP == "" || h.IP == "*" {
			continue
		}
		hopIP := net.ParseIP(h.IP)
		if hopIP == nil || !netutil.IsPrivate(hopIP) || belongsTo(hopIP, localPfx) {
			continue
		}
		prefix := netip.PrefixFrom(netip.MustParseAddr(hopIP.String()).Unmap(), 24).Masked()
		cidr := prefix.String()
		if local[cidr] || seen[cidr] {
			continue
		}
		seen[cidr] = true
		confidence := 60
		evidence := "salto intermedio detectado en la traza"
		if h.Reached || hopIP.Equal(ip) {
			confidence = 85
			evidence = "router respondió a la traza"
		}
		v4 := hopIP.To4()
		if v4 != nil && (v4[3] == 1 || v4[3] == 254) {
			confidence += 5
			evidence = "puerta de enlace de la red vecina respondió a la traza"
		}
		path := make([]string, 0, len(r.Hops))
		for _, p := range r.Hops {
			path = append(path, p.IP)
		}
		out = append(out, Network{
			CIDR:           cidr,
			RouterIP:       h.IP,
			RouterHostname: strings.TrimSuffix(h.Hostname, "."),
			Path:           path,
			Evidence:       evidence,
			Confidence:     confidence,
			DetectedAt:     time.Now(),
		})
	}
	return out
}

// candidateGateways builds a short, private-only list of likely router IPs.
func candidateGateways(local map[string]bool, seeds []string) []string {
	seen := map[string]bool{}
	add := func(ip string) {
		p := net.ParseIP(ip)
		if p == nil || p.To4() == nil || !netutil.IsPrivate(p) || seen[ip] {
			return
		}
		cidr := subnetOf(p)
		if local[cidr] {
			return
		}
		seen[ip] = true
	}

	for cidr := range local {
		p, err := netip.ParsePrefix(cidr)
		if err != nil {
			continue
		}
		a := p.Addr().As4()
		base := [4]byte{a[0], a[1], a[2], 0}
		for _, off := range []int{-2, -1, 1, 2} {
			n := int(base[2]) + off
			if n < 0 || n > 255 {
				continue
			}
			for _, host := range []byte{1, 254} {
				addr := base
				addr[2] = byte(n)
				addr[3] = host
				add(net.IP(addr[:]).String())
			}
		}
		// Common alternative private layout on the same router.
		if a[0] == 192 && a[1] == 168 {
			add("10.0.0.1")
			add("10.0.1.1")
			add("172.16.0.1")
		}
	}
	for _, seed := range seeds {
		ip := net.ParseIP(seed)
		if ip == nil || ip.To4() == nil || !netutil.IsPrivate(ip) {
			continue
		}
		add(seed)
		v4 := ip.To4()
		add(fmt.Sprintf("%d.%d.%d.1", v4[0], v4[1], v4[2]))
	}

	out := make([]string, 0, len(seen))
	for ip := range seen {
		out = append(out, ip)
	}
	sort.Strings(out)
	return out
}

func normalizeCIDRs(values []string) map[string]bool {
	out := map[string]bool{}
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		p, err := netip.ParsePrefix(v)
		if err != nil || !p.Addr().Is4() {
			continue
		}
		bits := p.Bits()
		if bits < 24 {
			bits = 24
		}
		out[netip.PrefixFrom(p.Addr().Unmap(), bits).Masked().String()] = true
	}
	return out
}

func localPrefixes(local map[string]bool) []netip.Prefix {
	out := make([]netip.Prefix, 0, len(local))
	for cidr := range local {
		if p, err := netip.ParsePrefix(cidr); err == nil {
			out = append(out, p)
		}
	}
	return out
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func cleanIPs(values []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" || seen[v] {
			continue
		}
		if ip := net.ParseIP(v); ip != nil && ip.To4() != nil && netutil.IsPrivate(ip) {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

func subnetOf(ip net.IP) string {
	v4 := ip.To4()
	if v4 == nil {
		return ""
	}
	return fmt.Sprintf("%d.%d.%d.0/24", v4[0], v4[1], v4[2])
}

func belongsTo(ip net.IP, prefixes []netip.Prefix) bool {
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return false
	}
	addr = addr.Unmap()
	for _, p := range prefixes {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}
