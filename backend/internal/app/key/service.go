// Package keyApp owns API-key lifecycle. Two mutation flows are supported:
//
//  1. Standard generate-then-set: GET /generate stages a Primed key (sender
//     copies the plaintext from the response). Admin then hits GET /set
//     which finds the Primed and promotes it to Active.
//
//  2. Break-glass set-by-plaintext: POST /setKey {"key": "..."} bypasses
//     Primed and immediately stores+activates the supplied key. Used when
//     the admin already holds a key the sender is using and needs to align
//     online/'s state without re-rolling.
//
// Each method gets its own span so Tempo shows the full key-lifecycle
// trace. Custom counters (KeysGenerated / KeysActivated / KeysBootstrap)
// land in Prometheus for at-a-glance dashboards.
//
// The active key is cached in an atomic.Pointer for the read-mostly Verify
// hot path. There is no Redis: SQLite is the source of truth.
package keyApp

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"errors"
	"sync/atomic"

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

type KeyService struct {
	repo   keys.Repository
	logger zerolog.Logger
	meters observability.Meters
	tracer trace.Tracer
	cache  atomic.Pointer[keys.APIKey]
}

func NewKeyService(repo keys.Repository, logger zerolog.Logger, meters observability.Meters) *KeyService {
	return &KeyService{
		repo:   repo,
		logger: logger.With().Str("service", "key").Logger(),
		meters: meters,
		tracer: otel.Tracer(tracerName),
	}
}

// Bootstrap is called once after migrations. If no Active key exists in
// SQLite, it inserts the provided default. Either way, the cache is warmed
// with whatever Active key is in the DB after this call.
func (s *KeyService) Bootstrap(ctx context.Context, defaultPlain string) error {
	ctx, span := s.tracer.Start(ctx, "key.bootstrap")
	defer span.End()

	s.logger.Debug().Ctx(ctx).Msg("bootstrap: checking for existing active key")

	existing, err := s.repo.GetActive(ctx)
	if err == nil && existing != nil {
		s.cache.Store(existing)
		span.SetAttributes(attribute.String("result", "existing"), attribute.String("key.id", existing.ID))
		s.meters.KeysBootstrap.Add(ctx, 1, metric.WithAttributes(attribute.String("result", "existing")))
		s.logger.Info().Ctx(ctx).Str("key_id", existing.ID).Msg("bootstrap: loaded existing active key")
		return nil
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		span.RecordError(err)
		span.SetStatus(codes.Error, "GetActive failed")
		s.logger.Error().Err(err).Ctx(ctx).Msg("bootstrap: GetActive returned unexpected error")
		return err
	}

	s.logger.Debug().Ctx(ctx).Msg("bootstrap: no active key in DB")

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
	s.cache.Store(seed)
	span.SetAttributes(attribute.String("result", "seeded"), attribute.String("key.id", seed.ID))
	s.meters.KeysBootstrap.Add(ctx, 1, metric.WithAttributes(attribute.String("result", "seeded")))
	s.logger.Info().Ctx(ctx).Str("key_id", seed.ID).Msg("bootstrap: seeded default API key into DB")
	return nil
}

// Verify is the hot path: middleware calls this on every request. It reads
// the atomic pointer, falls back to DB only if the cache is empty (which
// shouldn't happen post-Bootstrap), and uses constant-time comparison.
func (s *KeyService) Verify(ctx context.Context, plain string) (bool, error) {
	ctx, span := s.tracer.Start(ctx, "key.verify")
	defer span.End()

	cached := s.cache.Load()
	if cached == nil {
		s.logger.Debug().Ctx(ctx).Msg("verify: cache miss, falling back to DB")
		fresh, err := s.repo.GetActive(ctx)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "GetActive failed")
			s.logger.Error().Err(err).Ctx(ctx).Msg("verify: DB lookup failed")
			return false, err
		}
		s.cache.Store(fresh)
		cached = fresh
	}

	match := subtle.ConstantTimeCompare([]byte(cached.Key), []byte(plain)) == 1
	span.SetAttributes(attribute.Bool("match", match))
	s.logger.Debug().Ctx(ctx).Bool("match", match).Msg("verify: comparison complete")
	return match, nil
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

