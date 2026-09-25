package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/wangn-tech/campus-hub/internal/model"
	mailpkg "github.com/wangn-tech/campus-hub/internal/platform/mail"
	"github.com/wangn-tech/campus-hub/internal/repository"
	"github.com/wangn-tech/campus-hub/internal/token"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrRateLimited        = errors.New("rate limited")
	ErrInvalidCode        = errors.New("invalid email code")
	ErrEmailInUse         = errors.New("email already registered")
)

type AuthService struct {
	users    *repository.UserRepository
	tokens   *token.Manager
	redis    *redis.Client
	mail     mailpkg.Sender
	validate *validator.Validate
}

// NewAuthService wires the auth service against its explicit collaborators:
// the user repository, the token signer, the Redis client used for session
// state, and the mail sender used for verification codes.
func NewAuthService(users *repository.UserRepository, tokens *token.Manager, redisClient *redis.Client, mailSender mailpkg.Sender) *AuthService {
	return &AuthService{users: users, tokens: tokens, redis: redisClient, mail: mailSender, validate: validator.New()}
}

type RegisterInput struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8,max=72"`
	Nickname string `json:"nickname" validate:"required,min=1,max=50"`
	Code     string `json:"code" validate:"required,len=6"`
}
type LoginInput struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}
type RefreshInput struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

