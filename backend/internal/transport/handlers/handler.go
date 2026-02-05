package handler

import (
	keyApp "quicc/online/internal/app/key"
	orderApp "quicc/online/internal/app/order"

	"github.com/rs/zerolog"
)

type Handler struct {
	keySvc   *keyApp.KeyService
	orderSvc *orderApp.OrderService
	log      zerolog.Logger
}

func NewHandler(keySvc *keyApp.KeyService, orderSvc *orderApp.OrderService, logger zerolog.Logger) *Handler {
	log := logger.With().Str("module", "handler").Logger()
	return &Handler{
		keySvc:   keySvc,
		orderSvc: orderSvc,
		log:      log,
	}
}
