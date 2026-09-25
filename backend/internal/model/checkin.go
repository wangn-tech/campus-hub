package model

import "time"

type CheckIn struct {
	ID              uint64   `gorm:"primaryKey"`
	UUID            string   `gorm:"type:char(36);uniqueIndex;not null"`
	TicketID        uint64   `gorm:"uniqueIndex;not null"`
	RegistrationID  uint64   `gorm:"not null"`
	ActivityID      uint64   `gorm:"not null"`
	UserID          uint64   `gorm:"not null"`
	OperatorID      uint64   `gorm:"not null"`
	ClientRequestID string   `gorm:"size:64;uniqueIndex;not null"`
	Longitude       *float64 `gorm:"type:decimal(10,7)"`
	Latitude        *float64 `gorm:"type:decimal(10,7)"`
	CheckedInAt     time.Time
	CreatedAt       time.Time
}

func (CheckIn) TableName() string { return "check_ins" }
