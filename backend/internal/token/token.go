// Package token signs and verifies the JSON Web Tokens used for CampusHub
// authentication.
//
// It deliberately depends on nothing but the JWT library and the JWT
// configuration block, so both the service layer and the HTTP middleware can
// use it without importing each other or creating an import cycle.
package token

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/wangn-tech/campus-hub/internal/config"
)

// ErrInvalid is returned when a token is missing, malformed, expired, signed
// with the wrong secret, or has the wrong type.
var ErrInvalid = errors.New("invalid token")

// kind distinguishes the two token types issued by a Manager.
type kind uint8

const (
	access kind = iota
	refresh
)

// refreshType is the value stored in the reserved "typ" claim of refresh
// tokens. Access tokens intentionally omit the claim so that tokens issued by
// earlier builds keep working.
const refreshType = "refresh"

// Manager signs and verifies access and refresh tokens for a single issuer.
type Manager struct {
	issuer        string
	accessSecret  []byte
	refreshSecret []byte
	accessTTL     time.Duration
	refreshTTL    time.Duration
}

// NewManager builds a Manager from the JWT configuration block.
func NewManager(cfg config.JWTConfig) *Manager {
	return &Manager{
		issuer:        cfg.Issuer,
		accessSecret:  []byte(cfg.AccessSecret),
		refreshSecret: []byte(cfg.RefreshSecret),
		accessTTL:     cfg.AccessTTL,
		refreshTTL:    cfg.RefreshTTL,
	}
}

// Claims is the validated payload extracted from a token.
type Claims struct {
	Subject   string
	ID        string
	ExpiresAt int64
}

// Pair is a freshly issued access/refresh token pair.
type Pair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

// RefreshTTL is the lifetime of issued refresh tokens.
func (m *Manager) RefreshTTL() time.Duration { return m.refreshTTL }

// Issue creates a token pair for subject. It also returns the refresh token's
// jti so callers can track or revoke the session.
func (m *Manager) Issue(subject string) (*Pair, string, error) {
	now := time.Now().UTC()
	accessToken, err := m.sign(access, subject, uuid.NewString(), now, m.accessTTL, m.accessSecret)
	if err != nil {
		return nil, "", err
	}
	refreshID := uuid.NewString()
	refreshToken, err := m.sign(refresh, subject, refreshID, now, m.refreshTTL, m.refreshSecret)
	if err != nil {
		return nil, "", err
	}
	pair := &Pair{AccessToken: accessToken, RefreshToken: refreshToken, ExpiresIn: int64(m.accessTTL.Seconds())}
	return pair, refreshID, nil
}

// ParseAccess validates an access token and returns its claims.
func (m *Manager) ParseAccess(raw string) (*Claims, error) {
	return m.parse(raw, access, m.accessSecret)
}

// ParseRefresh validates a refresh token and returns its claims.
func (m *Manager) ParseRefresh(raw string) (*Claims, error) {
	return m.parse(raw, refresh, m.refreshSecret)
}

func (m *Manager) sign(k kind, subject, id string, now time.Time, ttl time.Duration, secret []byte) (string, error) {
	claims := jwt.MapClaims{
		"sub": subject,
		"jti": id,
		"iss": m.issuer,
		"iat": now.Unix(),
		"exp": now.Add(ttl).Unix(),
	}
	if k == refresh {
		claims["typ"] = refreshType
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secret)
}

func (m *Manager) parse(raw string, want kind, secret []byte) (*Claims, error) {
	parsed, err := jwt.Parse(raw, func(parsed *jwt.Token) (any, error) {
		if parsed.Method != jwt.SigningMethodHS256 {
			return nil, ErrInvalid
		}
		return secret, nil
	}, jwt.WithIssuer(m.issuer))
	if err != nil || !parsed.Valid {
		return nil, ErrInvalid
	}
	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return nil, ErrInvalid
	}
	subject, _ := claims["sub"].(string)
	id, _ := claims["jti"].(string)
	typ, _ := claims["typ"].(string)
	exp, ok := claims["exp"].(float64)
	if subject == "" || id == "" || !ok || (want == refresh) != (typ == refreshType) {
		return nil, ErrInvalid
	}
	return &Claims{Subject: subject, ID: id, ExpiresAt: int64(exp)}, nil
}
