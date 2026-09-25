package repository

import (
	"context"
	"github.com/wangn-tech/campus-hub/internal/model"
	"gorm.io/gorm"
)

type TagRepository struct{ db *gorm.DB }

func NewTagRepository(db *gorm.DB) *TagRepository { return &TagRepository{db: db} }
func (r *TagRepository) ListInterest(ctx context.Context) ([]model.Tag, error) {
	var tags []model.Tag
	err := r.db.WithContext(ctx).Table("tags").Select("tags.*").Joins("JOIN tag_scopes ON tag_scopes.tag_id = tags.id").Where("tag_scopes.scope = ? AND tag_scopes.status = ? AND tags.status = ?", "interest", 1, 1).Order("tags.name").Find(&tags).Error
	return tags, err
}
func (r *TagRepository) ReplaceInterests(ctx context.Context, userID uint64, uuids []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("DELETE FROM user_interest_relations WHERE user_id = ?", userID).Error; err != nil {
			return err
		}
		if len(uuids) == 0 {
			return nil
		}
		return tx.Exec("INSERT INTO user_interest_relations (user_id, tag_id, created_at) SELECT ?, id, UTC_TIMESTAMP(3) FROM tags WHERE uuid IN ? AND status = 1 AND EXISTS (SELECT 1 FROM tag_scopes WHERE tag_scopes.tag_id = tags.id AND scope = 'interest' AND status = 1)", userID, uuids).Error
	})
}
func (r *TagRepository) Interests(ctx context.Context, userID uint64) ([]model.Tag, error) {
	var tags []model.Tag
	err := r.db.WithContext(ctx).Table("tags").Select("tags.*").Joins("JOIN user_interest_relations r ON r.tag_id=tags.id").Where("r.user_id=?", userID).Find(&tags).Error
	return tags, err
}
