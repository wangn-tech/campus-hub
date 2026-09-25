package model

import "time"

type ChatGroup struct {
	ID         uint64 `gorm:"primaryKey"`
	UUID       string `gorm:"type:char(36);uniqueIndex;not null"`
	ActivityID uint64 `gorm:"uniqueIndex;not null"`
	Name       string `gorm:"size:100;not null"`
	Status     uint8  `gorm:"not null;default:1"`
	OwnerID    uint64 `gorm:"not null"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (ChatGroup) TableName() string { return "chat_groups" }

// Chat group status values.
const (
	ChatGroupActive    uint8 = 1
	ChatGroupReadOnly  uint8 = 2
	ChatGroupDissolved uint8 = 3
)

type ChatGroupMember struct {
	ID                uint64 `gorm:"primaryKey"`
	GroupID           uint64 `gorm:"not null"`
	UserID            uint64 `gorm:"not null"`
	Role              uint8  `gorm:"not null;default:1"`
	Status            uint8  `gorm:"not null;default:1"`
	JoinedAt          time.Time
	LeftAt            *time.Time
	LastReadMessageID *uint64
	Muted             bool `gorm:"not null;default:false"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (ChatGroupMember) TableName() string { return "chat_group_members" }

// Chat group member roles and states.
const (
	ChatMemberRoleMember uint8 = 1
	ChatMemberRoleOwner  uint8 = 2

	ChatMemberStatusActive uint8 = 1
	ChatMemberStatusLeft   uint8 = 2
)

type ChatMessage struct {
	ID              uint64 `gorm:"primaryKey"`
	UUID            string `gorm:"type:char(36);uniqueIndex;not null"`
	GroupID         uint64 `gorm:"not null"`
	SenderID        uint64 `gorm:"not null"`
	ClientMessageID string `gorm:"size:64;not null"`
	MsgType         uint8  `gorm:"not null"`
	Content         *string
	ImageFileID     *uint64
	Status          uint8 `gorm:"not null;default:1"`
	RecalledAt      *time.Time
	CreatedAt       time.Time
}

func (ChatMessage) TableName() string { return "chat_messages" }

// Message types from the WebSocket protocol document.
const (
	ChatMessageText  uint8 = 1
	ChatMessageImage uint8 = 2
)
