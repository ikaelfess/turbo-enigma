package helpers

import (
	"testing"

	"github.com/go-openapi/testify/v2/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/kafka"
)

const (
	KafkaImage = "confluentinc/confluent-local:8.3.2"
	ClusterID  = "0DPxTQhKQemkY6QPeHeaNw"
)

func WithKafkaContainer(t *testing.T, fn func(t *testing.T, brokers []string)) {
	t.Helper()

	container, err := kafka.Run(
		t.Context(),
		KafkaImage,
		kafka.WithClusterID(ClusterID),
		testcontainers.WithEnv(map[string]string{
			"KAFKA_AUTO_CREATE_TOPICS_ENABLE": "true",
		}),
	)
	require.NoError(t, err, "kafka container run")

	brokers, err := container.Brokers(t.Context())
	require.NoError(t, err, "kafka brokers")

	t.Cleanup(func() {
		require.NoError(
			t,
			testcontainers.TerminateContainer(container),
			"kafka container terminate",
		)
	})

	fn(t, brokers)
}
