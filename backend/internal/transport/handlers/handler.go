package handler

import (
	keyApp "quicc/online/internal/app/key"
	orderApp "quicc/online/internal/app/order"
	"quicc/online/internal/infra/notify"

	"github.com/rs/zerolog"
)

type Handler struct {
	keySvc   *keyApp.KeyService
	orderSvc *orderApp.OrderService
	notifier *notify.Notifier
	log      zerolog.Logger
}

func NewHandler(keySvc *keyApp.KeyService, orderSvc *orderApp.OrderService, notifier *notify.Notifier, logger zerolog.Logger) *Handler {
	log := logger.With().Str("module", "handler").Logger()
	return &Handler{
		keySvc:   keySvc,
		orderSvc: orderSvc,
		notifier: notifier,
		log:      log,
	}
}
