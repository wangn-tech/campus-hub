package repository

import (
	"context"
	"time"

	"github.com/wangn-tech/campus-hub/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ChatRepository struct{ db *gorm.DB }

func NewChatRepository(db *gorm.DB) *ChatRepository { return &ChatRepository{db: db} }

// CreateGroupWithOwner creates the activity group and its owner membership in
// one transaction. Extra rows are written by the caller inside the same
// transaction through the variants below.
func (r *ChatRepository) CreateGroupWithOwner(ctx context.Context, group *model.ChatGroup, owner *model.ChatGroupMember) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return createGroupWithOwner(tx, group, owner)
	})
}

func createGroupWithOwner(tx *gorm.DB, group *model.ChatGroup, owner *model.ChatGroupMember) error {
	if err := tx.Create(group).Error; err != nil {
		return err
	}
	owner.GroupID = group.ID
	return tx.Create(owner).Error
}

func (r *ChatRepository) FindGroupByUUID(ctx context.Context, uuid string) (*model.ChatGroup, error) {
	var group model.ChatGroup
	if err := r.db.WithContext(ctx).Where("uuid = ?", uuid).First(&group).Error; err != nil {
		return nil, err
	}
	return &group, nil
}

func (r *ChatRepository) FindGroupByActivityID(ctx context.Context, activityID uint64) (*model.ChatGroup, error) {
	var group model.ChatGroup
	if err := r.db.WithContext(ctx).Where("activity_id = ?", activityID).First(&group).Error; err != nil {
		return nil, err
	}
	return &group, nil
}

func (r *ChatRepository) ListGroupsByUser(ctx context.Context, userID uint64) ([]model.ChatGroup, error) {
	var groups []model.ChatGroup
	err := r.db.WithContext(ctx).Table("chat_groups").
		Select("chat_groups.*").
		Joins("JOIN chat_group_members ON chat_group_members.group_id = chat_groups.id").
		Where("chat_group_members.user_id = ? AND chat_group_members.status = ?", userID, model.ChatMemberStatusActive).
		Order("chat_groups.id DESC").
		Find(&groups).Error
	return groups, err
}

// CountMembers counts the active members per group.
func (r *ChatRepository) CountMembers(ctx context.Context, groupIDs []uint64) (map[uint64]int64, error) {
	counts := make(map[uint64]int64, len(groupIDs))
	if len(groupIDs) == 0 {
		return counts, nil
	}
	var rows []struct {
		GroupID uint64
		Total   int64
	}
	err := r.db.WithContext(ctx).Model(&model.ChatGroupMember{}).
		Select("group_id, COUNT(*) AS total").
		Where("group_id IN ? AND status = ?", groupIDs, model.ChatMemberStatusActive).
		Group("group_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		counts[row.GroupID] = row.Total
	}
	return counts, nil
}

func (r *ChatRepository) ListMembers(ctx context.Context, groupID uint64) ([]model.ChatGroupMember, error) {
	var members []model.ChatGroupMember
	err := r.db.WithContext(ctx).
		Where("group_id = ? AND status = ?", groupID, model.ChatMemberStatusActive).
		Order("role DESC, joined_at").
		Find(&members).Error
	return members, err
}

func (r *ChatRepository) FindMember(ctx context.Context, groupID, userID uint64) (*model.ChatGroupMember, error) {
	var member model.ChatGroupMember
	if err := r.db.WithContext(ctx).Where("group_id = ? AND user_id = ?", groupID, userID).First(&member).Error; err != nil {
		return nil, err
	}
	return &member, nil
}

// JoinGroup adds the user to the group, reactivating a membership row that was
// left earlier.
func (r *ChatRepository) JoinGroup(ctx context.Context, groupID, userID uint64, role uint8) error {
	now := time.Now().UTC()
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "group_id"}, {Name: "user_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"status": model.ChatMemberStatusActive, "left_at": nil, "joined_at": now, "updated_at": now,
		}),
	}).Create(&model.ChatGroupMember{
		GroupID: groupID, UserID: userID, Role: role, Status: model.ChatMemberStatusActive,
		JoinedAt: now, CreatedAt: now, UpdatedAt: now,
	}).Error
}

