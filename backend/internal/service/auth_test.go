package service

import (
	"context"
	"testing"
	"time"

	"github.com/wangn-tech/campus-hub/internal/config"
	"github.com/wangn-tech/campus-hub/internal/model"
	"github.com/wangn-tech/campus-hub/internal/token"
)

const testJWTIssuer = "campushub-test"

func newTestAuthService(jwtConfig config.JWTConfig) *AuthService {
	return NewAuthService(nil, token.NewManager(jwtConfig), nil, nil)
}

func TestAccessTokenRoundTrip(t *testing.T) {
	auth := newTestAuthService(config.JWTConfig{Issuer: testJWTIssuer, AccessSecret: "access-secret", AccessTTL: time.Hour})
	pair, err := auth.issueTokens(context.Background(), &model.User{UUID: "user-uuid"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := auth.ParseAccessToken(pair.AccessToken)
	if err != nil || got != "user-uuid" {
		t.Fatalf("parse access token: %q, %v", got, err)
	}
}

func TestAccessTokenRejectsRefreshToken(t *testing.T) {
	auth := newTestAuthService(config.JWTConfig{Issuer: testJWTIssuer, AccessSecret: "access-secret", RefreshSecret: "refresh-secret", AccessTTL: time.Hour, RefreshTTL: time.Hour})
	pair, err := auth.issueTokens(context.Background(), &model.User{UUID: "user-uuid"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := auth.ParseAccessToken(pair.RefreshToken); err == nil {
		t.Fatal("refresh token must not be accepted as access token")
	}
}