func normalizedEmail(email string) string { return strings.ToLower(strings.TrimSpace(email)) }
func (s *AuthService) Current(ctx context.Context, id string) (*model.User, error) {
	return s.users.FindByUUID(ctx, id)
}
func (s *AuthService) Register(ctx context.Context, in RegisterInput) (*model.User, error) {
	if err := s.validate.Struct(in); err != nil {
		return nil, err
	}
	email := normalizedEmail(in.Email)
	if err := s.VerifyEmailCode(ctx, email, "register", in.Code); err != nil {
		return nil, err
	}
	if _, err := s.users.FindByEmail(ctx, email); err == nil {
		return nil, ErrEmailInUse
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	u := &model.User{UUID: uuid.NewString(), Email: email, PasswordHash: string(hash), Nickname: strings.TrimSpace(in.Nickname), Status: 1}
	if err = s.users.Create(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}
func (s *AuthService) Login(ctx context.Context, in LoginInput) (*model.User, *token.Pair, error) {
	if err := s.validate.Struct(in); err != nil {
		return nil, nil, err
	}
	email := normalizedEmail(in.Email)
	if err := s.loginAllowed(ctx, email); err != nil {
		return nil, nil, err
	}
	u, err := s.users.FindByEmail(ctx, email)
	if errors.Is(err, gorm.ErrRecordNotFound) || err != nil || u.Status != 1 || bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(in.Password)) != nil {
		_ = s.loginFailed(ctx, email)
		return nil, nil, ErrInvalidCredentials
	}
	_ = s.redisDel(ctx, "campushub:auth:login-fail:"+email)
	pair, err := s.issueTokens(ctx, u)
	if err != nil {
		return nil, nil, err
	}
	now := time.Now().UTC()
	u.LastLoginAt = &now
	if err = s.users.UpdateLastLogin(ctx, u.ID, now); err != nil {
		return nil, nil, err
	}
	return u, pair, nil
}
func (s *AuthService) Refresh(ctx context.Context, raw string) (*token.Pair, error) {
	claims, err := s.tokens.ParseRefresh(raw)
	if err != nil || s.redis == nil {
		return nil, ErrInvalidCredentials
	}
	saved, err := s.redis.GetDel(ctx, "campushub:auth:refresh:"+claims.ID).Result()
	if err != nil || saved != tokenHash(raw) {
		_ = s.RevokeAll(ctx, claims.Subject)
		return nil, ErrInvalidCredentials
	}
	u, err := s.users.FindByUUID(ctx, claims.Subject)
	if err != nil || u.Status != 1 {
		return nil, ErrInvalidCredentials
	}
	return s.issueTokens(ctx, u)
}
func (s *AuthService) Logout(ctx context.Context, raw string) error {
	claims, err := s.tokens.ParseAccess(raw)
	if err != nil {
		return ErrInvalidCredentials
	}
	if s.redis != nil {
		ttl := time.Until(time.Unix(claims.ExpiresAt, 0))
		if ttl > 0 {
			if err = s.redis.Set(ctx, "campushub:auth:blacklist:"+claims.ID, "1", ttl).Err(); err != nil {
				return err
			}
		}
		return s.RevokeAll(ctx, claims.Subject)
	}
	return nil
}
func (s *AuthService) ChangePassword(ctx context.Context, user *model.User, oldPassword, newPassword string) error {
	if len(newPassword) < 8 || len(newPassword) > 72 || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(oldPassword)) != nil {
		return ErrInvalidCredentials
	}
	h, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	if err = s.users.UpdatePassword(ctx, user.ID, string(h)); err != nil {
		return err
	}
	return s.RevokeAll(ctx, user.UUID)
}
func (s *AuthService) ResetPassword(ctx context.Context, email, code, password string) error {
	email = normalizedEmail(email)
	if len(password) < 8 || len(password) > 72 {
		return ErrInvalidCredentials
	}
	if err := s.VerifyEmailCode(ctx, email, "forgot_password", code); err != nil {
		return err
	}
	u, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		return ErrInvalidCredentials
	}
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	if err = s.users.UpdatePassword(ctx, u.ID, string(h)); err != nil {
		return err
	}
	return s.RevokeAll(ctx, u.UUID)
}
func (s *AuthService) Logoff(ctx context.Context, user *model.User, password, code string) error {
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return ErrInvalidCredentials
	}
	if err := s.VerifyEmailCode(ctx, user.Email, "logoff", code); err != nil {
		return err
	}
	if err := s.users.Logoff(ctx, user.ID, user.Email); err != nil {
		return err
	}
	return s.RevokeAll(ctx, user.UUID)
}
func (s *AuthService) SendEmailCode(ctx context.Context, email, scene string) error {
	email = normalizedEmail(email)
	if scene != "register" && scene != "forgot_password" && scene != "logoff" {
		return ErrInvalidCode
	}
	if s.redis == nil || s.mail == nil {
		return errors.New("email delivery unavailable")
	}
	cool := "campushub:auth:email-cooldown:" + scene + ":" + email
	if ok, err := s.redis.SetNX(ctx, cool, "1", time.Minute).Result(); err != nil {
		return err
	} else if !ok {
		return ErrRateLimited
	}
	limit := "campushub:auth:email-send:" + email
	n, err := s.redis.Incr(ctx, limit).Result()
	if err != nil {
		return err
	}
	if n == 1 {
		_ = s.redis.Expire(ctx, limit, time.Hour)
	}
	if n > 5 {
		return ErrRateLimited
	}
	code, err := randomCode()
	if err != nil {
		return err
	}
	if err = s.mail.Send(ctx, email, "CampusHub verification code", fmt.Sprintf("Your verification code is %s. It expires in 10 minutes.", code)); err != nil {
		_ = s.redis.Del(ctx, cool).Err()
		return err
	}
	h, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.redis.Set(ctx, "campushub:auth:email-code:"+scene+":"+email, string(h), 10*time.Minute).Err()
}
func (s *AuthService) VerifyEmailCode(ctx context.Context, email, scene, code string) error {
	if s.redis == nil {
		return ErrInvalidCode
	}
	key := "campushub:auth:email-code:" + scene + ":" + normalizedEmail(email)
	hash, err := s.redis.Get(ctx, key).Result()
	if err != nil {
		return ErrInvalidCode
	}
	attempt := "campushub:auth:email-attempt:" + scene + ":" + normalizedEmail(email)
	if n, _ := s.redis.Incr(ctx, attempt).Result(); n == 1 {
		_ = s.redis.Expire(ctx, attempt, 10*time.Minute)
	} else if n > 5 {
		return ErrInvalidCode
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(code)) != nil {
		return ErrInvalidCode
	}
	_, err = s.redis.TxPipelined(ctx, func(p redis.Pipeliner) error { p.Del(ctx, key); p.Del(ctx, attempt); return nil })
	return err
}
func (s *AuthService) Authenticate(ctx context.Context, raw string) (string, error) {
	claims, err := s.tokens.ParseAccess(raw)
	if err != nil {
		return "", ErrInvalidCredentials
	}
	if s.redis != nil {
		if _, err = s.redis.Get(ctx, "campushub:auth:blacklist:"+claims.ID).Result(); err == nil {
			return "", ErrInvalidCredentials
		} else if !errors.Is(err, redis.Nil) {
			return "", err
		}
	}
	return claims.Subject, nil
}
func (s *AuthService) ParseAccessToken(raw string) (string, error) {
	claims, err := s.tokens.ParseAccess(raw)
	if err != nil {
		return "", ErrInvalidCredentials
	}
	return claims.Subject, nil
}
func (s *AuthService) RevokeAll(ctx context.Context, sub string) error {
	if s.redis == nil {
		return nil
	}
	key := "campushub:auth:refresh-user:" + sub
	ids, err := s.redis.SMembers(ctx, key).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return err
	}
	p := s.redis.Pipeline()
	for _, id := range ids {
		p.Del(ctx, "campushub:auth:refresh:"+id)
	}
	p.Del(ctx, key)
	_, err = p.Exec(ctx)
	return err
}
func (s *AuthService) issueTokens(ctx context.Context, u *model.User) (*token.Pair, error) {
	pair, refreshID, err := s.tokens.Issue(u.UUID)
	if err != nil {
		return nil, err
	}
	if s.redis != nil {
		p := s.redis.Pipeline()
		p.Set(ctx, "campushub:auth:refresh:"+refreshID, tokenHash(pair.RefreshToken), s.tokens.RefreshTTL())
		p.SAdd(ctx, "campushub:auth:refresh-user:"+u.UUID, refreshID)
		p.Expire(ctx, "campushub:auth:refresh-user:"+u.UUID, s.tokens.RefreshTTL())
		if _, err = p.Exec(ctx); err != nil {
			return nil, err
		}
	}
	return pair, nil
}
func (s *AuthService) loginAllowed(ctx context.Context, email string) error {
	if s.redis == nil {
		return nil
	}
	n, err := s.redis.Get(ctx, "campushub:auth:login-fail:"+email).Int()
	if err != nil && !errors.Is(err, redis.Nil) {
		return err
	}
	if n >= 5 {
		return ErrRateLimited
	}
	return nil
}
func (s *AuthService) loginFailed(ctx context.Context, email string) error {
	if s.redis == nil {
		return nil
	}
	key := "campushub:auth:login-fail:" + email
	n, err := s.redis.Incr(ctx, key).Result()
	if n == 1 {
		_ = s.redis.Expire(ctx, key, 15*time.Minute)
	}
	return err
}
func (s *AuthService) redisDel(ctx context.Context, key string) error {
	if s.redis == nil {
		return nil
	}
	return s.redis.Del(ctx, key).Err()
}
func randomCode() (string, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	n := uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
	return fmt.Sprintf("%06d", n%1000000), nil
}
func tokenHash(raw string) string { h := sha256.Sum256([]byte(raw)); return hex.EncodeToString(h[:]) }
