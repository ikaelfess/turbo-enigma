package helpers

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-openapi/testify/v2/require"
)

func FindProjectRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	require.NoError(t, err)

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		require.NotEqual(t, parent, dir, "project root not found")

		dir = parent
	}
}
