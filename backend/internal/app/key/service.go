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

func (s *KeyService) GetActive(cmd GetActiveKeyCommand) (*GetActiveKeyResult, error) {
	activeKey, err := s.keyRepo.GetActive(context.Background())
	if err != nil {
		return nil, err
	}
	return &GetActiveKeyResult{
		ID:  activeKey.ID,
		Key: activeKey.Key,
	}, nil
}

func (s *KeyService) GetPrimed(cmd GetPrimedKeyCommand) (*GetPrimedKeyResult, error) {
	primedKey, err := s.keyRepo.GetPrimed(context.TODO())
	if err != nil {
		return nil, err
	}
	return &GetPrimedKeyResult{
		ID:  primedKey.ID,
		Key: primedKey.Key,
	}, nil
}

func (s *KeyService) GetByID(cmd GetKeyByIDCommand) (*GetKeyByIDResult, error) {
	key, err := s.keyRepo.GetByID(context.TODO(), cmd.ID)
	if err != nil {
		return nil, err
	}
	return &GetKeyByIDResult{
		ID:  key.ID,
		Key: key.Key,
	}, nil
}

func (s *KeyService) Activate(cmd ActivateKeyCommand) (*ActivateKeyResult, error) {
	err := s.keyRepo.Activate(context.TODO(), cmd.ID)
	if err != nil {
		return nil, err
	}
	return &ActivateKeyResult{
		ID: cmd.ID,
	}, nil
}

func (s *KeyService) Deactivate(cmd DeactivateKeyCommand) (*DeactivateKeyResult, error) {
	err := s.keyRepo.Deactivate(context.TODO(), cmd.ID)
	if err != nil {
		return nil, err
	}
	return &DeactivateKeyResult{
		ID: cmd.ID,
	}, nil
}
