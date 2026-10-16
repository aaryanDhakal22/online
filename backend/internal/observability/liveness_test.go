package observability

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/metric/noop"
)

func newTestWatchdog(t *testing.T, threshold time.Duration) *Watchdog {
	t.Helper()
	stale, err := noop.NewMeterProvider().Meter("test").Int64Counter("stale")
	if err != nil {
		t.Fatal(err)
	}
	return NewWatchdog(threshold, zerolog.Nop(), Meters{LivenessStale: stale})
}

func panics(f func()) (did bool) {
	defer func() { did = recover() != nil }()
	f()
	return false
}

// An idle server (no vendor traffic) stays live as long as its own self-probe
// reaches the pulsing handler; without the probe the watchdog trips.
func TestSelfProbeKeepsIdleServerLive(t *testing.T) {
	const threshold = 150 * time.Millisecond

	w := newTestWatchdog(t, threshold)
	pulse := w.Register("http_server")
	var probes atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		pulse() // stands in for PulseMiddleware
		if r.Header.Get(SelfProbeHeader) != "" {
			probes.Add(1)
		}
		rw.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go RunSelfProbe(ctx, srv.URL+"/health", 20*time.Millisecond, zerolog.Nop())

	time.Sleep(3 * threshold)
	if panics(func() { w.check(ctx) }) {
		t.Fatal("watchdog tripped even though the self-probe was pulsing")
	}
	if probes.Load() == 0 {
		t.Fatal("self-probe never sent the marker header")
	}
}

func TestWatchdogTripsWithoutPulses(t *testing.T) {
	w := newTestWatchdog(t, 50*time.Millisecond)
	w.Register("http_server")
	time.Sleep(120 * time.Millisecond)
	if !panics(func() { w.check(context.Background()) }) {
		t.Fatal("watchdog should panic on a stale heartbeat")
	}
}