// Set is the standard activate flow: find the Primed key, deactivate any
// existing Active, activate the primed one, refresh the cache.
func (s *KeyService) Set(ctx context.Context) (*SetKeyResult, error) {
	ctx, span := s.tracer.Start(ctx, "key.set")
	defer span.End()

	s.logger.Debug().Ctx(ctx).Msg("set: looking up primed key")
	primed, err := s.repo.GetPrimed(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "GetPrimed failed")
		s.logger.Warn().Err(err).Ctx(ctx).Msg("set: no primed key available")
		return nil, err
	}

	s.logger.Debug().Ctx(ctx).Str("key_id", primed.ID).Msg("set: deactivating prior active keys")
	if err := s.repo.DeactivateAllKeys(ctx); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "DeactivateAllKeys failed")
		s.logger.Error().Err(err).Ctx(ctx).Msg("set: DeactivateAllKeys failed")
		return nil, err
	}
	s.logger.Debug().Ctx(ctx).Str("key_id", primed.ID).Msg("set: activating primed key")
	if err := s.repo.Activate(ctx, primed.ID); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Activate failed")
		s.logger.Error().Err(err).Ctx(ctx).Msg("set: Activate failed")
		return nil, err
	}

	fresh, err := s.repo.GetByID(ctx, primed.ID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "GetByID failed")
		s.logger.Error().Err(err).Ctx(ctx).Msg("set: GetByID failed after activation")
		return nil, err
	}
	s.cache.Store(fresh)

	span.SetAttributes(attribute.String("key.id", fresh.ID), attribute.String("path", "primed"))
	s.meters.KeysActivated.Add(ctx, 1, metric.WithAttributes(attribute.String("path", "primed")))
	s.logger.Info().Ctx(ctx).Str("key_id", fresh.ID).Msg("set: primed key activated and cache refreshed")
	return &SetKeyResult{ID: fresh.ID, Key: fresh.Key}, nil
}

// SetByKey is the break-glass path: the admin already has the plaintext
// they want to make active (typically pasted from a CLI). It inserts that
// plaintext directly as Active without going through Primed, deactivates
// any prior Active, and refreshes the cache.
func (s *KeyService) SetByKey(ctx context.Context, plain string) (*SetKeyResult, error) {
	ctx, span := s.tracer.Start(ctx, "key.set_by_key")
	defer span.End()

	if plain == "" {
		err := errors.New("key is required")
		span.RecordError(err)
		span.SetStatus(codes.Error, "empty plaintext")
		return nil, err
	}

	s.logger.Debug().Ctx(ctx).Msg("set_by_key: deactivating prior active keys")
	if err := s.repo.DeactivateAllKeys(ctx); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "DeactivateAllKeys failed")
		s.logger.Error().Err(err).Ctx(ctx).Msg("set_by_key: DeactivateAllKeys failed")
		return nil, err
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
	s.cache.Store(row)

	span.SetAttributes(attribute.String("key.id", row.ID), attribute.String("path", "break_glass"))
	s.meters.KeysActivated.Add(ctx, 1, metric.WithAttributes(attribute.String("path", "break_glass")))
	s.logger.Info().Ctx(ctx).Str("key_id", row.ID).Msg("set_by_key: break-glass key activated")
	return &SetKeyResult{ID: row.ID, Key: row.Key}, nil
}

// Get returns the currently active key (plaintext). Used by the admin GET endpoint.
func (s *KeyService) Get(ctx context.Context) (*GetKeyResult, error) {
	cached := s.cache.Load()
	if cached == nil {
		s.logger.Debug().Ctx(ctx).Msg("get: cache miss, falling back to DB")
		fresh, err := s.repo.GetActive(ctx)
		if err != nil {
			s.logger.Error().Err(err).Ctx(ctx).Msg("get: DB lookup failed")
			return nil, err
		}
		s.cache.Store(fresh)
		cached = fresh
	}
	return &GetKeyResult{Key: cached.Key}, nil
}
