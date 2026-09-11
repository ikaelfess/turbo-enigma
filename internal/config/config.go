package config

import (
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

// Use one config for all services for simplicity
// Each service takes what it needs
type Config struct {
	ServerAddress string `env:"SERVER_ADDRESS" env-default:"localhost:3000"`

	DatabaseUrl       string        `env:"DATABASE_URL" env-required:"true"`
	DBMaxConns        int           `env:"DB_MAX_CONNS" env-default:"5"`
	DBMinConns        int           `env:"DB_MIN_CONNS" env-default:"1"`
	DBMaxConnLifetime time.Duration `env:"DB_MAX_CONN_LIFETIME" env-default:"1m"`

	ShutdownTimeout time.Duration `env:"SHUTDOWN_TIMEOUT" env-default:"20s"`
	ReadTimeout     time.Duration `env:"READ_TIMEOUT" env-default:"5s"`
	WriteTimeout    time.Duration `env:"WRITE_TIMEOUT" env-default:"10s"`
	IdleTimeout     time.Duration `env:"IDLE_TIMEOUT" env-default:"30s"`
}

func NewConfig() (Config, error) {
	var c Config
	err := cleanenv.ReadEnv(&c)

	return c, err
}
