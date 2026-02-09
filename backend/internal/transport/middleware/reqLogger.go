package custom_middlewares

import (
	"time"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
)

const (
	Red    = "\033[31m"
	Green  = "\033[32m"
	Yellow = "\033[33m"
	Reset  = "\033[0m"
)

// NewSimpleReqLogger logs: time (Mon dd HH:MM:SS), request_id, real_ip, method, path
func NewSimpleReqLogger(l zerolog.Logger) echo.MiddlewareFunc {
	const timeFmt = "Jan 02 15:04:05"

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := c.Request()

			// Try common places for request id (works whether you set it earlier or later)
			requestID := firstNonEmpty(
				req.Header.Get("X-Request-ID"),
				req.Header.Get(echo.HeaderXRequestID),
				c.Response().Header().Get("X-Request-ID"),
				c.Response().Header().Get(echo.HeaderXRequestID),
			)

			l.Info().
				Msgf("%v %s : %s%s%s %s%s%s %s",
					time.Now().Format(timeFmt),
					c.RealIP(),
					Green,
					req.Method,
					Reset,
					Green,
					req.URL.Path,
					Reset,
					requestID,
				)

			// l.Info().
			// 		Str("time", time.Now().Format(timeFmt)).
			// 		Str("request_id", requestID).
			// 		Str("real_ip", c.RealIP()).
			// 		Str("method", req.Method).
			// 		Str("path", req.URL.Path).
			// 		Msg("request")
			//
			return next(c)
		}
	}
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}
