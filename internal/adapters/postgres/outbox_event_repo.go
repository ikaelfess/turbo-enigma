package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/ikaelfess/transactional-outbox/internal/config"
	"github.com/ikaelfess/transactional-outbox/internal/domain"
)

type OutboxEventRepo struct {
	db       bun.IDB
	claimTTL time.Duration
}

func NewOutboxEventRepo(db bun.IDB) *OutboxEventRepo {
	return &OutboxEventRepo{db: db}
}

func NewPublisherOutboxEventRepo(db *bun.DB, cfg config.Config) *OutboxEventRepo {
	return &OutboxEventRepo{
		db:       db,
		claimTTL: cfg.OutboxClaimTTL,
	}
}

func (o *OutboxEventRepo) Create(
	ctx context.Context,
	event domain.OutboxEvent,
) (domain.OutboxEvent, error) {
	eventModel := NewOutboxEvent(&event)
	_, err := o.db.NewInsert().
		Model(eventModel).
		Exec(ctx)
	if err != nil {
		return domain.OutboxEvent{}, fmt.Errorf("create outbox event: %w", err)
	}

	event.ID = eventModel.ID
	return event, nil
}

func (o *OutboxEventRepo) FindUnpublishedBatch(
	ctx context.Context,
	limit int,
) ([]domain.OutboxEvent, error) {
	db, ok := o.db.(*bun.DB)
	if !ok {
		return nil, fmt.Errorf("find unpublished batch: requires *bun.DB")
	}

	var models []OutboxEvent
	err := db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		claimTTL := fmt.Sprintf("%d microseconds", o.claimTTL.Microseconds())

		toClaim := tx.NewSelect().
			Model((*OutboxEvent)(nil)).
			Column("id").
			Where("published_at IS NULL").
			Where("(claimed_until IS NULL OR claimed_until < NOW())").
			Order("created_at ASC").
			Limit(limit).
			For("UPDATE SKIP LOCKED")

		claimed := tx.NewUpdate().
			Model((*OutboxEvent)(nil)).
			TableExpr("to_claim").
			Set("claimed_until = NOW() + ?::interval", claimTTL).
			Where("outbox_event.id = to_claim.id").
			Returning("?TableAlias.*")

		return tx.NewSelect().
			With("to_claim", toClaim).
			With("claimed", claimed).
			Model(&models).
			ModelTableExpr("claimed AS outbox_event").
			Order("created_at ASC").
			Scan(ctx)
	})
	if err != nil {
		return nil, fmt.Errorf("find unpublished batch: %w", err)
	}

	events := make([]domain.OutboxEvent, 0, len(models))
	for i := range models {
		events = append(events, *models[i].ToDomain())
	}

	return events, nil
}

func (o *OutboxEventRepo) MarkPublished(ctx context.Context, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}

	_, err := o.db.NewUpdate().
		Model((*OutboxEvent)(nil)).
		Set("published_at = NOW()").
		Where("id IN (?)", bun.List(ids)).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("mark published: %w", err)
	}

	return nil
}
