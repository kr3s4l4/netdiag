// Package discover implements the scan engine: L2 presence, ICMP sweep,
// port probing, banner grabbing, SNMP enrichment and persistence.
package discover

import (
	"context"
	"errors"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kr3s4l4/netdiag/internal/fingerprint"
	"github.com/kr3s4l4/netdiag/internal/model"
)

func isRefused(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "connection refused") || strings.Contains(s, "connection reset")
}

// ScanPorts runs a TCP connect scan and returns only open ports.
func ScanPorts(ctx context.Context, ip string, ports []int, timeout time.Duration, concurrency int) []model.Port {
	if concurrency < 1 {
		concurrency = 64
	}
	if concurrency > 512 {
		concurrency = 512
	}
	if timeout <= 0 {
		timeout = 900 * time.Millisecond
	}
	type result struct {
		port int
		open bool
	}
	jobs := make(chan int)
	results := make(chan result, concurrency)
	var wg sync.WaitGroup
	workers := concurrency
	if workers > len(ports) {
		workers = len(ports)
	}
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			d := net.Dialer{Timeout: timeout, KeepAlive: -1}
			for port := range jobs {
				if ctx.Err() != nil {
					return
				}
				cctx, cancel := context.WithTimeout(ctx, timeout)
				conn, err := d.DialContext(cctx, "tcp", net.JoinHostPort(ip, strconv.Itoa(port)))
				cancel()
				if err == nil {
					conn.Close()
					select {
					case results <- result{port: port, open: true}:
					case <-ctx.Done():
						return
					}
					continue
				}
				if isRefused(err) {
					select {
					case results <- result{port: port, open: false}:
					case <-ctx.Done():
						return
					}
				}
			}
		}()
	}
	go func() {
		defer close(jobs)
		for _, p := range ports {
			select {
			case jobs <- p:
			case <-ctx.Done():
				return
			}
		}
	}()
	go func() {
		wg.Wait()
		close(results)
	}()

	var out []model.Port
	for r := range results {
		if r.open {
			out = append(out, model.Port{
				Number:   r.port,
				Protocol: "tcp",
				State:    "open",
				Service:  fingerprint.ServiceName(r.port),
			})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Number < out[j].Number })
	return out
}

// EnrichBanners probes open ports in parallel and merges banner metadata.
func EnrichBanners(ctx context.Context, ip string, ports []model.Port, timeout time.Duration, concurrency int) []model.Port {
	if concurrency < 1 {
		concurrency = 8
	}
	if timeout <= 0 {
		timeout = 900 * time.Millisecond
	}
	out := make([]model.Port, len(ports))
	copy(out, ports)
	jobs := make(chan int)
	var wg sync.WaitGroup
	var mu sync.Mutex
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				if ctx.Err() != nil {
					return
				}
				p := fingerprint.Probe(ctx, ip, out[idx].Number, timeout)
				mu.Lock()
				if p.Service != "" {
					out[idx].Service = p.Service
				}
				if p.Product != "" {
					out[idx].Product = p.Product
				}
				if p.Version != "" {
					out[idx].Version = p.Version
				}
				if p.Banner != "" {
					out[idx].Banner = p.Banner
				}
				if p.TLS != nil {
					out[idx].TLS = p.TLS
				}
				mu.Unlock()
			}
		}()
	}
	for i := range out {
		select {
		case jobs <- i:
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			return out
		}
	}
	close(jobs)
	wg.Wait()
	return out
}

// AliveProbe is the result of a liveness check.
type AliveProbe struct {
	Alive bool
	MAC   string
	RTT   time.Duration
	TTL   int
	Via   string // icmp | arp | tcp
}

// probeAlive checks one address using ARP data, ICMP and finally TCP.
func probeAlive(ctx context.Context, ip net.IP, mac string, pinger interface {
	Ping(context.Context, net.IP, time.Duration) (time.Duration, int, error)
}, timeout time.Duration) AliveProbe {
	if mac != "" {
		return AliveProbe{Alive: true, MAC: mac, Via: "arp"}
	}
	if pinger != nil {
		if rtt, ttl, err := pinger.Ping(ctx, ip, timeout); err == nil {
			return AliveProbe{Alive: true, RTT: rtt, TTL: ttl, Via: "icmp"}
		}
	}
	// ARP requests issued by TCP dials also populate the neighbour cache.
	ports := []int{80, 443, 22, 445, 3389, 53, 8080}
	if ok, _ := tcpAlive(ctx, ip.String(), ports, timeout); ok {
		return AliveProbe{Alive: true, Via: "tcp"}
	}
	return AliveProbe{}
}

func tcpAlive(ctx context.Context, ip string, ports []int, timeout time.Duration) (bool, bool) {
	d := net.Dialer{Timeout: timeout, KeepAlive: -1}
	for _, port := range ports {
		if ctx.Err() != nil {
			return false, false
		}
		cctx, cancel := context.WithTimeout(ctx, timeout)
		conn, err := d.DialContext(cctx, "tcp", net.JoinHostPort(ip, strconv.Itoa(port)))
		cancel()
		if err == nil {
			conn.Close()
			return true, true
		}
		if isRefused(err) {
			return true, true
		}
	}
	return false, false
}

// errCanceled is a sentinel used by the engine.
var errCanceled = errors.New("escaneo cancelado")

// workerPool runs fn for each index with bounded concurrency and cancels on
// context cancellation.
func workerPool(ctx context.Context, n, workers int, fn func(ctx context.Context, i int)) {
	if workers < 1 {
		workers = 1
	}
	ch := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range ch {
				if ctx.Err() != nil {
					return
				}
				fn(ctx, i)
			}
		}()
	}
	for i := 0; i < n; i++ {
		select {
		case ch <- i:
		case <-ctx.Done():
			close(ch)
			wg.Wait()
			return
		}
	}
	close(ch)
	wg.Wait()
}

// atomicInt is a tiny helper for progress counters.
type atomicInt struct{ v atomic.Int64 }

func (a *atomicInt) Add(n int64) int64 { return a.v.Add(n) }
func (a *atomicInt) Load() int64       { return a.v.Load() }
