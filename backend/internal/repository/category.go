package repository

import (
	"context"

	"github.com/wangn-tech/campus-hub/internal/model"
	"gorm.io/gorm"
)

type CategoryRepository struct{ db *gorm.DB }

func NewCategoryRepository(db *gorm.DB) *CategoryRepository { return &CategoryRepository{db: db} }

func (r *CategoryRepository) ListActive(ctx context.Context) ([]model.Category, error) {
	var items []model.Category
	err := r.db.WithContext(ctx).Where("status = ?", 1).Order("sort, id").Find(&items).Error
	return items, err
}

func (r *CategoryRepository) FindByUUID(ctx context.Context, uuid string) (*model.Category, error) {
	var item model.Category
	if err := r.db.WithContext(ctx).Where("uuid = ?", uuid).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *CategoryRepository) FindByIDs(ctx context.Context, ids []uint64) ([]model.Category, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var items []model.Category
	err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&items).Error
	return items, err
}
