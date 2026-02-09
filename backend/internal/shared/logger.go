package shared

import (
	"io"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func NewLogger(level zerolog.Level, output io.Writer, style string) zerolog.Logger {
	var zer zerolog.ConsoleWriter
	if output == nil {
		zer = zerolog.ConsoleWriter{Out: os.Stderr}
	}
	zer = zerolog.ConsoleWriter{Out: output}
	zerolog.SetGlobalLevel(zerolog.Level(level))
	log.Logger = log.Output(zer)
	return zerolog.New(zer).With().Timestamp().Logger()
}
