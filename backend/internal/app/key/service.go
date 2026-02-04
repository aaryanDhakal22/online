package keyApp

import (
	"context"

	keys "quicc/online/internal/domain/key"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

type KeyService struct {
	keyRepo keys.Repository
	rd      *redis.Client
	logger  zerolog.Logger
}

func NewKeyService(keyRepo keys.Repository, rd *redis.Client, logger zerolog.Logger) *KeyService {
	logger = logger.With().Str("service", "key").Logger()
	return &KeyService{
		keyRepo: keyRepo,
		rd:      rd,
		logger:  logger,
	}
}

func (s *KeyService) Generate(cmd GenerateKeyCommand) (*GenerateKeyResult, error) {
	// Generate a new API key
	s.logger.Info().Msg("Generating new key entity")
	newKey, err := keys.NewAPIKey()
	if err != nil {
		s.logger.Error().Err(err).Msg("Error generating new key entity")
		return nil, err
	}

	// Unprime the previous key
	s.logger.Info().Msg("Unpriming previous key")
	if err := s.keyRepo.UnprimeAll(context.TODO()); err != nil {
		s.logger.Error().Err(err).Msg("Error unpriming previous key")
		return nil, err
	}

	// Create the API key
	s.logger.Info().Msg("Creating new key record in database")
	s.keyRepo.Create(context.TODO(), newKey)

	// Set the API key in Redis
	s.logger.Info().Msg("Setting new key in Redis")
	s.rd.Set(context.TODO(), "primed_key", newKey.Key, 0)

	// Return the API key
	s.logger.Info().Msg("Returning new key")
	defer s.logger.Info().Msg("New Key was successfully generated and returned.")
	return &GenerateKeyResult{
		ID:  newKey.ID,
		Key: newKey.Key,
	}, nil
}

func (s *KeyService) Set(cmd SetKeyCommand) (*SetKeyResult, error) {
	// Get the API key
	s.logger.Debug().Msg("Getting active key")
	key, err := s.keyRepo.GetPrimed(context.TODO())
	if err != nil {
		return nil, err
	}
	// Deactivate other API keys
	s.logger.Debug().Msg("Deactivating all keys")
	if err := s.keyRepo.DeactivateAllKeys(context.TODO()); err != nil {
		return nil, err
	}

	// Set the API key status
	s.logger.Debug().Msg("Setting active key")
	if err := s.keyRepo.Activate(context.TODO(), key.ID); err != nil {
		return nil, err
	}

	s.logger.Info().Msg("Key set")
	// Update the redis key
	s.rd.Set(context.TODO(), "active_key", key.Key, 0)

	defer s.logger.Info().Msg("Key was successfully set.")
	return &SetKeyResult{
		ID:  key.ID,
		Key: key.Key,
	}, nil
}
