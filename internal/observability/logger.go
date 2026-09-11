package observability

import (
	"fmt"
	"os"

	"github.com/rs/zerolog"
)

type LoggerConfig struct {
	ServiceName string
	Level       string
}

func NewLogger(config LoggerConfig) (zerolog.Logger, error) {
	level, err := zerolog.ParseLevel(config.Level)
	if err != nil {
		return zerolog.Logger{}, fmt.Errorf("parse log level %q: %w", config.Level, err)
	}

	return zerolog.New(os.Stdout).
		Level(level).
		With().
		Timestamp().
		Str("service", config.ServiceName).
		Logger(), nil
}
