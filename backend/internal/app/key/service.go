// Package keyApp owns API-key lifecycle. The vendor's key is the one thing
// that, if lost, stops every online order — so rotation is two-step and the
// last active key can never be removed:
//
//  1. Add: GET /generate stages a Primed key (the sender copies the
//     plaintext); admin GET /set promotes it to Active. Break-glass: admin
//     POST /setKey {"key": "..."} stores a known plaintext as Active
//     directly. Neither deactivates anything — the old key keeps working.
//  2. Retire: once the vendor has used the new key, admin
//     POST /keys/{id}/retire deactivates the old one. Refused with
//     ErrLastActiveKey when it is the only active key, and with
//     ErrNewKeyUnconfirmed until another active key has been used since
//     both keys were added. The DB triggers in 002_key_guard.sql enforce the
//     last-active rule too, whoever writes the table.
//
// Verify accepts any active key (constant-time compare over each) and stamps
// the matching key's last_used_at, which is what confirms a new key.
//
// Each method gets its own span so Tempo shows the full key-lifecycle
// trace. Custom counters (KeysGenerated / KeysActivated / KeysBootstrap /
// KeysRetired) land in Prometheus.
//
// The active keys are cached as an immutable snapshot behind an
// atomic.Pointer for the read-mostly Verify path; every mutation reloads it.
// There is no Redis: SQLite is the source of truth.
package keyApp

import (
	"context"
	"crypto/subtle"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	keys "quicc/online/internal/domain/key"
	"quicc/online/internal/observability"

	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const tracerName = "quiccpos/online/key"

// stampEvery bounds last_used_at writes to one per key per interval in this
// process. Any key mutation resets it, so the first use after a new key is
// added is always recorded.
const stampEvery = 30 * time.Second

type KeyService struct {
	repo   keys.Repository
	logger zerolog.Logger
	meters observability.Meters
	tracer trace.Tracer

	// active is the snapshot of active keys, newest first. Never mutated
	// after Store — reload swaps in a new slice.
	active atomic.Pointer[[]*keys.APIKey]

	// mu serializes mutations (generate / set / setKey / retire).
	mu sync.Mutex

	stampMu sync.Mutex
	stamped map[string]time.Time
}

func NewKeyService(repo keys.Repository, logger zerolog.Logger, meters observability.Meters) *KeyService {
	return &KeyService{
		repo:    repo,
		logger:  logger.With().Str("service", "key").Logger(),
		meters:  meters,
		tracer:  otel.Tracer(tracerName),
		stamped: map[string]time.Time{},
	}
}

// reload re-reads the active keys into the cache and resets the stamp
// throttle.
func (s *KeyService) reload(ctx context.Context) ([]*keys.APIKey, error) {
	list, err := s.repo.ListActive(ctx)
	if err != nil {
		return nil, err
	}
	s.active.Store(&list)
	s.stampMu.Lock()
	s.stamped = map[string]time.Time{}
	s.stampMu.Unlock()
	return list, nil
}

func (s *KeyService) snapshot(ctx context.Context) ([]*keys.APIKey, error) {
	if p := s.active.Load(); p != nil {
		return *p, nil
	}
	s.logger.Debug().Ctx(ctx).Msg("cache miss, loading active keys from DB")
	return s.reload(ctx)
}

// Bootstrap is called once after migrations. If no Active key exists in
// SQLite, it inserts the provided default. Either way, the cache is warmed
// with whatever Active keys are in the DB after this call.
func (s *KeyService) Bootstrap(ctx context.Context, defaultPlain string) error {
	ctx, span := s.tracer.Start(ctx, "key.bootstrap")
	defer span.End()

	s.mu.Lock()
	defer s.mu.Unlock()

	existing, err := s.reload(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "ListActive failed")
		s.logger.Error().Err(err).Ctx(ctx).Msg("bootstrap: ListActive returned unexpected error")
		return err
	}
	if len(existing) > 0 {
		span.SetAttributes(attribute.String("result", "existing"), attribute.Int("keys.active", len(existing)))
		s.meters.KeysBootstrap.Add(ctx, 1, metric.WithAttributes(attribute.String("result", "existing")))
		s.logger.Info().Ctx(ctx).Int("active_keys", len(existing)).Str("newest_key_id", existing[0].ID).
			Msg("bootstrap: loaded existing active keys")
		return nil
	}

	if defaultPlain == "" {
		span.SetAttributes(attribute.String("result", "skipped"))
		s.meters.KeysBootstrap.Add(ctx, 1, metric.WithAttributes(attribute.String("result", "skipped")))
		s.logger.Warn().Ctx(ctx).Msg("bootstrap: no DEFAULT_API_KEY provided — service will reject all auth until a key is generated and set")
		return nil
	}

	seed := keys.FromExisting(defaultPlain)
	if err := s.repo.Create(ctx, seed); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Create failed")
		s.logger.Error().Err(err).Ctx(ctx).Msg("bootstrap: failed to seed default key")
		return err
	}
	if _, err := s.reload(ctx); err != nil {
		return err
	}
	span.SetAttributes(attribute.String("result", "seeded"), attribute.String("key.id", seed.ID))
	s.meters.KeysBootstrap.Add(ctx, 1, metric.WithAttributes(attribute.String("result", "seeded")))
	s.logger.Info().Ctx(ctx).Str("key_id", seed.ID).Msg("bootstrap: seeded default API key into DB")
	return nil
}

