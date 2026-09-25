package repository

import (
	"context"
	"time"

	"github.com/wangn-tech/campus-hub/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type NotificationRepository struct{ db *gorm.DB }

func NewNotificationRepository(db *gorm.DB) *NotificationRepository {
	return &NotificationRepository{db: db}
}

// Create inserts a notification, ignoring a duplicate uuid. The consumer derives
// the uuid from the source event and recipient, which makes redelivered events
// idempotent without an extra columns.
// Create inserts a notification and reports whether this invocation created it.
// Consumers use the boolean to avoid broadcasting a duplicate delivery hint
// when Kafka redelivers an already processed event.
func (r *NotificationRepository) Create(ctx context.Context, notification *model.Notification, events []*model.OutboxEvent) (bool, error) {
	created := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(notification)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}
		created = true
		return enqueueOutboxEvents(tx, events)
	})
	return created, err
}

func (r *NotificationRepository) ListByUser(ctx context.Context, userID uint64, page, pageSize int) ([]model.Notification, int64, error) {
	base := func() *gorm.DB {
		return r.db.WithContext(ctx).Model(&model.Notification{}).Where("user_id = ?", userID)
	}
	var total int64
	if err := base().Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var notifications []model.Notification
	err := base().Select("notifications.*").
		Order("created_at DESC, id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&notifications).Error
	if err != nil {
		return nil, 0, err
	}
	return notifications, total, nil
}

func (r *NotificationRepository) UnreadCount(ctx context.Context, userID uint64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Notification{}).
		Where("user_id = ? AND is_read = 0", userID).
		Count(&count).Error
	return count, err
}

// MarkRead marks the given notifications of the user read. It is idempotent and
// reports how many rows it changed.
func (r *NotificationRepository) MarkRead(ctx context.Context, userID uint64, uuids []string) (int64, error) {
	if len(uuids) == 0 {
		return 0, nil
	}
	now := time.Now().UTC()
	result := r.db.WithContext(ctx).Model(&model.Notification{}).
		Where("user_id = ? AND uuid IN ? AND is_read = 0", userID, uuids).
		Updates(map[string]any{"is_read": true, "read_at": now})
	return result.RowsAffected, result.Error
}

func (r *NotificationRepository) MarkAllRead(ctx context.Context, userID uint64) (int64, error) {
	now := time.Now().UTC()
	result := r.db.WithContext(ctx).Model(&model.Notification{}).
		Where("user_id = ? AND is_read = 0", userID).
		Updates(map[string]any{"is_read": true, "read_at": now})
	return result.RowsAffected, result.Error
}
