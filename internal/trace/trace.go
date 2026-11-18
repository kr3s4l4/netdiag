// Package trace implements unprivileged ICMP traceroute using UDP probes and
// Linux IP_RECVERR. No raw sockets or external binaries are required, so it
// works in the same portable builds as the scanner.
package trace

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"sort"
	"syscall"
	"time"
	"unsafe"
)

// Hop is one hop in a traced path.
type Hop struct {
	TTL      int       `json:"ttl"`
	IP       string    `json:"ip,omitempty"`
	Hostname string    `json:"hostname,omitempty"`
	RTTMs    []float64 `json:"rtt_ms"`
	Loss     int       `json:"loss"`
	Reached  bool      `json:"reached"`
}

// Result is a complete trace to one target.
type Result struct {
	Target    string    `json:"target"`
	TargetIP  string    `json:"target_ip"`
	Hops      []Hop     `json:"hops"`
	Probes    int       `json:"probes"`
	MaxHops   int       `json:"max_hops"`
	ElapsedMS float64   `json:"elapsed_ms"`
	StartedAt time.Time `json:"started_at"`
}

// Options controls one trace.
type Options struct {
	MaxHops int
	Probes  int
	Timeout time.Duration
	Resolve bool
}

// Normalize fills safe defaults.
func (o *Options) Normalize() {
	if o.MaxHops <= 0 {
		o.MaxHops = 20
	}
	if o.MaxHops > 64 {
		o.MaxHops = 64
	}
	if o.Probes <= 0 {
		o.Probes = 2
	}
	if o.Probes > 5 {
		o.Probes = 5
	}
	if o.Timeout <= 0 {
		o.Timeout = 1200 * time.Millisecond
	}
}

// Run traces the path to target.
func Run(ctx context.Context, target string, opts Options) (*Result, error) {
	opts.Normalize()
	ip := net.ParseIP(target)
	if ip == nil {
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, target)
		if err != nil || len(ips) == 0 {
			return nil, fmt.Errorf("no se pudo resolver %q: %w", target, err)
		}
		for _, candidate := range ips {
			if v4 := candidate.IP.To4(); v4 != nil {
				ip = v4
				break
			}
		}
	}
	if ip == nil || ip.To4() == nil {
		return nil, errors.New("la traza solo admite IPv4")
	}
	v4 := ip.To4()

	started := time.Now()
	res := &Result{
		Target:    target,
		TargetIP:  v4.String(),
		Probes:    opts.Probes,
		MaxHops:   opts.MaxHops,
		StartedAt: started,
		Hops:      []Hop{},
	}

	for ttl := 1; ttl <= opts.MaxHops; ttl++ {
		if ctx.Err() != nil {
			break
		}
		hop := Hop{TTL: ttl, RTTMs: []float64{}}
		port := 33434 + ttl
		for probe := 0; probe < opts.Probes; probe++ {
			if ctx.Err() != nil {
				break
			}
			offender, rtt, reached, err := probeHop(ctx, v4, port+probe, ttl, opts.Timeout)
			if err != nil {
				hop.Loss++
				continue
			}
			if offender != "" {
				hop.IP = offender
			}
			hop.RTTMs = append(hop.RTTMs, float64(rtt.Microseconds())/1000)
			if reached {
				hop.Reached = true
			}
		}
		if hop.IP != "" {
			if len(hop.RTTMs) == 0 {
				hop.Loss = opts.Probes
			} else {
				hop.Loss = opts.Probes - len(hop.RTTMs)
			}
			if hop.Loss < 0 {
				hop.Loss = 0
			}
			if opts.Resolve {
				hop.Hostname = resolveName(ctx, hop.IP)
			}
			res.Hops = append(res.Hops, hop)
		} else {
			hop.Loss = opts.Probes
			hop.IP = "*"
			res.Hops = append(res.Hops, hop)
		}
		if hop.Reached {
			break
		}
	}

	res.ElapsedMS = float64(time.Since(started).Microseconds()) / 1000
	return res, nil
}

