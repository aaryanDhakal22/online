package orderApp

import (
	"context"
	"quicc/online/internal/domain/order"

	"github.com/rs/zerolog"
)

type OrderService struct {
	orderRepo order.Repository
	logger    zerolog.Logger
}

func NewOrderService(orderRepo order.Repository, logger zerolog.Logger) *OrderService {
	logger = logger.With().Str("service", "order").Logger()
	return &OrderService{
		orderRepo: orderRepo,
		logger:    logger,
	}
}

func (s *OrderService) Create(cmd CreateOrderCommand) (*CreateOrderResult, error) {
	order := order.NewOrder(cmd.OrderID, cmd.payload, cmd.DateCreated, cmd.CreatedAt)
	defer s.logger.Info().Msg("Order was successfully created.")
	err := s.orderRepo.Create(context.TODO(), order)
	if err != nil {
		s.logger.Error().Err(err).Msg("Error creating order")
		return nil, err
	}
	defer s.logger.Info().Msg("Order was successfully created.")
	return &CreateOrderResult{ID: order.ID}, nil
}
