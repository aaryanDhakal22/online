package orderApp

import (
	"context"

	"quicc/online/internal/domain/order"
	"quicc/online/internal/infra/message"

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

func NewOrderService(orderRepo order.Repository, mb *message.MessageBroker, logger zerolog.Logger) *OrderService {
	logger = logger.With().Str("service", "order").Logger()
	return &OrderService{
		orderRepo: orderRepo,
		mb:        mb,
		logger:    logger,
	}
}

func (s *OrderService) Create(cmd CreateOrderCommand) (*CreateOrderResult, error) {
	order := order.NewOrder(cmd.OrderID, cmd.Payload, cmd.DateCreated, cmd.CreatedAt)
	defer s.logger.Info().Msg("Order was successfully created.")
	err := s.orderRepo.Create(context.TODO(), order)
	if err != nil {
		s.logger.Error().Err(err).Msg("Error creating order")
		return nil, err
	}
	defer s.logger.Info().Msg("Order was successfully created.")
	return &CreateOrderResult{ID: order.ID}, nil
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
