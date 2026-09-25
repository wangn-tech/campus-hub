package repository

import (
	"context"
	"errors"
	"time"

	"github.com/wangn-tech/campus-hub/internal/model"
	"gorm.io/gorm"
)

// defaultCreditScore is used when a user has no credit profile row yet.
const defaultCreditScore = 100

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

func (r *UserRepository) FindByIDs(ctx context.Context, ids []uint64) ([]model.User, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var users []model.User
	err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&users).Error
	return users, err
}

// HasRole reports whether the user holds the given role code.
func (r *UserRepository) HasRole(ctx context.Context, userID uint64, code string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Table("user_roles").
		Joins("JOIN roles ON roles.id = user_roles.role_id").
		Where("user_roles.user_id = ? AND roles.code = ?", userID, code).
		Count(&count).Error
	return count > 0, err
}

// CreditScore returns the user's credit score, falling back to the default when
// the profile row does not exist yet.
func (r *UserRepository) CreditScore(ctx context.Context, userID uint64) (int, error) {
	var profile struct{ Score int }
	err := r.db.WithContext(ctx).Table("user_credit_profiles").
		Select("score").
		Where("user_id = ?", userID).
		Take(&profile).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return defaultCreditScore, nil
	}
	if err != nil {
		return 0, err
	}
	return profile.Score, nil
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
