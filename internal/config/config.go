package config

import (
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

// Use one config for all services for simplicity
// Each service takes what it needs
type Config struct {
	// database settings
	DatabaseUrl       string        `env:"DATABASE_URL" env-required:"true"`
	DBMaxConns        int           `env:"DB_MAX_CONNS" env-default:"20"`
	DBMinConns        int           `env:"DB_MIN_CONNS" env-default:"5"`
	DBMaxConnLifetime time.Duration `env:"DB_MAX_CONN_LIFETIME" env-default:"30m"`

	// api server settings
	ServerAddress   string        `env:"SERVER_ADDRESS" env-default:"localhost:3000"`
	ShutdownTimeout time.Duration `env:"SHUTDOWN_TIMEOUT" env-default:"20s"`
	ReadTimeout     time.Duration `env:"READ_TIMEOUT" env-default:"5s"`
	WriteTimeout    time.Duration `env:"WRITE_TIMEOUT" env-default:"10s"`
	IdleTimeout     time.Duration `env:"IDLE_TIMEOUT" env-default:"30s"`

	// outbox-event-publisher worker settings
	WorkersNum      int           `env:"WORKERS_NUM" env-default:"5"`
	OutboxBatchSize int           `env:"OUTBOX_BATCH_SIZE" env-default:"100"`
	OutboxClaimTTL  time.Duration `env:"OUTBOX_CLAIM_TTL" env-default:"2m"`

	// kafka settings, shared by the outbox event publisher and consumer
	KafkaBrokers       []string `env:"KAFKA_BROKERS" env-default:"kafka:9092"`
	KafkaTopic         string   `env:"KAFKA_TOPIC" env-default:"order.created"`
	KafkaConsumerGroup string   `env:"KAFKA_CONSUMER_GROUP" env-default:"outbox-event-consumer"`
}

func NewConfig() (Config, error) {
	var c Config
	err := cleanenv.ReadEnv(&c)

	return c, err
}