// LeaveGroup marks the membership as left. Leaving twice is a no-op.
func (r *ChatRepository) LeaveGroup(ctx context.Context, groupID, userID uint64) error {
	now := time.Now().UTC()
	return r.db.WithContext(ctx).Model(&model.ChatGroupMember{}).
		Where("group_id = ? AND user_id = ? AND status = ?", groupID, userID, model.ChatMemberStatusActive).
		Updates(map[string]any{"status": model.ChatMemberStatusLeft, "left_at": now, "updated_at": now}).Error
}

func (r *ChatRepository) SetGroupStatus(ctx context.Context, groupID uint64, status uint8) error {
	return r.db.WithContext(ctx).Model(&model.ChatGroup{}).Where("id = ?", groupID).
		Updates(map[string]any{"status": status, "updated_at": time.Now().UTC()}).Error
}

type MessageFilter struct {
	GroupID uint64
	AfterID uint64
	Limit   int
}

// ListMessages returns the newest messages of a group, newest first.
func (r *ChatRepository) ListMessages(ctx context.Context, filter MessageFilter) ([]model.ChatMessage, error) {
	query := r.db.WithContext(ctx).Model(&model.ChatMessage{}).Where("group_id = ?", filter.GroupID)
	if filter.AfterID > 0 {
		query = query.Where("id > ?", filter.AfterID)
	}
	var messages []model.ChatMessage
	err := query.Select("chat_messages.*").
		Order("id DESC").
		Limit(filter.Limit).
		Find(&messages).Error
	return messages, err
}

// ListMessagesForGroups returns the newest messages across several groups,
// used to serve offline messages.
func (r *ChatRepository) ListMessagesForGroups(ctx context.Context, groupIDs []uint64, afterID uint64, limit int) ([]model.ChatMessage, error) {
	if len(groupIDs) == 0 {
		return nil, nil
	}
	query := r.db.WithContext(ctx).Model(&model.ChatMessage{}).Where("group_id IN ?", groupIDs)
	if afterID > 0 {
		query = query.Where("id > ?", afterID)
	}
	var messages []model.ChatMessage
	err := query.Select("chat_messages.*").
		Order("id DESC").
		Limit(limit).
		Find(&messages).Error
	return messages, err
}

func (r *ChatRepository) FindMessagesByIDs(ctx context.Context, ids []uint64) ([]model.ChatMessage, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var messages []model.ChatMessage
	err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&messages).Error
	return messages, err
}

func (r *ChatRepository) FindGroupsByIDs(ctx context.Context, ids []uint64) ([]model.ChatGroup, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var groups []model.ChatGroup
	err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&groups).Error
	return groups, err
}

// CreateMessage persists one chat message. The unique key on
// (sender_id, client_message_id) makes a retry of the same client message fail
// instead of duplicating it.
func (r *ChatRepository) CreateMessage(ctx context.Context, message *model.ChatMessage) error {
	return r.db.WithContext(ctx).Create(message).Error
}

// FindMessageByClientID looks up a message by the sender's client side ID so
// resends can be answered idempotently.
func (r *ChatRepository) FindMessageByClientID(ctx context.Context, senderID uint64, clientMessageID string) (*model.ChatMessage, error) {
	var message model.ChatMessage
	if err := r.db.WithContext(ctx).
		Where("sender_id = ? AND client_message_id = ?", senderID, clientMessageID).
		First(&message).Error; err != nil {
		return nil, err
	}
	return &message, nil
}

// FindMessageByUUID resolves a read receipt target.
func (r *ChatRepository) FindMessageByUUID(ctx context.Context, messageUUID string) (*model.ChatMessage, error) {
	var message model.ChatMessage
	if err := r.db.WithContext(ctx).Where("uuid = ?", messageUUID).First(&message).Error; err != nil {
		return nil, err
	}
	return &message, nil
}

// UpdateLastRead moves the member's read pointer forward; it never rewinds it
// when an older message is acknowledged late.
func (r *ChatRepository) UpdateLastRead(ctx context.Context, groupID, userID, messageID uint64) error {
	now := time.Now().UTC()
	return r.db.WithContext(ctx).Model(&model.ChatGroupMember{}).
		Where("group_id = ? AND user_id = ? AND (last_read_message_id IS NULL OR last_read_message_id < ?)", groupID, userID, messageID).
		Updates(map[string]any{"last_read_message_id": messageID, "updated_at": now}).Error
}
