package repositories

import (
	"context"

	"quicc/online/internal/domain/order"
	models "quicc/online/internal/infra/database/models"

	"github.com/rs/zerolog"
)

type OrderRepository struct {
	db     *models.Queries
	logger zerolog.Logger
}

func NewOrderRepository(db *models.Queries, logger zerolog.Logger) *OrderRepository {
	dbLogger := logger.With().Str("component", "repository").Logger()
	return &OrderRepository{
		db:     db,
		logger: dbLogger,
	}
}

func toOrderDomain(od *models.Order) (*order.Order, error) {
	return &order.Order{
		ID:          od.ID,
		Payload:     od.Payload,
		DateCreated: od.DateCreated,
	}, nil
}

func (r *OrderRepository) Create(ctx context.Context, od *order.Order) error {
	_, err := r.db.CreateOrder(ctx, models.CreateOrderParams{
		ID:          od.ID,
		Payload:     od.Payload,
		DateCreated: od.DateCreated,
	})
	return err
}

func (r *OrderRepository) GetLatest(ctx context.Context) (*order.Order, error) {
	od, err := r.db.GetLatestOrder(ctx)
	if err != nil {
		return nil, err
	}
	return toOrderDomain(&od)
}

func (r *OrderRepository) GetByID(ctx context.Context, id string) (*order.Order, error) {
	order, err := r.db.GetOrderByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return toOrderDomain(&order)
}

func (r *OrderRepository) Delete(ctx context.Context, id string) error {
	return r.db.DeleteOrder(ctx, id)
}
