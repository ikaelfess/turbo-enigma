package postgres

import "github.com/jackc/pgx/v5/pgxpool"

type OutboxEventRepository struct {
	db *pgxpool.Pool
}

func NewOutboxEventRepository(db *pgxpool.Pool) *OutboxEventRepository {
	return &OutboxEventRepository{db: db}
}
