package keyApp

import (
	"context"
	"time"

	keys "quicc/online/internal/domain/key"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

type RedisClient interface {
	Get(ctx context.Context, key string) *redis.StringCmd
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd
}

type KeyService struct {
	keyRepo keys.Repository
	rd      RedisClient
	logger  zerolog.Logger
}

func NewKeyService(keyRepo keys.Repository, rd RedisClient, logger zerolog.Logger) *KeyService {
	logger = logger.With().Str("service", "key").Logger()
	return &KeyService{
		keyRepo: keyRepo,
		rd:      rd,
		logger:  logger,
	}
}

func (s *KeyService) Generate(ctx context.Context, cmd GenerateKeyCommand) (*GenerateKeyResult, error) {
	// Generate a new API key
	s.logger.Info().Msg("Generating new key entity")
	newKey, err := keys.NewAPIKey()
	if err != nil {
		s.logger.Error().Err(err).Msg("Error generating new key entity")
		return nil, err
	}

	// Unprime the previous key
	s.logger.Info().Msg("Unpriming previous key")
	if err := s.keyRepo.UnprimeAll(ctx); err != nil {
		s.logger.Error().Err(err).Msg("Error unpriming previous key")
		return nil, err
	}

	// Create the API key
	s.logger.Info().Msg("Creating new key record in database")
	if err := s.keyRepo.Create(ctx, newKey); err != nil {
		s.logger.Error().Err(err).Msg("Error creating new key record in database")
		return nil, err
	}

	// Return the API key
	s.logger.Info().Msg("Returning new key")
	defer s.logger.Info().Msg("New Key was successfully generated and returned.")
	return &GenerateKeyResult{
		ID:  newKey.ID,
		Key: newKey.Key,
	}, nil
}

func (s *KeyService) Set(ctx context.Context, cmd SetKeyCommand) (*SetKeyResult, error) {
	// Get the API key
	s.logger.Debug().Msg("Getting active key")
	key, err := s.keyRepo.GetPrimed(ctx)
	if err != nil {
		return nil, err
	}
	// Deactivate other API keys
	s.logger.Debug().Msg("Deactivating all keys")
	if err := s.keyRepo.DeactivateAllKeys(ctx); err != nil {
		return nil, err
	}

	// Set the API key status
	s.logger.Debug().Msg("Setting active key")
	if err := s.keyRepo.Activate(ctx, key.ID); err != nil {
		return nil, err
	}

	s.logger.Info().Msg("Key set")
	// Update the redis key
	status := s.rd.Set(ctx, "active_key", key.Key, 0)
	if status.Err() != nil {
		s.logger.Error().Err(status.Err()).Msg("Error setting active key in Redis")
		return nil, status.Err()
	}

	defer s.logger.Info().Msg("Key was successfully set.")
	return &SetKeyResult{
		ID:  key.ID,
		Key: key.Key,
	}, nil
}

func (s *KeyService) Verify(ctx context.Context, cmd VerifyKeyCommand) (*VerifyKeyResult, error) {
	s.logger.Info().Msg("Verifying key")
	key, err := s.rd.Get(ctx, "active_key").Result()
	if err != nil {
		s.logger.Error().Err(err).Msg("Error getting active key from Redis")
		return nil, err
	}
	s.logger.Info().Msg("Key retrieved")
	return &VerifyKeyResult{
		Match: key == cmd.Key,
	}, nil
}
