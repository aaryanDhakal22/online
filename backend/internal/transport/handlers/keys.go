package handler

import (
	"errors"
	"net/http"
	"strings"

	keyApp "quicc/online/internal/app/key"

	"github.com/labstack/echo/v5"
)

var (
	ErrMissingAuthHeader = errors.New("authorization header missing")
	ErrInvalidAuthHeader = errors.New("invalid authorization header format")
)

// Generate stages a new Primed key. Open (no admin gate) so a sender can
// pull a fresh key without operator help — they then ping the operator to
// activate it via /set.
func (h *Handler) Generate(c *echo.Context) error {
	ctx := c.Request().Context()
	h.log.Debug().Ctx(ctx).Msg("handler.generate: invoked")
	res, err := h.keySvc.Generate(ctx, keyApp.GenerateKeyCommand{})
	if err != nil {
		h.log.Error().Err(err).Ctx(ctx).Msg("handler.generate: failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "unable to generate key"})
	}
	h.log.Debug().Ctx(ctx).Str("key_id", res.ID).Msg("handler.generate: returned key to sender")
	return c.JSON(http.StatusOK, res)
}

// Set activates the most recently primed key. Admin-gated.
func (h *Handler) Set(c *echo.Context) error {
	ctx := c.Request().Context()
	h.log.Debug().Ctx(ctx).Msg("handler.set: invoked")
	res, err := h.keySvc.Set(ctx)
	if err != nil {
		h.log.Error().Err(err).Ctx(ctx).Msg("handler.set: failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "unable to set key"})
	}
	h.log.Debug().Ctx(ctx).Str("key_id", res.ID).Msg("handler.set: primed key activated")
	return c.JSON(http.StatusOK, res)
}

// SetKey is the break-glass path: admin pastes a known plaintext (e.g. one
// the sender already has configured in their CLI) and we activate it
// directly. Bypasses the Primed staging step.
func (h *Handler) SetKey(c *echo.Context) error {
	ctx := c.Request().Context()
	var cmd keyApp.SetByKeyCommand
	if err := c.Bind(&cmd); err != nil {
		h.log.Warn().Err(err).Ctx(ctx).Msg("handler.setKey: bind failed")
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "unable to parse request body"})
	}
	if cmd.Key == "" {
		h.log.Warn().Ctx(ctx).Msg("handler.setKey: empty key in body")
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "key is required"})
	}
	h.log.Debug().Ctx(ctx).Int("key_len", len(cmd.Key)).Msg("handler.setKey: invoking SetByKey")

	res, err := h.keySvc.SetByKey(ctx, cmd.Key)
	if err != nil {
		h.log.Error().Err(err).Ctx(ctx).Msg("handler.setKey: failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "unable to set key"})
	}
	h.log.Debug().Ctx(ctx).Str("key_id", res.ID).Msg("handler.setKey: break-glass key activated")
	return c.JSON(http.StatusOK, res)
}

func ExtractBearerToken(c *echo.Context) (string, error) {
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

// Verify reports whether a Bearer token matches the active key.
func (h *Handler) Verify(c *echo.Context) error {
	ctx := c.Request().Context()

	token, err := ExtractBearerToken(c)
	if err != nil {
		h.log.Debug().Err(err).Ctx(ctx).Msg("handler.verify: bad header")
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	match, err := h.keySvc.Verify(ctx, token)
	if err != nil {
		h.log.Error().Err(err).Ctx(ctx).Msg("handler.verify: failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "unable to verify key"})
	}
	h.log.Debug().Ctx(ctx).Bool("match", match).Msg("handler.verify: comparison complete")
	return c.JSON(http.StatusOK, keyApp.VerifyKeyResult{Match: match})
}

// GetKey returns the active key plaintext (admin-only).
func (h *Handler) GetKey(c *echo.Context) error {
	ctx := c.Request().Context()
	res, err := h.keySvc.Get(ctx)
	if err != nil {
		h.log.Error().Err(err).Ctx(ctx).Msg("handler.getKey: failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "unable to get key"})
	}
	h.log.Debug().Ctx(ctx).Msg("handler.getKey: served")
	return c.JSON(http.StatusOK, res)
}
