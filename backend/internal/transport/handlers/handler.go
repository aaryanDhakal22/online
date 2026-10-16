package handler

import (
	keyApp "quicc/online/internal/app/key"
	orderApp "quicc/online/internal/app/order"
	"quicc/online/internal/observability"

	"github.com/rs/zerolog"
)

type Handler struct {
	keySvc   *keyApp.KeyService
	orderSvc *orderApp.OrderService
	meters   observability.Meters
	log      zerolog.Logger
}

func NewHandler(keySvc *keyApp.KeyService, orderSvc *orderApp.OrderService, meters observability.Meters, logger zerolog.Logger) *Handler {
	return &Handler{
		keySvc:   keySvc,
		orderSvc: orderSvc,
		meters:   meters,
		log:      logger.With().Str("module", "handler").Logger(),
	}
}
