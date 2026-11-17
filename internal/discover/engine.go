package discover

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/kr3s4l4/netdiag/internal/fingerprint"
	"github.com/kr3s4l4/netdiag/internal/model"
	"github.com/kr3s4l4/netdiag/internal/netutil"
	"github.com/kr3s4l4/netdiag/internal/snmp"
)

// Options controls one discovery run.
type Options struct {
	CIDR            string
	NetworkID       int64
	ScanID          int64
	Interface       string
	Gateway         string
	Profile         string
	Ports           []int
	Timeout         time.Duration
	Concurrency     int // liveness workers
	PortConcurrency int // TCP probes per host
	EnrichWorkers   int // hosts analysed in parallel
	MaxHosts        int
	ResolveDNS      bool
	BannerGrab      bool
	SNMP            bool
	SNMPConfig      snmp.Config
	AllowPublic     bool
	SkipPortScan    bool
	Trigger         string
}

// Normalize fills safe defaults.
func (o *Options) Normalize() error {
	o.CIDR = strings.TrimSpace(o.CIDR)
	if o.CIDR == "" {
		return errors.New("falta el CIDR objetivo")
	}
	if o.Timeout <= 0 {
		o.Timeout = 900 * time.Millisecond
	}
	if o.Timeout > 10*time.Second {
		o.Timeout = 10 * time.Second
	}
	if o.Concurrency < 1 {
		o.Concurrency = 128
	}
	if o.Concurrency > 2048 {
		o.Concurrency = 2048
	}
	if o.PortConcurrency < 1 {
		o.PortConcurrency = 256
	}
	if o.PortConcurrency > 1024 {
		o.PortConcurrency = 1024
	}
	if o.EnrichWorkers < 1 {
		o.EnrichWorkers = 8
	}
	if o.EnrichWorkers > 64 {
		o.EnrichWorkers = 64
	}
	if o.MaxHosts < 1 {
		o.MaxHosts = 4096
	}
	if o.Profile == "" {
		o.Profile = "common"
	}
	return nil
}

// Progress is a snapshot of engine state. Callbacks may be invoked
// concurrently and should throttle persistence themselves.
type Progress struct {
	Stage    string    `json:"stage"`
	Scanned  int       `json:"scanned"`
	Total    int       `json:"total"`
	Found    int       `json:"found"`
	Percent  float64   `json:"percent"`
	Current  string    `json:"current,omitempty"`
	Canceled bool      `json:"canceled"`
	Started  time.Time `json:"started_at"`
	Finished time.Time `json:"finished_at,omitempty"`
}

// Result is the outcome of a run. When Canceled is true, Hosts still contains
// every host confirmed before cancellation.
type Result struct {
	Hosts        []model.Host
	Total        int
	Scanned      int
	Found        int
	ARPEntries   int
	LocalSegment bool
	Canceled     bool
	Duration     time.Duration
	Progress     Progress
	Err          error
}

// Engine runs discovery. It is stateless and safe to reuse.
type Engine struct{}

// New creates an engine.
func New() *Engine { return &Engine{} }

