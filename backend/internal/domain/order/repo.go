package order

import "context"

type Repository interface {
	Create(ctx context.Context, order *Order) error
	GetByID(ctx context.Context, id int64) (*Order, error)
	GetLatest(ctx context.Context) (*Order, error)
	GetAllToday(ctx context.Context) ([]*Order, error)
	Delete(ctx context.Context, id int64) error
}
