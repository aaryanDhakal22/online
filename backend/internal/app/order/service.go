package orderApp

import (
	"context"
	"fmt"
	"quicc/online/internal/domain/order"
	"time"

	"github.com/rs/zerolog"
)

type EventPublisher interface {
	Publish(OrderID string, order order.Order) error
}

type OrderService struct {
	orderRepo order.Repository
	mb        EventPublisher
	logger    zerolog.Logger
}

func NewOrderService(orderRepo order.Repository, mb EventPublisher, logger zerolog.Logger) *OrderService {
	logger = logger.With().Str("service", "order").Logger()
	return &OrderService{
		orderRepo: orderRepo,
		mb:        mb,
		logger:    logger,
	}
}

func (s *OrderService) Create(cmd CreateOrderCommand) (*CreateOrderResult, error) {
	s.logger.Info().Msg("Creating order")
	order := order.NewOrder(cmd.OrderID, cmd.Payload, cmd.DateCreated, cmd.CreatedAt)
	err := s.orderRepo.Create(context.TODO(), order)
	if err != nil {
		s.logger.Error().Err(err).Msg("Error creating order")
		return nil, err
	}
	s.logger.Info().Msg("Order was successfully created.")
	return &CreateOrderResult{
		Status:          "ordered",
		ExtOrderID:      fmt.Sprintf("brygid-%v", order.ID),
		OrderPlacedTime: time.Now().Format(time.RFC3339),
	}, nil
}

func (s *OrderService) RelayOrder(cmd RelayOrderCommand) error {
	s.logger.Info().Msg("Relaying order")
	if err := s.mb.Publish(cmd.OrderID, cmd.Order); err != nil {
		s.logger.Error().Err(err).Msg("Error publishing order")
		return err
	}
	s.logger.Info().Msg("Order was successfully published.")
	return nil
}
