package repository

import (
	"context"
	"time"

	"github.com/wangn-tech/campus-hub/internal/model"
	"gorm.io/gorm"
)

type UserRepository struct{ db *gorm.DB }

func NewUserRepository(db *gorm.DB) *UserRepository { return &UserRepository{db: db} }

func (r *UserRepository) Create(ctx context.Context, user *model.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	var user model.User
	if err := r.db.WithContext(ctx).Where("email = ?", email).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) FindByUUID(ctx context.Context, uuid string) (*model.User, error) {
	var user model.User
	if err := r.db.WithContext(ctx).Where("uuid = ?", uuid).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) UpdateLastLogin(ctx context.Context, userID uint64, at time.Time) error {
	return r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", userID).Update("last_login_at", at).Error
}

func (r *UserRepository) UpdateProfile(ctx context.Context, userID uint64, values map[string]any) error {
	return r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", userID).Updates(values).Error
}

func (r *UserRepository) UpdatePassword(ctx context.Context, userID uint64, hash string) error {
	return r.UpdateProfile(ctx, userID, map[string]any{"password_hash": hash})
}

func (r *UserRepository) Logoff(ctx context.Context, userID uint64, email string) error {
	return r.UpdateProfile(ctx, userID, map[string]any{"status": 3, "email": "deleted-" + email, "password_hash": ""})
}
