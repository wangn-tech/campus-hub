package token

import (
	"testing"
	"time"

	"github.com/wangn-tech/campus-hub/internal/config"
)

func testManager() *Manager {
	return NewManager(config.JWTConfig{
		Issuer:        "campushub-test",
		AccessSecret:  "access-secret",
		RefreshSecret: "refresh-secret",
		AccessTTL:     time.Hour,
		RefreshTTL:    24 * time.Hour,
	})
}

func TestIssueAndParseAccess(t *testing.T) {
	pair, refreshID, err := testManager().Issue("user-uuid")
	if err != nil {
		t.Fatal(err)
	}
	if refreshID == "" {
		t.Fatal("expected a non-empty refresh id")
	}
	claims, err := testManager().ParseAccess(pair.AccessToken)
	if err != nil {
		t.Fatalf("parse access token: %v", err)
	}
	if claims.Subject != "user-uuid" || claims.ID == "" || claims.ExpiresAt == 0 {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}

func TestRefreshRoundTrip(t *testing.T) {
	pair, refreshID, err := testManager().Issue("user-uuid")
	if err != nil {
		t.Fatal(err)
	}
	claims, err := testManager().ParseRefresh(pair.RefreshToken)
	if err != nil {
		t.Fatalf("parse refresh token: %v", err)
	}
	if claims.Subject != "user-uuid" || claims.ID != refreshID {
		t.Fatalf("unexpected refresh claims: %+v", claims)
	}
}

func TestParseAccessRejectsRefreshToken(t *testing.T) {
	pair, _, err := testManager().Issue("user-uuid")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := testManager().ParseAccess(pair.RefreshToken); err == nil {
		t.Fatal("refresh token must not parse as an access token")
	}
}

func TestParseRefreshRejectsAccessToken(t *testing.T) {
	pair, _, err := testManager().Issue("user-uuid")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := testManager().ParseRefresh(pair.AccessToken); err == nil {
		t.Fatal("access token must not parse as a refresh token")
	}
}

func TestParseRejectsWrongIssuer(t *testing.T) {
	pair, _, err := testManager().Issue("user-uuid")
	if err != nil {
		t.Fatal(err)
	}
	other := NewManager(config.JWTConfig{Issuer: "other-issuer", AccessSecret: "access-secret", AccessTTL: time.Hour})
	if _, err := other.ParseAccess(pair.AccessToken); err == nil {
		t.Fatal("token from another issuer must be rejected")
	}
}

func TestParseRejectsWrongSecret(t *testing.T) {
	pair, _, err := testManager().Issue("user-uuid")
	if err != nil {
		t.Fatal(err)
	}
	other := NewManager(config.JWTConfig{Issuer: "campushub-test", AccessSecret: "different-secret", AccessTTL: time.Hour})
	if _, err := other.ParseAccess(pair.AccessToken); err == nil {
		t.Fatal("token signed with another secret must be rejected")
	}
}
