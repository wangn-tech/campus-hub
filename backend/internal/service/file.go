package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/wangn-tech/campus-hub/internal/config"
	"github.com/wangn-tech/campus-hub/internal/model"
	storagepkg "github.com/wangn-tech/campus-hub/internal/platform/storage"
	"github.com/wangn-tech/campus-hub/internal/repository"
	"gorm.io/gorm"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

var ErrStorageUnavailable = errors.New("storage unavailable")

type FileService struct {
	repo    *repository.FileRepository
	storage *storagepkg.Client
	max     int64
	bucket  string
}

func NewFileService(repo *repository.FileRepository, st *storagepkg.Client, cfg config.StorageConfig) *FileService {
	return &FileService{repo: repo, storage: st, max: cfg.MaxImageSize, bucket: cfg.Bucket}
}
func (s *FileService) UploadImage(ctx context.Context, user *model.User, name, biz string, r io.Reader) (*model.File, string, error) {
	if biz != "avatar" && biz != "student_verification_front" && biz != "student_verification_back" && biz != "activity_cover" {
		return nil, "", ErrForbidden
	}
	b, e := io.ReadAll(io.LimitReader(r, s.max+1))
	if e != nil {
		return nil, "", e
	}
	if int64(len(b)) > s.max || len(b) == 0 {
		return nil, "", ErrForbidden
	}
	mime := http.DetectContentType(b)
	ext := map[string]string{"image/jpeg": "jpg", "image/png": "png", "image/webp": "webp", "image/gif": "gif"}[mime]
	if ext == "" || !validExtension(name, ext) {
		return nil, "", ErrForbidden
	}
	id := uuid.NewString()
	key := fmt.Sprintf("images/%s/%s/%s.%s", biz, time.Now().UTC().Format("2006/01"), id, ext)
	if e = s.storage.Put(ctx, key, bytes.NewReader(b), int64(len(b)), mime); e != nil {
		return nil, "", fmt.Errorf("%w: %v", ErrStorageUnavailable, e)
	}
	hash := sha256.Sum256(b)
	f := &model.File{UUID: id, StorageDriver: "rustfs", Bucket: s.bucket, ObjectKey: key, OriginName: filepath.Base(name), BizType: biz, FileSize: uint64(len(b)), MIMEType: mime, Extension: ext, SHA256: hex.EncodeToString(hash[:]), UploaderID: user.ID, Status: 1}
	if e = s.repo.Create(ctx, f); e != nil {
		_ = s.storage.Remove(ctx, key)
		return nil, "", e
	}
	url, e := s.storage.SignedURL(ctx, key)
	if e != nil {
		return nil, "", fmt.Errorf("%w: %v", ErrStorageUnavailable, e)
	}
	return f, url, nil
}
func (s *FileService) Get(ctx context.Context, user *model.User, id string) (*model.File, string, error) {
	f, e := s.repo.FindByUUID(ctx, id)
	if e != nil {
		return nil, "", e
	}
	if f.UploaderID != user.ID {
		return nil, "", ErrForbidden
	}
	url, e := s.storage.SignedURL(ctx, f.ObjectKey)
	if e != nil {
		return nil, "", fmt.Errorf("%w: %v", ErrStorageUnavailable, e)
	}
	return f, url, nil
}
func (s *FileService) Avatar(ctx context.Context, user *model.User) (*model.File, string, error) {
	if user.AvatarFileID == nil {
		return nil, "", nil
	}
	f, err := s.repo.FindByID(ctx, *user.AvatarFileID)
	if err != nil {
		return nil, "", err
	}
	url, err := s.AccessURL(ctx, f)
	if err != nil {
		return nil, "", err
	}
	return f, url, nil
}

// AccessURL returns a fresh, short lived URL for an already stored file.
func (s *FileService) AccessURL(ctx context.Context, f *model.File) (string, error) {
	url, err := s.storage.SignedURL(ctx, f.ObjectKey)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrStorageUnavailable, err)
	}
	return url, nil
}
func (s *FileService) Delete(ctx context.Context, user *model.User, id string) error {
	f, e := s.repo.FindByUUID(ctx, id)
	if e != nil {
		return e
	}
	if f.UploaderID != user.ID {
		return ErrForbidden
	}
	ref, e := s.repo.IsReferenced(ctx, f.ID)
	if e != nil {
		return e
	}
	if ref {
		return ErrForbidden
	}
	if e = s.storage.Remove(ctx, f.ObjectKey); e != nil {
		return fmt.Errorf("%w: %v", ErrStorageUnavailable, e)
	}
	return s.repo.SoftDelete(ctx, f.ID)
}
func validExtension(name, expected string) bool {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), ".")
	return ext == expected || (expected == "jpg" && ext == "jpeg")
}

var _ = gorm.ErrRecordNotFound
