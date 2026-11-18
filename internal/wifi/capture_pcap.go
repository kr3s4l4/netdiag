//go:build pcap

package wifi

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/pcap"
)

// Available reports whether this binary can capture raw 802.11 frames.
func Available() bool { return true }

// Interfaces lists capture-capable devices (including monitor-mode devices
// without an L3 address, which netutil.Interfaces() intentionally omits).
func Interfaces() ([]string, error) {
	devs, err := pcap.FindAllDevs()
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(devs))
	for _, d := range devs {
		out = append(out, d.Name)
	}
	return out, nil
}

// Capture listens on iface for duration and delivers every decoded frame to
// onPacket. The caller must put the device into monitor mode beforehand; this
// function only optionally hops channels via `iw` when channels is non-empty.
func Capture(ctx context.Context, iface string, duration time.Duration, channels []int, onPacket func(gopacket.Packet)) error {
	if iface == "" {
		return fmt.Errorf("sin interfaz de monitorización")
	}
	handle, err := pcap.OpenLive(iface, 65536, true, 200*time.Millisecond)
	if err != nil {
		return err
	}
	defer handle.Close()
	if err := handle.SetBPFFilter("type mgt subtype beacon or type mgt subtype probe-resp or type data"); err != nil {
		return err
	}
	source := gopacket.NewPacketSource(handle, handle.LinkType())
	source.DecodeOptions.Lazy = true
	source.NoCopy = true
	packets := source.Packets()

	ctx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()

	if len(channels) > 0 {
		go hopChannels(ctx, iface, channels)
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case pkt, ok := <-packets:
			if !ok {
				return nil
			}
			onPacket(pkt)
		}
	}
}

// hopChannels is best effort: iw may be absent or the driver may reject a
// channel. Capture continues on the current channel in that case.
func hopChannels(ctx context.Context, iface string, channels []int) {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	i := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ch := channels[i%len(channels)]
			i++
			_ = exec.CommandContext(ctx, "iw", "dev", iface, "set", "channel", fmt.Sprint(ch)).Run()
		}
	}
}
