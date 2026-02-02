package handler

import (
	"net/http"

	keyApp "quicc/online/internal/app/key"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
)

type Handler struct {
	keySvc *keyApp.KeyService
	log    zerolog.Logger
}

func NewHandler(keySvc *keyApp.KeyService, logger zerolog.Logger) *Handler {
	return &Handler{
		keySvc: keySvc,
		log:    logger,
	}
}

func (h *Handler) Generate(c echo.Context) error {
	newKey, err := h.keySvc.Generate(keyApp.GenerateKeyCommand{})
	if err != nil {
		return c.String(http.StatusInternalServerError, "Unable to generate key")
	}
	return c.JSON(http.StatusOK, keyApp.GenerateKeyResult{
		ID:  newKey.ID,
		Key: newKey.Key,
	})
}
