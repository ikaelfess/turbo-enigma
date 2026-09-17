package helpers

import (
	"testing"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

func FindBy[T any](t *testing.T, db *bun.DB, where string, args ...any) (*T, error) {
	t.Helper()

	var model T
	err := db.NewSelect().
		Model(&model).
		Where(where, args...).
		Limit(1).
		Scan(t.Context())

	return &model, err
}

func Find[T any](t *testing.T, db *bun.DB, id uuid.UUID) (*T, error) {
	t.Helper()

	return FindBy[T](t, db, "id = ?", id)
}

func FindAll[T any](t *testing.T, db *bun.DB) ([]T, error) {
	t.Helper()

	var models []T
	err := db.
		NewSelect().
		Model(&models).
		Scan(t.Context())

	return models, err
}
