package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/wangn-tech/campus-hub/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	outboxMaxAttempts = 10
	outboxMaxBackoff  = 5 * time.Minute
)

// enqueueOutboxEvents writes events inside an existing transaction so they are
// committed together with the business rows they describe.
func enqueueOutboxEvents(tx *gorm.DB, events []*model.OutboxEvent) error {
	for _, event := range events {
		if event == nil {
			continue
		}
		if err := tx.Create(event).Error; err != nil {
			return err
		}
	}
	return nil
}

type OutboxRepository struct{ db *gorm.DB }

func NewOutboxRepository(db *gorm.DB) *OutboxRepository { return &OutboxRepository{db: db} }

// PublishBatch claims up to limit deliverable events, hands them to publish and
// records the outcome in the same transaction. Keeping the publish inside the
// transaction means a crash republishes instead of dropping the event, which
// matches the at-least-once contract. SKIP LOCKED lets parallel instances take
// disjoint batches.
func (r *OutboxRepository) PublishBatch(ctx context.Context, now time.Time, limit int, publish func([]model.OutboxEvent) []error) (int, int, error) {
	// READ COMMITTED matters here: under MySQL's default REPEATABLE READ the
	// SELECT ... FOR UPDATE below would take gap locks over the pending range
	// and block concurrent inserts of new outbox rows for as long as the
	// publish call takes. READ COMMITTED keeps the locking to matched rows.
	tx := r.db.WithContext(ctx).Begin(&sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if tx.Error != nil {
		return 0, 0, tx.Error
	}
	sent, failed, err := publishBatch(tx, now, limit, publish)
	if err != nil {
		_ = tx.Rollback()
		return 0, 0, err
	}
	if err := tx.Commit().Error; err != nil {
		return 0, 0, err
	}
	return sent, failed, nil
}

func publishBatch(tx *gorm.DB, now time.Time, limit int, publish func([]model.OutboxEvent) []error) (int, int, error) {
	var sent, failed int
	var events []model.OutboxEvent
	err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
		Where("(status = ? AND (next_retry_at IS NULL OR next_retry_at <= ?)) OR (status = ? AND next_retry_at IS NOT NULL AND next_retry_at <= ?)",
			model.OutboxPending, now, model.OutboxFailed, now).
		Order("id").
		Limit(limit).
		Find(&events).Error
	if err != nil {
		return 0, 0, err
	}
	if len(events) == 0 {
		return 0, 0, nil
	}
	results := publish(events)
	sentIDs := make([]uint64, 0, len(events))
	for i := range events {
		if i < len(results) && results[i] == nil {
			sentIDs = append(sentIDs, events[i].ID)
			sent++
			continue
		}
		failed++
		attempts := events[i].RetryCount + 1
		err := tx.Model(&model.OutboxEvent{}).Where("id = ?", events[i].ID).
			Updates(map[string]any{
				"status":        model.OutboxFailed,
				"retry_count":   attempts,
				"next_retry_at": nextRetryAt(now, attempts),
				"updated_at":    now,
			}).Error
		if err != nil {
			return 0, 0, err
		}
	}
	if len(sentIDs) > 0 {
		err := tx.Model(&model.OutboxEvent{}).Where("id IN ?", sentIDs).
			Updates(map[string]any{
				"status":        model.OutboxSent,
				"sent_at":       now,
				"next_retry_at": nil,
				"updated_at":    now,
			}).Error
		if err != nil {
			return 0, 0, err
		}
	}
	return sent, failed, nil
}

// nextRetryAt backs off exponentially and parks the event once the attempts run
// out, leaving it visible as status = failed for operators.
func nextRetryAt(now time.Time, attempts int) *time.Time {
	if attempts >= outboxMaxAttempts {
		return nil
	}
	backoff := time.Second << uint(attempts)
	if backoff > outboxMaxBackoff {
		backoff = outboxMaxBackoff
	}
	next := now.Add(backoff)
	return &next
}
