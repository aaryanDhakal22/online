package observability

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Watchdog tracks heartbeats from long-running goroutines. Each registered
// goroutine receives a Pulse() func it calls on every meaningful loop
// iteration. A 30-second ticker checks all heartbeats; any goroutine that
// hasn't pulsed within Threshold gets a LivenessStale metric increment and
// triggers a panic so the container restarts.
//
// This is deliberately blunt: the goal is "process noticed it's wedged before
// the restaurant calls". Tuning is via the Threshold parameter — too aggressive
// causes false-positive restarts; too loose lets a wedged goroutine sit.
type Watchdog struct {
	threshold time.Duration
	tick      time.Duration
	logger    zerolog.Logger
	stale     metric.Int64Counter

	mu    sync.RWMutex
	beats map[string]*atomic.Int64 // last pulse unix-nano
}

func NewWatchdog(threshold time.Duration, logger zerolog.Logger, m Meters) *Watchdog {
	return &Watchdog{
		threshold: threshold,
		tick:      30 * time.Second,
		logger:    logger.With().Str("module", "watchdog").Logger(),
		stale:     m.LivenessStale,
		beats:     make(map[string]*atomic.Int64),
	}
}

// Register adds a goroutine to the watchdog. The returned Pulse func should be
// called on every meaningful loop iteration to refresh the heartbeat.
func (w *Watchdog) Register(name string) func() {
	w.mu.Lock()
	defer w.mu.Unlock()
	beat, ok := w.beats[name]
	if !ok {
		beat = new(atomic.Int64)
		w.beats[name] = beat
	}
	beat.Store(time.Now().UnixNano())
	return func() { beat.Store(time.Now().UnixNano()) }
}

func (w *Watchdog) Run(ctx context.Context) {
	pulse := w.Register("watchdog")
	t := time.NewTicker(w.tick)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			w.logger.Info().Msg("watchdog stopping")
			return
		case <-t.C:
			pulse()
			w.check(ctx)
		}
	}
}

func (w *Watchdog) check(ctx context.Context) {
	now := time.Now().UnixNano()
	limit := w.threshold.Nanoseconds()

	w.mu.RLock()
	stale := make([]string, 0)
	for name, beat := range w.beats {
		if now-beat.Load() > limit {
			stale = append(stale, name)
		}
	}
	w.mu.RUnlock()

	if len(stale) == 0 {
		return
	}

	for _, name := range stale {
		w.stale.Add(ctx, 1, metric.WithAttributes(attribute.String("goroutine", name)))
		w.logger.Error().Str("goroutine", name).Dur("threshold", w.threshold).
			Msg("liveness threshold exceeded — panicking to force container restart")
	}
	panic("liveness watchdog: stale goroutine(s): " + joinNames(stale))
}

func joinNames(names []string) string {
	return strings.Join(names, ", ")
}

// SelfProbeHeader marks the watchdog's own /health probe so the OTEL and
// request-log middlewares skip it (no span / log line every interval) while
// PulseMiddleware still counts it.
const SelfProbeHeader = "X-Quicc-Self-Probe"

// RunSelfProbe GETs url (the server's own /health) every interval until ctx
// ends. Each probe travels the real middleware chain, so it pulses the
// "http_server" heartbeat exactly like vendor traffic — an idle-but-healthy
// server no longer trips the watchdog, while a wedged one still does (its
// probe can't complete). Failures are logged, never fatal: the watchdog is
// the one that decides.
func RunSelfProbe(ctx context.Context, url string, interval time.Duration, logger zerolog.Logger) {
	lg := logger.With().Str("module", "self-probe").Logger()
	client := &http.Client{Timeout: 5 * time.Second}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			if err != nil {
				lg.Warn().Err(err).Msg("self-probe: build request")
				continue
			}
			req.Header.Set(SelfProbeHeader, "1")
			resp, err := client.Do(req)
			if err != nil {
				if ctx.Err() == nil {
					lg.Warn().Err(err).Str("url", url).Msg("self-probe failed")
				}
				continue
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				lg.Warn().Int("status", resp.StatusCode).Str("url", url).Msg("self-probe: unexpected status")
			}
		}
	}
}
