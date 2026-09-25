package model

import "time"

// RoleAdmin is the role code for platform administrators.
const RoleAdmin = "admin"

type User struct {
	ID           uint64     `gorm:"primaryKey"`
	UUID         string     `gorm:"type:char(36);uniqueIndex;not null"`
	Email        string     `gorm:"size:100;uniqueIndex;not null"`
	PasswordHash string     `gorm:"size:255;not null"`
	Nickname     string     `gorm:"size:50;not null"`
	AvatarFileID *uint64    `gorm:"index"`
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

type Tag struct {
	ID     uint64 `gorm:"primaryKey" json:"-"`
	UUID   string `gorm:"type:char(36);uniqueIndex;not null" json:"id"`
	Name   string `gorm:"size:50;not null" json:"name"`
	Slug   string `gorm:"size:80;not null" json:"slug"`
	Color  string `gorm:"size:20;not null;default:''" json:"color"`
	Icon   string `gorm:"size:255;not null;default:''" json:"icon"`
	Status uint8  `gorm:"not null;default:1" json:"status"`
}

func (Tag) TableName() string { return "tags" }

type File struct {
	ID            uint64 `gorm:"primaryKey"`
	UUID          string `gorm:"type:char(36);uniqueIndex;not null"`
	StorageDriver string
	Bucket        string
	ObjectKey     string
	URL           string
	OriginName    string
	BizType       string
	FileSize      uint64
	MIMEType      string
	Extension     string
	SHA256        string
	UploaderID    uint64
	Status        uint8
	CreatedAt     time.Time
	UpdatedAt     time.Time
	DeletedAt     *time.Time
}

func (File) TableName() string { return "files" }

type StudentVerification struct {
	ID                 uint64 `gorm:"primaryKey"`
	UUID               string `gorm:"type:char(36);uniqueIndex;not null"`
	UserID             uint64 `gorm:"uniqueIndex;not null"`
	Status             uint8
	RealNameEncrypted  string
	SchoolName         string
	StudentIDEncrypted string
	StudentIDHash      string
	Department         string
	AdmissionYear      string
	FrontFileID        *uint64
	BackFileID         *uint64
	SubmittedAt        *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

func (StudentVerification) TableName() string { return "student_verifications" }

type StudentVerificationEvent struct {
	ID             uint64 `gorm:"primaryKey"`
	UUID           string `gorm:"type:char(36);uniqueIndex;not null"`
	VerificationID uint64
	UserID         uint64
	FromStatus     uint8
	ToStatus       uint8
	EventType      string
	OperatorID     uint64
	OperatorType   uint8
	Reason         string
	TraceID        string
	CreatedAt      time.Time
}

func (StudentVerificationEvent) TableName() string { return "student_verification_events" }
