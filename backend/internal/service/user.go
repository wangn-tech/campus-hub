package service

import (
	"context"
	"errors"
	"github.com/wangn-tech/campus-hub/internal/model"
	"github.com/wangn-tech/campus-hub/internal/repository"
	"gorm.io/gorm"
	"time"
)

var ErrForbidden = errors.New("forbidden")

type UserService struct {
	users *repository.UserRepository
	tags  *repository.TagRepository
	files *repository.FileRepository
}

func NewUserService(users *repository.UserRepository, tags *repository.TagRepository, files *repository.FileRepository) *UserService {
	return &UserService{users: users, tags: tags, files: files}
}

type ProfileInput struct {
	Nickname     *string    `json:"nickname"`
	Introduction *string    `json:"introduction"`
	Gender       *uint8     `json:"gender"`
	Birthday     *time.Time `json:"birthday"`
	AvatarFileID *string    `json:"avatar_file_id"`
}

func (s *UserService) Me(ctx context.Context, id string) (*model.User, []model.Tag, error) {
	u, e := s.users.FindByUUID(ctx, id)
	if e != nil {
		return nil, nil, e
	}
	t, e := s.tags.Interests(ctx, u.ID)
	return u, t, e
}
func (s *UserService) Current(ctx context.Context, id string) (*model.User, error) {
	return s.users.FindByUUID(ctx, id)
}
func (s *UserService) Update(ctx context.Context, id string, in ProfileInput) (*model.User, error) {
	u, e := s.users.FindByUUID(ctx, id)
	if e != nil {
		return nil, e
	}
	values := map[string]any{}
	if in.Nickname != nil {
		if len(*in.Nickname) < 1 || len(*in.Nickname) > 50 {
			return nil, ErrForbidden
		}
		values["nickname"] = *in.Nickname
	}
	if in.Introduction != nil {
		if len(*in.Introduction) > 500 {
			return nil, ErrForbidden
		}
		values["introduction"] = *in.Introduction
	}
	if in.Gender != nil {
		if *in.Gender > 2 {
			return nil, ErrForbidden
		}
		values["gender"] = *in.Gender
	}
	if in.Birthday != nil {
		values["birthday"] = in.Birthday
	}
	if in.AvatarFileID != nil {
		f, e := s.files.FindByUUID(ctx, *in.AvatarFileID)
		if e != nil || f.UploaderID != u.ID {
			return nil, ErrForbidden
		}
		values["avatar_file_id"] = f.ID
		values["avatar_url"] = ""
		if u.AvatarFileID != nil && *u.AvatarFileID != f.ID {
			_ = s.files.RemoveReference(ctx, *u.AvatarFileID, "user", u.ID, "avatar")
		}
		_ = s.files.AddReference(ctx, f.ID, "user", u.ID, "avatar")
	}
	if len(values) == 0 {
		return u, nil
	}
	if e = s.users.UpdateProfile(ctx, u.ID, values); e != nil {
		return nil, e
	}
	return s.users.FindByUUID(ctx, id)
}
func (s *UserService) ReplaceInterests(ctx context.Context, id string, ids []string) error {
	u, e := s.users.FindByUUID(ctx, id)
	if e != nil {
		return e
	}
	all, e := s.tags.ListInterest(ctx)
	if e != nil {
		return e
	}
	set := map[string]bool{}
	for _, t := range all {
		set[t.UUID] = true
	}
	for _, v := range ids {
		if !set[v] {
			return gorm.ErrRecordNotFound
		}
	}
	return s.tags.ReplaceInterests(ctx, u.ID, ids)
}