// Run executes all phases until completion or cancellation. It never returns
// nil: on error the partial result is included.
func (e *Engine) Run(ctx context.Context, opts Options, onProgress func(Progress)) *Result {
	started := time.Now()
	res := &Result{Progress: Progress{Started: started, Stage: "validando"}}
	var progressMu sync.Mutex
	report := func(p Progress) {
		p.Started = started
		p.Percent = percent(p.Scanned, p.Total)
		progressMu.Lock()
		defer progressMu.Unlock()
		res.Progress = p
		if onProgress != nil {
			onProgress(p)
		}
	}
	fail := func(err error) *Result {
		res.Err = err
		res.Duration = time.Since(started)
		res.Progress.Stage = "error"
		res.Progress.Finished = time.Now()
		res.Canceled = errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
		res.Progress.Canceled = res.Canceled
		if onProgress != nil {
			onProgress(res.Progress)
		}
		return res
	}

	if err := opts.Normalize(); err != nil {
		return fail(err)
	}
	targets, err := netutil.HostsInCIDR(opts.CIDR, opts.MaxHosts)
	if err != nil {
		return fail(err)
	}
	prefix, err := netip.ParsePrefix(opts.CIDR)
	if err != nil {
		return fail(err)
	}
	if !prefix.Addr().Is4() {
		return fail(errors.New("solo se admite IPv4"))
	}
	if !opts.AllowPublic && !netutil.IsPrivate(net.ParseIP(prefix.Addr().String())) {
		return fail(errors.New("el CIDR es público: activa la opción de redes públicas para escanearlo"))
	}
	res.Total = len(targets)
	report(Progress{Stage: "validando", Total: len(targets)})

	ifaces, _ := netutil.Interfaces()
	ifaceName := opts.Interface
	if ifaceName == "" {
		ifaceName = interfaceForCIDR(ifaces, prefix)
	}
	gateway := opts.Gateway
	if gateway == "" && ifaceName != "" {
		gateway = netutil.GatewayFor(ifaceName, ifaces)
	}
	local := isLocalSegment(ifaces, prefix, ifaceName)
	res.LocalSegment = local
	own := ownIPv4(ifaces)
	for ip := range opts.ownExclusions() {
		own[ip] = true
	}

	// ---- Phase 1: L2 presence ----
	report(Progress{Stage: "descubriendo", Total: len(targets)})
	var arp map[string]string
	if local {
		if ARPAvailable() && ifaceName != "" {
			arp = ARPScan(ctx, ifaceName, targets, 2500*time.Millisecond)
		}
		nudge := NudgeARP(ctx, ifaceName, targets, 500*time.Millisecond)
		if len(arp) == 0 {
			arp = nudge
		} else {
			for ip, mac := range nudge {
				if _, ok := arp[ip]; !ok {
					arp[ip] = mac
				}
			}
		}
		arp = filterARP(arp, targets)
	}
	res.ARPEntries = len(arp)
	if ctx.Err() != nil {
		res.Canceled = true
	}

	// ---- Phase 2: liveness ----
	pinger, perr := netutil.OpenICMP()
	if perr != nil {
		pinger = nil
		res.Progress.Stage = "descubriendo"
	}
	if pinger != nil {
		defer pinger.Close()
	}

	var (
		mu       sync.Mutex
		alive    []model.Host
		scanned  int
		canceled bool
	)
	workers := opts.Concurrency
	done := make(chan struct{})
	go func() {
		t := time.NewTicker(150 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-t.C:
				mu.Lock()
				s, f := scanned, len(alive)
				mu.Unlock()
				report(Progress{Stage: "descubriendo", Scanned: s, Total: len(targets), Found: f})
			}
		}
	}()
	workerPool(ctx, len(targets), workers, func(wctx context.Context, i int) {
		addr := targets[i]
		ipStr := addr.String()
		result := AliveProbe{}
		mac := ""
		if arp != nil {
			mac = arp[ipStr]
		}
		switch {
		case own[ipStr]:
			// Never inventory the scanner itself.
			result = AliveProbe{Alive: false}
		case mac != "":
			result = AliveProbe{Alive: true, MAC: mac, Via: "arp"}
		default:
			ip := net.IP(addr.AsSlice())
			result = probeAlive(wctx, ip, "", pinger, opts.Timeout)
		}
		mu.Lock()
		scanned++
		if result.Alive {
			alive = append(alive, model.Host{
				IP:        ipStr,
				MAC:       result.MAC,
				Status:    model.StatusUp,
				LatencyMS: roundMS(result.RTT),
				TTL:       result.TTL,
				Evidence:  evidenceFor(result.Via),
				FirstSeen: time.Now(),
				LastSeen:  time.Now(),
			})
		}
		mu.Unlock()
	})
	close(done)
	if ctx.Err() != nil {
		canceled = true
	}
	// Late ARP refresh catches hosts discovered by TCP dials.
	if local {
		for ip, mac := range netutil.ARPTable() {
			if mac == "" {
				continue
			}
			for i := range alive {
				if alive[i].IP == ip && alive[i].MAC == "" {
					alive[i].MAC = mac
					alive[i].Evidence = append(alive[i].Evidence, evidenceFor("arp")...)
					break
				}
			}
		}
	}
	if len(alive) == 0 {
		res.Scanned = scanned
		res.Found = 0
		res.Canceled = canceled
		res.Duration = time.Since(started)
		res.Progress = Progress{Stage: doneStage(canceled), Scanned: scanned, Total: len(targets), Found: 0,
			Percent: percent(scanned, len(targets)), Canceled: canceled, Started: started, Finished: time.Now()}
		if onProgress != nil {
			onProgress(res.Progress)
		}
		return res
	}

	// ---- Phase 3: enrichment ----
	var collector *snmp.Collector
	if opts.SNMP {
		collector = snmp.New(opts.SNMPConfig)
		if !collector.Enabled() {
			collector = nil
		}
	}
	ports := fingerprint.ProfilePorts(opts.Profile, opts.Ports)
	if len(ports) == 0 {
		opts.SkipPortScan = true
	}
	gwSet := map[string]bool{}
	if gateway != "" {
		gwSet[gateway] = true
	}
	report(Progress{Stage: "identificando", Scanned: scanned, Total: len(targets), Found: len(alive)})

	enriched := make([]model.Host, len(alive))
	copy(enriched, alive)
	enrichWorkers := opts.EnrichWorkers
	if enrichWorkers > len(enriched) {
		enrichWorkers = len(enriched)
	}
	workerPool(ctx, len(enriched), enrichWorkers, func(wctx context.Context, i int) {
		h := enriched[i]
		if wctx.Err() != nil {
			return
		}
		isGateway := gwSet[h.IP]
		h.IsGateway = isGateway
		now := time.Now()
		h.NetworkID = opts.NetworkID
		h.LastScanID = opts.ScanID
		h.LastSeen = now
		if h.FirstSeen.IsZero() {
			h.FirstSeen = now
		}

		if !opts.SkipPortScan {
			open := ScanPorts(wctx, h.IP, ports, opts.Timeout, opts.PortConcurrency)
			if opts.BannerGrab && len(open) > 0 {
				open = EnrichBanners(wctx, h.IP, open, opts.Timeout, 8)
			}
			// A non-nil slice means ports were probed; nil is never returned.
			h.Ports = open
		}

		if opts.ResolveDNS && h.Hostname == "" {
			if name := netutil.LookupName(wctx, h.IP, 600*time.Millisecond); name != "" {
				h.Hostname = name
			}
		}
		if collector != nil {
			if sum, err := collector.Collect(wctx, h.IP); err == nil && sum != nil {
				h.SNMP = sum
				if sum.SysName != "" {
					h.Hostname = sum.SysName
				}
				if h.Vendor == "" {
					if v := snmp.VendorFromSysObjectID(sum.SysObjectID); v != "" {
						h.Vendor = v
					}
				}
			}
		}

		identity := fingerprint.Classify(fingerprint.Input{
			IP:        h.IP,
			MAC:       h.MAC,
			Hostname:  h.Hostname,
			Vendor:    h.Vendor,
			Ports:     h.Ports,
			SNMP:      h.SNMP,
			IsGateway: isGateway,
			TTL:       h.TTL,
		})
		h.Vendor = identity.Vendor
		h.DeviceType = identity.DeviceType
		h.Confidence = identity.Confidence
		h.OS = identity.OS
		if identity.Model != "" {
			h.Model = identity.Model
		}
		if identity.Hostname != "" {
			h.Hostname = identity.Hostname
		}
		h.Evidence = append(h.Evidence, identity.Evidence...)
		enriched[i] = h
	})

	if ctx.Err() != nil {
		canceled = true
	}
	mu.Lock()
	scanCount := scanned
	mu.Unlock()
	sort.Slice(enriched, func(i, j int) bool { return ipLess(enriched[i].IP, enriched[j].IP) })
	res.Hosts = enriched
	res.Scanned = scanCount
	res.Found = len(enriched)
	res.Canceled = canceled
	res.Duration = time.Since(started)
	res.Progress = Progress{
		Stage:    doneStage(canceled),
		Scanned:  scanCount,
		Total:    len(targets),
		Found:    len(enriched),
		Percent:  percent(scanCount, len(targets)),
		Canceled: canceled,
		Started:  started,
		Finished: time.Now(),
	}
	if onProgress != nil {
		onProgress(res.Progress)
	}
	return res
}

