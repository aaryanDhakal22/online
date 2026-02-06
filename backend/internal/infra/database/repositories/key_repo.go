package repositories

import (
	"context"

	keys "quicc/online/internal/domain/key"
	models "quicc/online/internal/infra/database/models"

	"github.com/rs/zerolog"
)

type KeyRepository struct {
	db     *models.Queries
	logger zerolog.Logger
}

func NewKeyRepository(db *models.Queries, logger zerolog.Logger) *KeyRepository {
	dbLogger := logger.With().Str("component", "repository").Logger()
	return &KeyRepository{
		db:     db,
		logger: dbLogger,
	}
}

func (r *KeyRepository) Create(ctx context.Context, key *keys.APIKey) error {
	return r.db.CreateKey(ctx, models.CreateKeyParams{
		ID:     key.ID,
		Key:    key.Key,
		Status: string(key.Status),
	})
}

func (r *KeyRepository) GetActive(ctx context.Context) (*keys.APIKey, error) {
	key, err := r.db.GetActiveKey(ctx)
	if err != nil {
		return nil, err
	}
	return toKeyDomain(key), nil
}

func (r *KeyRepository) GetPrimed(ctx context.Context) (*keys.APIKey, error) {
	key, err := r.db.GetPrimedKey(ctx)
	if err != nil {
		return nil, err
	}
	return toKeyDomain(key), nil
}

func (r *KeyRepository) GetByID(ctx context.Context, id string) (*keys.APIKey, error) {
	key, err := r.db.GetKeyByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return toKeyDomain(key), nil
}

func (r *KeyRepository) Activate(ctx context.Context, id string) error {
	return r.db.ActivateKey(ctx, id)
}

func (r *KeyRepository) DeactivateAllKeys(ctx context.Context) error {
	return r.db.DeactivateKey(ctx)
}

// Unprime the previous key
func (r *KeyRepository) UnprimeAll(ctx context.Context) error {
	return r.db.UnprimeAll(ctx)
}

func (r *KeyRepository) Delete(ctx context.Context, id string) error {
	return r.db.DeleteKey(ctx, id)
}

func toKeyDomain(key models.ApiKey) *keys.APIKey {
	return &keys.APIKey{
		ID:     key.ID,
		Key:    key.Key,
		Status: keys.ParseKeyStatus(key.Status),
	}
}
