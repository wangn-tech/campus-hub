package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/wangn-tech/campus-hub/internal/config"
	"github.com/wangn-tech/campus-hub/internal/model"
	"github.com/wangn-tech/campus-hub/internal/repository"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var ErrInvalidCredentials = errors.New("invalid credentials")

type AuthService struct {
	users     *repository.UserRepository
	jwtConfig config.JWTConfig
	validate  *validator.Validate
}

func NewAuthService(users *repository.UserRepository, jwtConfig config.JWTConfig) *AuthService {
	return &AuthService{users: users, jwtConfig: jwtConfig, validate: validator.New()}
}

type RegisterInput struct {
	Email    string `validate:"required,email"`
	Password string `validate:"required,min=8,max=72"`
	Nickname string `validate:"required,min=1,max=50"`
}

type LoginInput struct {
	Email    string `validate:"required,email"`
	Password string `validate:"required"`
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

func (s *AuthService) Register(ctx context.Context, input RegisterInput) (*model.User, error) {
	if err := s.validate.Struct(input); err != nil {
		return nil, err
	}
	email := strings.ToLower(strings.TrimSpace(input.Email))
	if _, err := s.users.FindByEmail(ctx, email); err == nil {
		return nil, gorm.ErrDuplicatedKey
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	user := &model.User{UUID: uuid.NewString(), Email: email, PasswordHash: string(hash), Nickname: input.Nickname, Status: 1}
	if err := s.users.Create(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *AuthService) Login(ctx context.Context, input LoginInput) (*model.User, *TokenPair, error) {
	if err := s.validate.Struct(input); err != nil {
		return nil, nil, err
	}
	user, err := s.users.FindByEmail(ctx, strings.ToLower(strings.TrimSpace(input.Email)))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil, ErrInvalidCredentials
	}
	if err != nil || user.Status != 1 || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)) != nil {
		return nil, nil, ErrInvalidCredentials
	}
	pair, err := s.issueTokens(user)
	if err != nil {
		return nil, nil, err
	}
	now := time.Now().UTC()
	user.LastLoginAt = &now
	if err := s.users.UpdateLastLogin(ctx, user.ID, now); err != nil {
		return nil, nil, err
	}
	return user, pair, nil
}

func (s *AuthService) ParseAccessToken(tokenString string) (string, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(s.jwtConfig.AccessSecret), nil
	}, jwt.WithIssuer(s.jwtConfig.Issuer))
	if err != nil || !token.Valid {
		return "", ErrInvalidCredentials
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", ErrInvalidCredentials
	}
	subject, ok := claims["sub"].(string)
	if !ok || subject == "" {
		return "", ErrInvalidCredentials
	}
	if tokenType, exists := claims["typ"].(string); exists && tokenType != "" {
		return "", ErrInvalidCredentials
	}
	return subject, nil
}

func (s *AuthService) issueTokens(user *model.User) (*TokenPair, error) {
	now := time.Now().UTC()
	access, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": user.UUID, "jti": uuid.NewString(), "iss": s.jwtConfig.Issuer, "iat": now.Unix(), "exp": now.Add(s.jwtConfig.AccessTTL).Unix()}).SignedString([]byte(s.jwtConfig.AccessSecret))
	if err != nil {
		return nil, err
	}
	refresh, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": user.UUID, "jti": uuid.NewString(), "iss": s.jwtConfig.Issuer, "typ": "refresh", "iat": now.Unix(), "exp": now.Add(s.jwtConfig.RefreshTTL).Unix()}).SignedString([]byte(s.jwtConfig.RefreshSecret))
	if err != nil {
		return nil, err
	}
	return &TokenPair{AccessToken: access, RefreshToken: refresh, ExpiresIn: int64(s.jwtConfig.AccessTTL.Seconds())}, nil
}