// ownExclusions is a hook for tests and future per-run exclusions.
func (o Options) ownExclusions() map[string]bool { return nil }

func doneStage(canceled bool) string {
	if canceled {
		return "cancelado"
	}
	return "completado"
}

func percent(done, total int) float64 {
	if total <= 0 {
		return 0
	}
	p := float64(done) / float64(total) * 100
	if p > 100 {
		p = 100
	}
	return float64(int(p*10)) / 10
}

func roundMS(d time.Duration) float64 {
	if d <= 0 {
		return 0
	}
	ms := float64(d.Microseconds()) / 1000
	return float64(int(ms*100)) / 100
}

func evidenceFor(via string) []string {
	switch via {
	case "arp":
		return []string{"Presencia confirmada por tabla ARP"}
	case "icmp":
		return []string{"Responde a ICMP echo"}
	case "tcp":
		return []string{"Responde en un puerto TCP"}
	}
	return nil
}

func interfaceForCIDR(ifaces []model.SystemInterface, prefix netip.Prefix) string {
	for _, ifc := range ifaces {
		if ifc.Loopback || !ifc.Up {
			continue
		}
		for _, a := range ifc.Addrs {
			if a.Family != "ipv4" {
				continue
			}
			p, err := netip.ParsePrefix(a.CIDR)
			if err != nil {
				continue
			}
			if p.Masked().Contains(prefix.Addr()) {
				return ifc.Name
			}
		}
	}
	for _, ifc := range ifaces {
		if ifc.IsPrimary && !ifc.Loopback {
			return ifc.Name
		}
	}
	return ""
}

func isLocalSegment(ifaces []model.SystemInterface, prefix netip.Prefix, iface string) bool {
	for _, ifc := range ifaces {
		if iface != "" && ifc.Name != iface {
			continue
		}
		if ifc.Loopback {
			continue
		}
		for _, a := range ifc.Addrs {
			if a.Family != "ipv4" {
				continue
			}
			p, err := netip.ParsePrefix(a.CIDR)
			if err != nil {
				continue
			}
			if p.Masked().Contains(prefix.Addr()) {
				return true
			}
		}
	}
	return false
}

func ownIPv4(ifaces []model.SystemInterface) map[string]bool {
	out := map[string]bool{}
	for _, ifc := range ifaces {
		for _, a := range ifc.Addrs {
			if a.Family == "ipv4" {
				out[a.IP] = true
			}
		}
	}
	return out
}

func ipLess(a, b string) bool {
	pa, errA := netip.ParseAddr(a)
	pb, errB := netip.ParseAddr(b)
	if errA == nil && errB == nil {
		return pa.Less(pb)
	}
	return a < b
}
