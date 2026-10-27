package keys

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrLastActiveKey: the change would leave no active key (the DB
	// triggers in 002_key_guard.sql raise it too).
	ErrLastActiveKey = errors.New("last_active_key")
	// ErrNewKeyUnconfirmed: no other active key has been used by the vendor
	// since it (and the key being retired) were added.
	ErrNewKeyUnconfirmed = errors.New("new_key_unconfirmed")
	// ErrNotFound: no key with that id.
	ErrNotFound = errors.New("not_found")
	// ErrNotActive: the key exists but isn't active.
	ErrNotActive = errors.New("not_active")
)

type Repository interface {
	Create(ctx context.Context, key *APIKey) error
	// ListActive returns every active key, newest first.
	ListActive(ctx context.Context) ([]*APIKey, error)
	// List returns every key (any status), newest first.
	List(ctx context.Context) ([]*APIKey, error)
	GetPrimed(ctx context.Context) (*APIKey, error)
	GetByID(ctx context.Context, id string) (*APIKey, error)
	Activate(ctx context.Context, id string) error
	// Retire deactivates one active key. ErrNotActive if it wasn't active,
	// ErrLastActiveKey if it is the only one.
	Retire(ctx context.Context, id string) error
	TouchLastUsed(ctx context.Context, id string, at time.Time) error
	UnprimeAll(ctx context.Context) error
}
