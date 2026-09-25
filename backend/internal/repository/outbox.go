package repository

import (
	"github.com/wangn-tech/campus-hub/internal/model"
	"gorm.io/gorm"
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
