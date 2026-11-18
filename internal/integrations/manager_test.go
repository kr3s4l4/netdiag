package integrations

import (
	"context"
	"testing"
	"time"

	"github.com/kr3s4l4/netdiag/internal/config"
)

// TestStartDoesNotDeadlock guards against re-locking m.mu while Start holds it
// (a previous version called settings() inside the critical section and hung
// the whole server before it ever bound its listening socket).
func TestStartDoesNotDeadlock(t *testing.T) {
	cfg := config.Integrations{Enabled: true, Timeout: config.Duration(2 * time.Second), MaxPerMinute: 60, QueueSize: 16, AllowPrivateTargets: true}
	m := New(nil, cfg, nil)
	done := make(chan struct{})
	go func() {
		m.Start(context.Background())
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Start bloqueó: posible deadlock al tomar el mutex")
	}
	m.Stop()

	// A second Start/Stop cycle must also be safe.
	done2 := make(chan struct{})
	go func() { m.Start(context.Background()); close(done2) }()
	select {
	case <-done2:
	case <-time.After(2 * time.Second):
		t.Fatal("el segundo Start bloqueó")
	}
	m.Stop()
}
