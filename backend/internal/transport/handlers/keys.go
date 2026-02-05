package handler

import (
	"net/http"
	keyApp "quicc/online/internal/app/key"

	"github.com/labstack/echo/v4"
)

func (h *Handler) Generate(c echo.Context) error {
	h.log.Info().Msg("Generating new key")
	newKey, err := h.keySvc.Generate(keyApp.GenerateKeyCommand{})
	if err != nil {
		return c.String(http.StatusInternalServerError, "Unable to generate key")
	}
	h.log.Info().Msg("Key generated")
	defer h.log.Info().Msg("Key generated")
	return c.JSON(http.StatusOK, keyApp.GenerateKeyResult{
		ID:  newKey.ID,
		Key: newKey.Key,
	})
}

func (h *Handler) Set(c echo.Context) error {
	h.log.Info().Msg("Setting key")
	key, err := h.keySvc.Set(keyApp.SetKeyCommand{})
	if err != nil {
		h.log.Error().Err(err).Msg("Unable to set key")
		return c.String(http.StatusInternalServerError, "Unable to set key")
	}

	h.log.Info().Msg("Key set")
	defer h.log.Info().Msg("Key returned")

	return c.JSON(http.StatusOK, keyApp.SetKeyResult{
		ID:  key.ID,
		Key: key.Key,
	})
}

func (h *Handler) Verify(c echo.Context) error {
	return c.String(http.StatusOK, "Verified")
}
