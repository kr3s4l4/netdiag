//go:build !pcap

package wifi

import (
	"context"
	"time"

	"github.com/google/gopacket"
)

// Available reports whether this binary can capture raw 802.11 frames.
func Available() bool { return false }

// Interfaces lists capture-capable devices (unavailable in portable builds).
func Interfaces() ([]string, error) { return nil, ErrCaptureUnavailable }

// Capture is a no-op stub in portable builds.
func Capture(ctx context.Context, iface string, duration time.Duration, channels []int, onPacket func(gopacket.Packet)) error {
	return ErrCaptureUnavailable
}
