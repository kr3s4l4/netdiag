//go:build pcap

package capture

import (
	"context"
	"fmt"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/pcap"
	"github.com/kr3s4l4/netdiag/internal/model"
)

// Available reports whether this binary can open a live capture handle.
func Available() bool { return true }

// Capture listens for a bounded window on iface and streams metadata-only
// observations to onObservation. Missing privileges are reported, never fatal.
func Capture(ctx context.Context, cfg Config, onObservation func(model.PassiveObservation)) (*Result, error) {
	cfg.Normalize()
	res := &Result{Available: true, BPFFilter: cfg.BPFFilter, Duration: cfg.Duration.String()}
	if cfg.Interface == "" {
		res.Error = ErrUnavailable.Error() + " (sin interfaz)"
		return res, ErrUnavailable
	}
	handle, err := pcap.OpenLive(cfg.Interface, 65536, cfg.Promiscuous, 200*time.Millisecond)
	if err != nil {
		res.Error = err.Error()
		return res, err
	}
	defer handle.Close()
	if err := handle.SetBPFFilter(cfg.BPFFilter); err != nil {
		res.Error = fmt.Sprintf("BPF rechazado: %v", err)
		return res, err
	}

	source := gopacket.NewPacketSource(handle, handle.LinkType())
	source.NoCopy = true
	packets := source.Packets()
	ctx, cancel := context.WithTimeout(ctx, cfg.Duration)
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			return res, nil
		case pkt, ok := <-packets:
			if !ok {
				return res, nil
			}
			res.Packets++
			for _, obs := range Observe(pkt, cfg.Interface) {
				onObservation(obs)
				res.Observed++
			}
			if res.Packets >= cfg.MaxPackets {
				return res, nil
			}
		}
	}
}