// Verify is the hot path: middleware calls this on every request. It reads
// the cached snapshot and compares the token against every active key in
// constant time (no early exit), then stamps the matching key's
// last_used_at.
func (s *KeyService) Verify(ctx context.Context, plain string) (bool, error) {
	ctx, span := s.tracer.Start(ctx, "key.verify")
	defer span.End()

	active, err := s.snapshot(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "ListActive failed")
		s.logger.Error().Err(err).Ctx(ctx).Msg("verify: DB lookup failed")
		return false, err
	}

	var hit *keys.APIKey
	for _, k := range active {
		eq := subtle.ConstantTimeCompare([]byte(k.Key), []byte(plain)) == 1
		if eq && hit == nil {
			hit = k
		}
	}
	match := hit != nil
	span.SetAttributes(attribute.Bool("match", match), attribute.Int("keys.active", len(active)))
	if match {
		span.SetAttributes(attribute.String("key.id", hit.ID))
		s.stamp(ctx, hit.ID)
	}
	s.logger.Debug().Ctx(ctx).Bool("match", match).Msg("verify: comparison complete")
	return match, nil
}

// stamp records a successful use of key id (throttled per process). A failed
// write is logged and never fails the request.
func (s *KeyService) stamp(ctx context.Context, id string) {
	now := keys.Now()
	s.stampMu.Lock()
	if last, ok := s.stamped[id]; ok && now.Sub(last) < stampEvery {
		s.stampMu.Unlock()
		return
	}
	s.stamped[id] = now
	s.stampMu.Unlock()

	if err := s.repo.TouchLastUsed(ctx, id, now); err != nil {
		s.stampMu.Lock()
		delete(s.stamped, id)
		s.stampMu.Unlock()
		s.logger.Warn().Err(err).Ctx(ctx).Str("key_id", id).Msg("verify: last_used_at stamp failed")
	}
}

// Generate creates a new Primed key. Any prior Primed entries are unprimed
// first so only the most recent generation is in the Primed slot. The
// returned plaintext is shown to the sender so they can configure their
// integration; nothing is activated yet — call Set to promote.
func (s *KeyService) Generate(ctx context.Context, cmd GenerateKeyCommand) (*GenerateKeyResult, error) {
	ctx, span := s.tracer.Start(ctx, "key.generate",
		trace.WithAttributes(attribute.Bool("custom_plaintext", cmd.Key != "")),
	)
	defer span.End()

	key, err := keys.NewAPIKey()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "NewAPIKey failed")
		s.logger.Error().Err(err).Ctx(ctx).Msg("generate: rand failed")
		return nil, err
	}
	if cmd.Key != "" {
		key.Key = cmd.Key
		s.logger.Debug().Ctx(ctx).Msg("generate: using caller-supplied plaintext")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.logger.Debug().Ctx(ctx).Msg("generate: unpriming any prior primed entries")
	if err := s.repo.UnprimeAll(ctx); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "UnprimeAll failed")
		s.logger.Error().Err(err).Ctx(ctx).Msg("generate: UnprimeAll failed")
		return nil, err
	}
	if err := s.repo.Create(ctx, key); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Create failed")
		s.logger.Error().Err(err).Ctx(ctx).Msg("generate: Create failed")
		return nil, err
	}

	span.SetAttributes(attribute.String("key.id", key.ID))
	s.meters.KeysGenerated.Add(ctx, 1)
	s.logger.Info().Ctx(ctx).Str("key_id", key.ID).Msg("generate: new primed key stored")
	return &GenerateKeyResult{ID: key.ID, Key: key.Key}, nil
}

