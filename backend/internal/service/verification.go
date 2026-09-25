package service

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"github.com/google/uuid"
	"github.com/wangn-tech/campus-hub/internal/config"
	"github.com/wangn-tech/campus-hub/internal/model"
	"github.com/wangn-tech/campus-hub/internal/repository"
	"gorm.io/gorm"
	"io"
	"time"
)

type VerificationService struct {
	repo    *repository.VerificationRepository
	files   *repository.FileRepository
	key     []byte
	hashKey []byte
}

func NewVerificationService(repo *repository.VerificationRepository, files *repository.FileRepository, cfg config.SecurityConfig) (*VerificationService, error) {
	k, e := base64.StdEncoding.DecodeString(cfg.PIIEncryptionKey)
	if e != nil || len(k) != 32 {
		return nil, errors.New("invalid pii encryption key")
	}
	return &VerificationService{repo: repo, files: files, key: k, hashKey: []byte(cfg.PIIHashKey)}, nil
}

type VerificationInput struct {
	RealName      string `json:"real_name"`
	SchoolName    string `json:"school_name"`
	StudentID     string `json:"student_id"`
	Department    string `json:"department"`
	AdmissionYear string `json:"admission_year"`
	FrontFileID   string `json:"front_file_id"`
	BackFileID    string `json:"back_file_id"`
}

func (s *VerificationService) Current(ctx context.Context, u *model.User) (*model.StudentVerification, error) {
	return s.repo.Current(ctx, u.ID)
}
func (s *VerificationService) Submit(ctx context.Context, u *model.User, in VerificationInput, trace string) (*model.StudentVerification, error) {
	if in.RealName == "" || in.SchoolName == "" || in.StudentID == "" || in.FrontFileID == "" || in.BackFileID == "" {
		return nil, ErrForbidden
	}
	if _, e := s.repo.Current(ctx, u.ID); e == nil {
		return nil, ErrForbidden
	} else if !errors.Is(e, gorm.ErrRecordNotFound) {
		return nil, e
	}
	front, e := s.files.FindByUUID(ctx, in.FrontFileID)
	if e != nil || front.UploaderID != u.ID || front.BizType != "student_verification_front" {
		return nil, ErrForbidden
	}
	back, e := s.files.FindByUUID(ctx, in.BackFileID)
	if e != nil || back.UploaderID != u.ID || back.BizType != "student_verification_back" {
		return nil, ErrForbidden
	}
	real, e := s.encrypt(in.RealName)
	if e != nil {
		return nil, e
	}
	sid, e := s.encrypt(in.StudentID)
	if e != nil {
		return nil, e
	}
	h := hmac.New(sha256.New, s.hashKey)
	h.Write([]byte(in.StudentID))
	now := time.Now().UTC()
	v := &model.StudentVerification{UUID: uuid.NewString(), UserID: u.ID, Status: uint8(model.VerificationPendingConfirm), RealNameEncrypted: real, SchoolName: in.SchoolName, StudentIDEncrypted: sid, StudentIDHash: hex.EncodeToString(h.Sum(nil)), Department: in.Department, AdmissionYear: in.AdmissionYear, FrontFileID: &front.ID, BackFileID: &back.ID, SubmittedAt: &now}
	event := &model.StudentVerificationEvent{UUID: uuid.NewString(), UserID: u.ID, FromStatus: uint8(model.VerificationInitialized), ToStatus: uint8(model.VerificationPendingConfirm), EventType: "submitted", OperatorID: u.ID, OperatorType: 1, TraceID: trace}
	outbox, err := verificationOutboxEvent("verification.submitted", v, u.UUID, uint8(model.VerificationPendingConfirm), "OCR 识别完成，请确认", trace)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, v, event, outbox); err != nil {
		return nil, err
	}
	_ = s.files.AddReference(ctx, front.ID, "student_verification", v.ID, "front")
	_ = s.files.AddReference(ctx, back.ID, "student_verification", v.ID, "back")
	return v, nil
}
func (s *VerificationService) Confirm(ctx context.Context, u *model.User, id, trace string) error {
	v, e := s.repo.Current(ctx, u.ID)
	if e != nil || v.UUID != id || v.Status != uint8(model.VerificationPendingConfirm) {
		return ErrForbidden
	}
	outbox, err := verificationOutboxEvent("verification.confirmed", v, u.UUID, uint8(model.VerificationManualReview), "认证资料已提交人工审核", trace)
	if err != nil {
		return err
	}
	return s.repo.Transition(ctx, v, uint8(model.VerificationManualReview), "confirmed", "", trace, outbox)
}
func (s *VerificationService) Cancel(ctx context.Context, u *model.User, id, reason, trace string) error {
	v, e := s.repo.Current(ctx, u.ID)
	if e != nil || v.UUID != id || (v.Status != uint8(model.VerificationPendingConfirm) && v.Status != uint8(model.VerificationManualReview)) {
		return ErrForbidden
	}
	outbox, err := verificationOutboxEvent("verification.cancelled", v, u.UUID, uint8(model.VerificationCancelled), "认证申请已取消", trace)
	if err != nil {
		return err
	}
	return s.repo.Transition(ctx, v, uint8(model.VerificationCancelled), "cancelled", reason, trace, outbox)
}

func verificationOutboxEvent(eventType string, verification *model.StudentVerification, userUUID string, status uint8, message, trace string) (*model.OutboxEvent, error) {
	return newOutboxEvent(eventType, "student_verification", verification.UUID, trace, map[string]any{
		"verification_id": verification.UUID,
		"user_id":         userUUID,
		"status":          status,
		"message":         message,
	})
}
func (s *VerificationService) encrypt(value string) (string, error) {
	b, e := aes.NewCipher(s.key)
	if e != nil {
		return "", e
	}
	g, e := cipher.NewGCM(b)
	if e != nil {
		return "", e
	}
	nonce := make([]byte, g.NonceSize())
	if _, e = io.ReadFull(rand.Reader, nonce); e != nil {
		return "", e
	}
	return base64.StdEncoding.EncodeToString(g.Seal(nonce, nonce, []byte(value), nil)), nil
}
