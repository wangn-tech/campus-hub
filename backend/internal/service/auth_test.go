package service

import (
	"testing"
	"time"

	"github.com/wangn-tech/campus-hub/internal/config"
	"github.com/wangn-tech/campus-hub/internal/model"
)

func TestAccessTokenRoundTrip(t *testing.T) {
	service := NewAuthService(nil, config.JWTConfig{Issuer: "campushub-test", AccessSecret: "access-secret", AccessTTL: time.Hour})
	tokens, err := service.issueTokens(&model.User{UUID: "user-uuid"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := service.ParseAccessToken(tokens.AccessToken)
	if err != nil || got != "user-uuid" {
		t.Fatalf("parse access token: %q, %v", got, err)
	}
}

func TestAccessTokenRejectsRefreshToken(t *testing.T) {
	service := NewAuthService(nil, config.JWTConfig{Issuer: "campushub-test", AccessSecret: "access-secret", RefreshSecret: "refresh-secret", AccessTTL: time.Hour, RefreshTTL: time.Hour})
	tokens, err := service.issueTokens(&model.User{UUID: "user-uuid"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ParseAccessToken(tokens.RefreshToken); err == nil {
		t.Fatal("refresh token must not be accepted as access token")
	}
}
