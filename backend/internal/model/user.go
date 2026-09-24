package model

import "time"

type User struct {
	ID           uint64     `gorm:"primaryKey"`
	UUID         string     `gorm:"type:char(36);uniqueIndex;not null"`
	Email        string     `gorm:"size:100;uniqueIndex;not null"`
	PasswordHash string     `gorm:"size:255;not null"`
	Nickname     string     `gorm:"size:50;not null"`
	AvatarURL    string     `gorm:"size:500;not null;default:''"`
	Introduction string     `gorm:"size:500;not null;default:''"`
	Gender       uint8      `gorm:"not null;default:0"`
	Birthday     *time.Time `gorm:"type:date"`
	Status       uint8      `gorm:"not null;default:1"`
	LastLoginAt  *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    *time.Time `gorm:"index"`
}

func (User) TableName() string { return "users" }

type Role struct {
	ID          uint64 `gorm:"primaryKey"`
	Code        string `gorm:"size:50;uniqueIndex;not null"`
	Name        string `gorm:"size:50;not null"`
	Description string `gorm:"size:200;not null;default:''"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (Role) TableName() string { return "roles" }
