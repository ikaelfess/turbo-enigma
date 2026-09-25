package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewConfig_loadsWithoutUptraceDSN(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://outbox:outbox@postgres:5432/outbox?sslmode=disable")

	cfg, err := NewConfig()

	require.NoError(t, err)
	require.Equal(t, "postgres://outbox:outbox@postgres:5432/outbox?sslmode=disable", cfg.DatabaseUrl)
}
