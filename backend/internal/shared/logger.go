package shared

import (
	"os"

	"github.com/rs/zerolog"
)

// NewLogger builds a zerolog.Logger writing to os.Stderr. Style "json" emits
// raw JSON; anything else uses zerolog's ConsoleWriter for human-readable
// output during dev. File-based logging was removed — log shipping happens
// via the OTLP logs bridge in internal/observability/zlog.go.
func NewLogger(level zerolog.Level, style string) zerolog.Logger {
	zerolog.SetGlobalLevel(level)
	if style == "json" {
		return zerolog.New(os.Stderr).Level(level).With().Timestamp().Logger()
	}
	return zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).
		Level(level).
		With().
		Timestamp().
		Logger()
}
