package repository

import (
	"context"
	"errors"
	"time"

	"github.com/wangn-tech/campus-hub/internal/model"
	"gorm.io/gorm"
)

// ErrTicketNotUsable is returned when the ticket was not in the unused state,
// which also covers a repeated check-in of the same ticket.
var ErrTicketNotUsable = errors.New("ticket is not usable")

type CheckInRepository struct{ db *gorm.DB }

func NewCheckInRepository(db *gorm.DB) *CheckInRepository { return &CheckInRepository{db: db} }

// CheckIn marks the ticket used and records the check-in in one transaction.
func (r *CheckInRepository) CheckIn(ctx context.Context, record *model.CheckIn, events []*model.OutboxEvent) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		result := tx.Model(&model.Ticket{}).
			Where("id = ? AND status = ?", record.TicketID, uint8(model.TicketUnused)).
			Updates(map[string]any{
				"status":     uint8(model.TicketUsed),
				"used_at":    now,
				"updated_at": now,
				"version":    gorm.Expr("version + 1"),
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrTicketNotUsable
		}
		if err := tx.Create(record).Error; err != nil {
			return err
		}
		return enqueueOutboxEvents(tx, events)
	})
}

func (r *CheckInRepository) FindByRequestID(ctx context.Context, requestID string) (*model.CheckIn, error) {
	var record model.CheckIn
	if err := r.db.WithContext(ctx).Where("client_request_id = ?", requestID).First(&record).Error; err != nil {
		return nil, err
	}
	return &record, nil
}

func (r *CheckInRepository) ListByActivity(ctx context.Context, activityID uint64, page, pageSize int) ([]model.CheckIn, int64, error) {
	base := func() *gorm.DB {
		return r.db.WithContext(ctx).Model(&model.CheckIn{}).Where("activity_id = ?", activityID)
	}
	var total int64
	if err := base().Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var records []model.CheckIn
	err := base().
		Order("checked_in_at DESC, id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&records).Error
	if err != nil {
		return nil, 0, err
	}
	return records, total, nil
}
