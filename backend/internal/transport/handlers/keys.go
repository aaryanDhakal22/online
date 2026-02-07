package handler

import (
	"net/http"

	keyApp "quicc/online/internal/app/key"

	"github.com/labstack/echo/v4"
)

func (h *Handler) Generate(c echo.Context) error {
	h.log.Info().Msg("Generating new key")
	newKey, err := h.keySvc.Generate(c.Request().Context(), keyApp.GenerateKeyCommand{})
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
	key, err := h.keySvc.Set(c.Request().Context(), keyApp.SetKeyCommand{})
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
	h.log.Info().Msg("Verifying key")

	// Get the bearer token from the request header
	token := c.Request().Header.Get("Authorization")
	if token == "" {
		h.log.Error().Msg("No token provided")
		return c.String(http.StatusBadRequest, "No token provided")
	}
	// Split the token into the bearer and the token
	bearer, token := token[0:6], token[7:]
	if bearer != "Bearer" {
		h.log.Error().Msg("Invalid token")
		return c.String(http.StatusBadRequest, "Invalid token")
	}
	// Verify the token
	res, err := h.keySvc.Verify(c.Request().Context(), keyApp.VerifyKeyCommand{
		Key: token,
	})
	if err != nil {
		h.log.Error().Err(err).Msg("Unable to verify key")
		return c.String(http.StatusInternalServerError, "Unable to verify key")
	}

	return c.JSON(http.StatusOK, res)
}