// Set promotes the Primed key to Active, ADDING it next to the current
// active key(s) — nothing is deactivated. Retire the old key once the vendor
// has used the new one.
func (s *KeyService) Set(ctx context.Context) (*SetKeyResult, error) {
	ctx, span := s.tracer.Start(ctx, "key.set")
	defer span.End()

	s.mu.Lock()
	defer s.mu.Unlock()

	s.logger.Debug().Ctx(ctx).Msg("set: looking up primed key")
	primed, err := s.repo.GetPrimed(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "GetPrimed failed")
		s.logger.Warn().Err(err).Ctx(ctx).Msg("set: no primed key available")
		return nil, err
	}

	s.logger.Debug().Ctx(ctx).Str("key_id", primed.ID).Msg("set: activating primed key (prior keys stay active)")
	if err := s.repo.Activate(ctx, primed.ID); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Activate failed")
		s.logger.Error().Err(err).Ctx(ctx).Msg("set: Activate failed")
		return nil, err
	}
	active, err := s.reload(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "reload failed")
		s.logger.Error().Err(err).Ctx(ctx).Msg("set: reload failed after activation")
		return nil, err
	}

	span.SetAttributes(attribute.String("key.id", primed.ID), attribute.String("path", "primed"), attribute.Int("keys.active", len(active)))
	s.meters.KeysActivated.Add(ctx, 1, metric.WithAttributes(attribute.String("path", "primed")))
	s.logger.Info().Ctx(ctx).Str("key_id", primed.ID).Int("active_keys", len(active)).
		Msg("set: primed key added as active; retire the old key once the vendor uses the new one")
	return &SetKeyResult{ID: primed.ID, Key: primed.Key}, nil
}

// SetByKey is the break-glass path: the admin already has the plaintext
// they want active (typically pasted from a CLI). It stores that plaintext
// as Active directly, without going through Primed, and — like Set — leaves
// the existing active key(s) alone. A plaintext that is already active is
// returned as is.
func (s *KeyService) SetByKey(ctx context.Context, plain string) (*SetKeyResult, error) {
	ctx, span := s.tracer.Start(ctx, "key.set_by_key")
	defer span.End()

	if plain == "" {
		err := errors.New("key is required")
		span.RecordError(err)
		span.SetStatus(codes.Error, "empty plaintext")
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	active, err := s.reload(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "ListActive failed")
		return nil, err
	}
	for _, k := range active {
		if subtle.ConstantTimeCompare([]byte(k.Key), []byte(plain)) == 1 {
			span.SetAttributes(attribute.String("key.id", k.ID), attribute.String("result", "already_active"))
			s.logger.Info().Ctx(ctx).Str("key_id", k.ID).Msg("set_by_key: key already active, nothing to do")
			return &SetKeyResult{ID: k.ID, Key: k.Key}, nil
		}
	}

	s.logger.Debug().Ctx(ctx).Msg("set_by_key: unpriming prior primed keys")
	if err := s.repo.UnprimeAll(ctx); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "UnprimeAll failed")
		s.logger.Error().Err(err).Ctx(ctx).Msg("set_by_key: UnprimeAll failed")
		return nil, err
	}

	row := keys.FromExisting(plain) // status = Active
	if err := s.repo.Create(ctx, row); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Create failed")
		s.logger.Error().Err(err).Ctx(ctx).Msg("set_by_key: Create failed")
		return nil, err
	}
	if _, err := s.reload(ctx); err != nil {
		return nil, err
	}

	span.SetAttributes(attribute.String("key.id", row.ID), attribute.String("path", "break_glass"))
	s.meters.KeysActivated.Add(ctx, 1, metric.WithAttributes(attribute.String("path", "break_glass")))
	s.logger.Info().Ctx(ctx).Str("key_id", row.ID).Msg("set_by_key: break-glass key added as active")
	return &SetKeyResult{ID: row.ID, Key: row.Key}, nil
}

