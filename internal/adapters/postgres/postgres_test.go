package postgres

import "testing"

func TestDatabaseName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		databaseURL string
		expected    string
	}{
		{
			name:        "postgres url",
			databaseURL: "postgres://outbox:outbox@postgres:5432/outbox?sslmode=disable",
			expected:    "outbox",
		},
		{
			name:        "invalid url",
			databaseURL: "://",
			expected:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := databaseName(tt.databaseURL); got != tt.expected {
				t.Fatalf("databaseName(%q) = %q, expected %q", tt.databaseURL, got, tt.expected)
			}
		})
	}
}