// probeHop sends one UDP datagram with the given TTL and waits for the ICMP
// error it produces. It returns the router that answered, the RTT and whether
// the target itself was reached.
func probeHop(ctx context.Context, target net.IP, port, ttl int, timeout time.Duration) (string, time.Duration, bool, error) {
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_DGRAM, syscall.IPPROTO_UDP)
	if err != nil {
		return "", 0, false, err
	}
	defer syscall.Close(fd)
	if err := syscall.SetsockoptInt(fd, syscall.SOL_IP, syscall.IP_RECVERR, 1); err != nil {
		return "", 0, false, err
	}
	if err := syscall.SetsockoptInt(fd, syscall.IPPROTO_IP, syscall.IP_TTL, ttl); err != nil {
		return "", 0, false, err
	}
	payload := []byte(fmt.Sprintf("netdiag-trace-%d-%d-%d", ttl, port, time.Now().UnixNano()))
	sa := &syscall.SockaddrInet4{Port: port}
	copy(sa.Addr[:], target.To4())
	start := time.Now()
	if err := syscall.Sendto(fd, payload, 0, sa); err != nil {
		return "", 0, false, err
	}

	buf := make([]byte, 1500)
	oob := make([]byte, 512)
	deadline := start.Add(timeout)
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return "", 0, false, ctx.Err()
		}
		n, oobn, _, _, err := syscall.Recvmsg(fd, buf, oob, syscall.MSG_ERRQUEUE|syscall.MSG_DONTWAIT)
		if err != nil {
			time.Sleep(5 * time.Millisecond)
			continue
		}
		_ = n
		offender, icmpType, icmpCode, err := parseError(oob[:oobn])
		if err != nil || offender == nil {
			continue
		}
		rtt := time.Since(start)
		reached := icmpType == 3 && (icmpCode == 3 || offender.Equal(target))
		return offender.String(), rtt, reached, nil
	}
	return "", 0, false, errors.New("sin respuesta ICMP")
}

// parseError extracts the offending address and ICMP type from IP_RECVERR
// control messages.
func parseError(oob []byte) (net.IP, int, int, error) {
	headerSize := int(unsafe.Sizeof(syscall.Cmsghdr{}))
	align := int(unsafe.Alignof(syscall.Cmsghdr{}))
	for i := 0; i+headerSize <= len(oob); {
		raw := oob[i:]
		var length int
		if headerSize == 16 {
			length = int(binary.NativeEndian.Uint64(raw[0:8]))
		} else {
			length = int(binary.NativeEndian.Uint32(raw[0:4]))
		}
		if length < headerSize || i+length > len(oob) {
			break
		}
		var level, typ int
		if headerSize == 16 {
			level = int(int32(binary.NativeEndian.Uint32(raw[8:12])))
			typ = int(int32(binary.NativeEndian.Uint32(raw[12:16])))
		} else {
			level = int(int32(binary.NativeEndian.Uint32(raw[4:8])))
			typ = int(int32(binary.NativeEndian.Uint32(raw[8:12])))
		}
		data := raw[headerSize:length]
		if level == syscall.SOL_IP && typ == syscall.IP_RECVERR && len(data) >= 16 {
			origin := data[4]
			icmpType := int(data[5])
			icmpCode := int(data[6])
			if origin == 2 || origin == 3 { // SO_EE_ORIGIN_ICMP / LOCAL
				if len(data) >= 28 && binary.NativeEndian.Uint16(data[16:18]) == syscall.AF_INET {
					return net.IPv4(data[20], data[21], data[22], data[23]), icmpType, icmpCode, nil
				}
				return nil, icmpType, icmpCode, errors.New("sin dirección en el error extendido")
			}
		}
		i += (length + align - 1) &^ (align - 1)
	}
	return nil, 0, 0, errors.New("sin mensaje IP_RECVERR")
}

// resolveName does a best-effort reverse lookup that never blocks the trace.
func resolveName(ctx context.Context, ip string) string {
	ctx, cancel := context.WithTimeout(ctx, 250*time.Millisecond)
	defer cancel()
	names, err := net.DefaultResolver.LookupAddr(ctx, ip)
	if err != nil || len(names) == 0 {
		return ""
	}
	sort.Strings(names)
	return names[0]
}
