package repositories

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	keys "quicc/online/internal/domain/key"
	models "quicc/online/internal/infra/database/models"

	"github.com/rs/zerolog"
)

type KeyRepository struct {
	db     *models.Queries
	logger zerolog.Logger
}

func NewKeyRepository(db *models.Queries, logger zerolog.Logger) *KeyRepository {
	return &KeyRepository{
		db:     db,
		logger: logger.With().Str("component", "key_repo").Logger(),
	}
}

func (r *KeyRepository) Create(ctx context.Context, key *keys.APIKey) error {
	return mapErr(r.db.CreateKey(ctx, models.CreateKeyParams{
		ID:        key.ID,
		Key:       key.Key,
		Status:    string(key.Status),
		CreatedAt: fmtTime(key.CreatedAt),
	}))
}

func (r *KeyRepository) ListActive(ctx context.Context) ([]*keys.APIKey, error) {
	rows, err := r.db.ListActiveKeys(ctx)
	if err != nil {
		return nil, err
	}
	return toKeyDomains(rows), nil
}

func (r *KeyRepository) List(ctx context.Context) ([]*keys.APIKey, error) {
	rows, err := r.db.ListKeys(ctx)
	if err != nil {
		return nil, err
	}
	return toKeyDomains(rows), nil
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
	if errors.Is(err, sql.ErrNoRows) {
		return nil, keys.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return toKeyDomain(key), nil
}

func (r *KeyRepository) Activate(ctx context.Context, id string) error {
	return mapErr(r.db.ActivateKey(ctx, id))
}

func (r *KeyRepository) Retire(ctx context.Context, id string) error {
	n, err := r.db.RetireKey(ctx, id)
	if err != nil {
		return mapErr(err)
	}
	if n == 0 {
		return keys.ErrNotActive
	}
	return nil
}

func (r *KeyRepository) TouchLastUsed(ctx context.Context, id string, at time.Time) error {
	return r.db.TouchKeyLastUsed(ctx, models.TouchKeyLastUsedParams{ID: id, LastUsedAt: fmtTime(at)})
}

func (r *KeyRepository) UnprimeAll(ctx context.Context) error {
	return r.db.UnprimeAll(ctx)
}

// mapErr turns the key-guard trigger's RAISE(ABORT, 'last_active_key') into
// keys.ErrLastActiveKey.
func mapErr(err error) error {
	if err != nil && strings.Contains(err.Error(), "last_active_key") {
		return keys.ErrLastActiveKey
	}
	return err
}

func fmtTime(t time.Time) sql.NullString {
	if t.IsZero() {
		return sql.NullString{}
	}
	return sql.NullString{String: t.UTC().Format(keys.TimeLayout), Valid: true}
}

func parseTime(s sql.NullString) time.Time {
	if !s.Valid || s.String == "" {
		return time.Time{}
	}
	t, err := time.Parse(keys.TimeLayout, s.String)
	if err != nil {
		// Older rows may carry plain RFC3339.
		if t2, err2 := time.Parse(time.RFC3339Nano, s.String); err2 == nil {
			return t2.UTC()
		}
		return time.Time{}
	}
	return t
}

func toKeyDomains(rows []models.ApiKey) []*keys.APIKey {
	out := make([]*keys.APIKey, 0, len(rows))
	for _, k := range rows {
		out = append(out, toKeyDomain(k))
	}
	return out
}

func toKeyDomain(key models.ApiKey) *keys.APIKey {
	return &keys.APIKey{
		ID:         key.ID,
		Key:        key.Key,
		Status:     keys.ParseKeyStatus(key.Status),
		CreatedAt:  parseTime(key.CreatedAt),
		LastUsedAt: parseTime(key.LastUsedAt),
	}
}
