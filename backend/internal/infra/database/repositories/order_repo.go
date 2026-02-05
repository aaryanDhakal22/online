package repositories

import (
	"context"
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

func (r *OrderRepository) Create(ctx context.Context, order *models.Order) error {
	_, err := r.db.CreateOrder(ctx, models.CreateOrderParams{
		ID:          order.ID,
		Payload:     order.Payload,
		DateCreated: order.DateCreated,
	})
	return err
}

func (r *OrderRepository) GetLatest(ctx context.Context) (*models.Order, error) {
	order, err := r.db.GetLatestOrder(ctx)
	if err != nil {
		return nil, err
	}
	return &order, nil
}

func (r *OrderRepository) GetByID(ctx context.Context, id string) (*models.Order, error) {
	order, err := r.db.GetOrderByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return &order, nil
}

func (r *OrderRepository) Delete(ctx context.Context, id string) error {
	return r.db.DeleteOrder(ctx, id)
}
