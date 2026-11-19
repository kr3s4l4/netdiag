//go:build !pcap

package capture

import (
	"context"

	"github.com/kr3s4l4/netdiag/internal/model"
)

// Available reports whether this binary was built with passive capture.
func Available() bool { return false }

// Capture is an explicit no-op in portable builds.
func Capture(ctx context.Context, cfg Config, onObservation func(model.PassiveObservation)) (*Result, error) {
	return &Result{Available: false, Error: ErrUnavailable.Error()}, ErrUnavailable
}
