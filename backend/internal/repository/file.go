package repository

import (
	"context"
	"github.com/wangn-tech/campus-hub/internal/model"
	"gorm.io/gorm"
)

type FileRepository struct{ db *gorm.DB }

func NewFileRepository(db *gorm.DB) *FileRepository { return &FileRepository{db: db} }
func (r *FileRepository) Create(ctx context.Context, file *model.File) error {
	return r.db.WithContext(ctx).Create(file).Error
}
func (r *FileRepository) FindByUUID(ctx context.Context, id string) (*model.File, error) {
	var f model.File
	if err := r.db.WithContext(ctx).Where("uuid = ?", id).First(&f).Error; err != nil {
		return nil, err
	}
	return &f, nil
}
func (r *FileRepository) FindByID(ctx context.Context, id uint64) (*model.File, error) {
	var f model.File
	if err := r.db.WithContext(ctx).First(&f, id).Error; err != nil {
		return nil, err
	}
	return &f, nil
}
func (r *FileRepository) FindByIDs(ctx context.Context, ids []uint64) ([]model.File, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var files []model.File
	err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&files).Error
	return files, err
}
func (r *FileRepository) SoftDelete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&model.File{}, id).Error
}
func (r *FileRepository) IsReferenced(ctx context.Context, id uint64) (bool, error) {
	var n int64
	err := r.db.WithContext(ctx).Table("file_references").Where("file_id = ?", id).Count(&n).Error
	return n > 0, err
}
func (r *FileRepository) AddReference(ctx context.Context, fileID uint64, resourceType string, resourceID uint64, purpose string) error {
	return r.db.WithContext(ctx).Exec("INSERT IGNORE INTO file_references (file_id, resource_type, resource_id, purpose, created_at) VALUES (?, ?, ?, ?, UTC_TIMESTAMP(3))", fileID, resourceType, resourceID, purpose).Error
}
func (r *FileRepository) RemoveReference(ctx context.Context, fileID uint64, resourceType string, resourceID uint64, purpose string) error {
	return r.db.WithContext(ctx).Exec("DELETE FROM file_references WHERE file_id = ? AND resource_type = ? AND resource_id = ? AND purpose = ?", fileID, resourceType, resourceID, purpose).Error
}