// Retire deactivates one active key — step two of a rotation. Refused with
// keys.ErrLastActiveKey when it is the only active key, and with
// keys.ErrNewKeyUnconfirmed unless another active key has been used since it
// was added and since the retiring key was added (so the vendor is shown to
// be on a key that stays). keys.ErrNotFound / keys.ErrNotActive otherwise.
func (s *KeyService) Retire(ctx context.Context, id string) (err error) {
	ctx, span := s.tracer.Start(ctx, "key.retire", trace.WithAttributes(attribute.String("key.id", id)))
	defer span.End()
	result := "retired"
	defer func() {
		if err != nil {
			result = "error"
			for _, e := range []error{keys.ErrLastActiveKey, keys.ErrNewKeyUnconfirmed, keys.ErrNotFound, keys.ErrNotActive} {
				if errors.Is(err, e) {
					result = e.Error()
				}
			}
			span.SetStatus(codes.Error, result)
		}
		span.SetAttributes(attribute.String("result", result))
		s.meters.KeysRetired.Add(ctx, 1, metric.WithAttributes(attribute.String("result", result)))
	}()

	s.mu.Lock()
	defer s.mu.Unlock()

	target, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if target.Status != keys.Active {
		return keys.ErrNotActive
	}
	active, err := s.repo.ListActive(ctx)
	if err != nil {
		return err
	}
	others := 0
	confirmed := false
	for _, k := range active {
		if k.ID == id {
			continue
		}
		others++
		if !k.LastUsedAt.IsZero() && k.LastUsedAt.After(k.CreatedAt) && k.LastUsedAt.After(target.CreatedAt) {
			confirmed = true
		}
	}
	if others == 0 {
		s.logger.Warn().Ctx(ctx).Str("key_id", id).Msg("retire: refused — it is the only active key")
		return keys.ErrLastActiveKey
	}
	if !confirmed {
		s.logger.Warn().Ctx(ctx).Str("key_id", id).Msg("retire: refused — no other active key has been used since it was added")
		return keys.ErrNewKeyUnconfirmed
	}
	if err := s.repo.Retire(ctx, id); err != nil {
		return err
	}
	if _, err := s.reload(ctx); err != nil {
		return err
	}
	s.logger.Info().Ctx(ctx).Str("key_id", id).Int("active_keys", others).Msg("retire: key deactivated")
	return nil
}

// List returns every key's metadata (never the plaintext), newest first.
func (s *KeyService) List(ctx context.Context) ([]KeyInfo, error) {
	ctx, span := s.tracer.Start(ctx, "key.list")
	defer span.End()

	all, err := s.repo.List(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "List failed")
		return nil, err
	}
	out := make([]KeyInfo, 0, len(all))
	for _, k := range all {
		out = append(out, KeyInfo{
			ID:         k.ID,
			Status:     string(k.Status),
			CreatedAt:  tsPtr(k.CreatedAt),
			LastUsedAt: tsPtr(k.LastUsedAt),
		})
	}
	return out, nil
}

func tsPtr(t time.Time) *string {
	if t.IsZero() {
		return nil
	}
	v := t.UTC().Format(time.RFC3339Nano)
	return &v
}

// Get returns the newest active key (plaintext). Used by the admin GET
// endpoint. keys.ErrNotFound when there is no active key.
func (s *KeyService) Get(ctx context.Context) (*GetKeyResult, error) {
	active, err := s.snapshot(ctx)
	if err != nil {
		s.logger.Error().Err(err).Ctx(ctx).Msg("get: DB lookup failed")
		return nil, err
	}
	if len(active) == 0 {
		return nil, keys.ErrNotFound
	}
	return &GetKeyResult{ID: active[0].ID, Key: active[0].Key}, nil
}
