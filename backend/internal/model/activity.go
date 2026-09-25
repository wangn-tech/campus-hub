package model

import "time"

type Activity struct {
	ID                       uint64    `gorm:"primaryKey"`
	UUID                     string    `gorm:"type:char(36);uniqueIndex;not null"`
	Title                    string    `gorm:"size:100;not null"`
	CoverFileID              *uint64   `gorm:"index"`
	CoverURL                 string    `gorm:"size:500;not null;default:''"`
	Description              string    `gorm:"type:mediumtext"`
	CategoryID               uint64    `gorm:"index;not null"`
	OrganizerID              uint64    `gorm:"index;not null"`
	OrganizerName            string    `gorm:"size:50;not null;default:''"`
	OrganizerAvatar          string    `gorm:"size:500;not null;default:''"`
	ContactPhone             string    `gorm:"size:20;not null;default:''"`
	RegisterStartAt          time.Time `gorm:"not null"`
	RegisterEndAt            time.Time `gorm:"not null"`
	ActivityStartAt          time.Time `gorm:"not null"`
	ActivityEndAt            time.Time `gorm:"not null"`
	Location                 string    `gorm:"size:200;not null;default:''"`
	AddressDetail            string    `gorm:"size:500;not null;default:''"`
	Longitude                *float64  `gorm:"type:decimal(10,7)"`
	Latitude                 *float64  `gorm:"type:decimal(10,7)"`
	MaxParticipants          uint32    `gorm:"not null;default:0"`
	ApprovedParticipantCount uint32    `gorm:"not null;default:0"`
	PendingParticipantCount  uint32    `gorm:"not null;default:0"`
	RequireApproval          bool      `gorm:"not null;default:false"`
	RequireStudentVerify     bool      `gorm:"not null;default:false"`
	MinCreditScore           int       `gorm:"not null;default:0"`
	Status                   uint8     `gorm:"not null;default:0"`
	RejectReason             string    `gorm:"size:500;not null;default:''"`
	ViewCount                uint64    `gorm:"not null;default:0"`
	Version                  uint32    `gorm:"not null;default:0"`
	PublishedAt              *time.Time
	CreatedAt                time.Time
	UpdatedAt                time.Time
	DeletedAt                *time.Time `gorm:"index"`
}

func (Activity) TableName() string { return "activities" }

type ActivityStatusLog struct {
	ID           uint64 `gorm:"primaryKey"`
	UUID         string `gorm:"type:char(36);uniqueIndex;not null"`
	ActivityID   uint64 `gorm:"index;not null"`
	FromStatus   uint8  `gorm:"not null"`
	ToStatus     uint8  `gorm:"not null"`
	OperatorID   uint64 `gorm:"not null;default:0"`
	OperatorType uint8  `gorm:"not null;default:1"`
	Reason       string `gorm:"size:500;not null;default:''"`
	TraceID      string `gorm:"size:64;not null;default:''"`
	CreatedAt    time.Time
}

func (ActivityStatusLog) TableName() string { return "activity_status_logs" }

type ActivityTagRelation struct {
	ID         uint64 `gorm:"primaryKey"`
	ActivityID uint64 `gorm:"not null"`
	TagID      uint64 `gorm:"not null"`
	CreatedAt  time.Time
}

func (ActivityTagRelation) TableName() string { return "activity_tag_relations" }

type Category struct {
	ID        uint64    `gorm:"primaryKey" json:"-"`
	UUID      string    `gorm:"type:char(36);uniqueIndex;not null" json:"id"`
	Name      string    `gorm:"size:50;not null" json:"name"`
	Icon      string    `gorm:"size:100;not null;default:''" json:"icon"`
	Sort      int       `gorm:"not null;default:0" json:"sort"`
	Status    uint8     `gorm:"not null;default:1" json:"status"`
	CreatedAt time.Time `json:"-"`
	UpdatedAt time.Time `json:"-"`
}

func (Category) TableName() string { return "categories" }

// Operator types recorded in activity_status_logs.
const (
	ActivityOperatorUser   uint8 = 1
	ActivityOperatorAdmin  uint8 = 2
	ActivityOperatorSystem uint8 = 3
)
