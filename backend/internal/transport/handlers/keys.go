package handler

import (
	"errors"
	"net/http"
	"strings"

	keyApp "quicc/online/internal/app/key"

	"github.com/labstack/echo/v4"
)

var (
	ErrMissingAuthHeader = errors.New("authorization header missing")
	ErrInvalidAuthHeader = errors.New("invalid authorization header format")
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

func ExtractBearerToken(c echo.Context) (string, error) {
	authHeader := c.Request().Header.Get(echo.HeaderAuthorization)
	if authHeader == "" {
		return "", ErrMissingAuthHeader
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", ErrInvalidAuthHeader
	}

	token := strings.TrimSpace(parts[1])
	if token == "" {
		return "", ErrInvalidAuthHeader
	}

	return token, nil
}

func (h *Handler) Verify(c echo.Context) error {
	h.log.Info().Msg("Verifying key")

	token, err := ExtractBearerToken(c)
	if err != nil {
		h.log.Warn().Err(err).Msg("Invalid/missing authorization header")
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
	}

	res, err := h.keySvc.Verify(c.Request().Context(), keyApp.VerifyKeyCommand{
		Key: token,
	})
	if err != nil {
		h.log.Error().Err(err).Msg("Unable to verify key")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "unable to verify key",
		})
	}

	return c.JSON(http.StatusOK, res)
}
