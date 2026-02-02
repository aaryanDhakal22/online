package keys

import (
	"context"
)

type Repository interface {
	Create(ctx context.Context, key *APIKey) error
	GetActive(ctx context.Context) (*APIKey, error)
	GetPrimed(ctx context.Context) (*APIKey, error)
	GetByID(ctx context.Context, id string) (*APIKey, error)
	Activate(ctx context.Context, id string) error
	Deactivate(ctx context.Context, id string) error
	UnprimeAll(ctx context.Context) error
	Delete(ctx context.Context, id string) error
}
