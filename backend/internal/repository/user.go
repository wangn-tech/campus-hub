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

func (r *UserRepository) UpdateLastLogin(ctx context.Context, userID uint64, at time.Time) error {
	return r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", userID).Update("last_login_at", at).Error
}
