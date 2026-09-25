package model

import "time"

type Registration struct {
	ID           uint64 `gorm:"primaryKey"`
	UUID         string `gorm:"type:char(36);uniqueIndex;not null"`
	ActivityID   uint64 `gorm:"index;not null"`
	UserID       uint64 `gorm:"index;not null"`
	Status       uint8  `gorm:"not null;default:0"`
	ActiveFlag   *uint8 `gorm:"column:active_flag"`
	ExpiresAt    *time.Time
	ReviewedBy   *uint64
	DecidedAt    *time.Time
	CancelTime   *time.Time
	RejectReason string `gorm:"size:255;not null;default:''"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (Registration) TableName() string { return "activity_registrations" }

// Registration status log operator types (see the database design document
// section 11.2).
const (
	RegistrationOperatorUser      uint8 = 1
	RegistrationOperatorOrganizer uint8 = 2
	RegistrationOperatorAdmin     uint8 = 3
	RegistrationOperatorSystem    uint8 = 4
)

type RegistrationStatusLog struct {
	ID             uint64 `gorm:"primaryKey"`
	RegistrationID uint64 `gorm:"index;not null"`
	FromStatus     uint8  `gorm:"not null"`
	ToStatus       uint8  `gorm:"not null"`
	OperatorID     uint64 `gorm:"not null;default:0"`
	OperatorType   uint8  `gorm:"not null;default:1"`
	Reason         string `gorm:"size:500;not null;default:''"`
	TraceID        string `gorm:"size:64;not null;default:''"`
	CreatedAt      time.Time
}

func (RegistrationStatusLog) TableName() string { return "registration_status_logs" }

type Ticket struct {
	ID             uint64 `gorm:"primaryKey"`
	UUID           string `gorm:"type:char(36);uniqueIndex;not null"`
	Code           string `gorm:"size:32;uniqueIndex;not null"`
	RegistrationID uint64 `gorm:"uniqueIndex;not null"`
	ActivityID     uint64 `gorm:"index;not null"`
	UserID         uint64 `gorm:"index;not null"`
	Status         uint8  `gorm:"not null;default:0"`
	ValidStartAt   *time.Time
	ValidEndAt     *time.Time
	IssuedAt       time.Time
	UsedAt         *time.Time
	VoidedAt       *time.Time
	Version        uint32 `gorm:"not null;default:0"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (Ticket) TableName() string { return "tickets" }

type OutboxEvent struct {
	ID               uint64  `gorm:"primaryKey"`
	EventID          string  `gorm:"column:event_id;type:char(36);uniqueIndex;not null"`
	EventType        string  `gorm:"size:100;not null"`
	AggregateType    string  `gorm:"size:50;not null"`
	AggregateID      string  `gorm:"size:64;not null"`
	AggregateVersion uint32  `gorm:"not null;default:1"`
	PartitionKey     string  `gorm:"size:100;not null;default:''"`
	Payload          string  `gorm:"type:json;not null"`
	Headers          *string `gorm:"type:json"`
	Status           uint8   `gorm:"not null;default:0"`
	RetryCount       int     `gorm:"not null;default:0"`
	NextRetryAt      *time.Time
	SentAt           *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (OutboxEvent) TableName() string { return "outbox_events" }
