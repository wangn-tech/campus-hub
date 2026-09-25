package repository

import (
	"context"
	"github.com/google/uuid"
	"github.com/wangn-tech/campus-hub/internal/model"
	"gorm.io/gorm"
)

type VerificationRepository struct{ db *gorm.DB }

func NewVerificationRepository(db *gorm.DB) *VerificationRepository {
	return &VerificationRepository{db: db}
}
func (r *VerificationRepository) Current(ctx context.Context, userID uint64) (*model.StudentVerification, error) {
	var v model.StudentVerification
	if err := r.db.WithContext(ctx).Where("user_id=?", userID).First(&v).Error; err != nil {
		return nil, err
	}
	return &v, nil
}
func (r *VerificationRepository) Create(ctx context.Context, v *model.StudentVerification, e *model.StudentVerificationEvent, outbox *model.OutboxEvent) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(v).Error; err != nil {
			return err
		}
		e.VerificationID = v.ID
		if err := tx.Create(e).Error; err != nil {
			return err
		}
		return tx.Create(outbox).Error
	})
}
func (r *VerificationRepository) Transition(ctx context.Context, v *model.StudentVerification, to uint8, event string, reason string, trace string, outbox *model.OutboxEvent) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		from := v.Status
		if err := tx.Model(v).Updates(map[string]any{"status": to}).Error; err != nil {
			return err
		}
		if err := tx.Create(&model.StudentVerificationEvent{UUID: uuid.NewString(), VerificationID: v.ID, UserID: v.UserID, FromStatus: from, ToStatus: to, EventType: event, OperatorID: v.UserID, OperatorType: 1, Reason: reason, TraceID: trace}).Error; err != nil {
			return err
		}
		return tx.Create(outbox).Error
	})
}
