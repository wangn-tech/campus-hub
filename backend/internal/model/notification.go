package model

import "time"

type Notification struct {
	ID        uint64  `gorm:"primaryKey"`
	UUID      string  `gorm:"type:char(36);uniqueIndex;not null"`
	UserID    uint64  `gorm:"index;not null"`
	Type      string  `gorm:"size:32;not null"`
	Title     string  `gorm:"size:255;not null"`
	Content   string  `gorm:"type:text;not null"`
	Data      *string `gorm:"type:json"`
	IsRead    bool    `gorm:"not null;default:false"`
	ReadAt    *time.Time
	CreatedAt time.Time
}

func (Notification) TableName() string { return "notifications" }
