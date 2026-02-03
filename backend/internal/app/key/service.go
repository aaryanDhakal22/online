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
	return &KeyService{
		keyRepo: keyRepo,
		rd:      rd,
		logger:  logger,
	}
}

func (s *KeyService) Generate(cmd GenerateKeyCommand) (*GenerateKeyResult, error) {
	// Generate a new API key
	newKey, err := keys.NewAPIKey()
	if err != nil {
		return nil, err
	}

	// Unprime the previous key
	if err := s.keyRepo.UnprimeAll(context.TODO()); err != nil {
		return nil, err
	}

	// Create the API key
	s.keyRepo.Create(context.TODO(), newKey)

	// Set the API key in Redis
	s.rd.Set(context.TODO(), "primed_key", newKey.Key, 0)

	// Return the API key
	return &GenerateKeyResult{
		ID:  newKey.ID,
		Key: newKey.Key,
	}, nil
}

func (s *KeyService) Set(cmd SetKeyCommand) (*SetKeyResult, error) {
	// Get the API key
	key, err := s.keyRepo.GetPrimed(context.TODO())
	if err != nil {
		return nil, err
	}
	// Deactivate other API keys
	if err := s.keyRepo.DeactivateAllKeys(context.TODO()); err != nil {
		return nil, err
	}

	// Set the API key status
	if err := s.keyRepo.Activate(context.TODO(), key.ID); err != nil {
		return nil, err
	}

	// Update the redis key
	s.rd.Set(context.TODO(), "active_key", key.Key, 0)

	return &SetKeyResult{
		ID:  key.ID,
		Key: key.Key,
	}, nil
}
