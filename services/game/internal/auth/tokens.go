// Package auth implements Google sign-in, short-lived access JWTs and rotating refresh tokens.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	issuer   = "tiendao"
	audience = "tiendao-api"
)

// Principal is the authenticated caller.
type Principal struct {
	UserID uuid.UUID
	Role   string // player | admin
}

// IsAdmin reports admin role.
func (p Principal) IsAdmin() bool { return p.Role == "admin" }

// Issuer signs and verifies access tokens (HS256).
type Issuer struct {
	secret []byte
	ttl    time.Duration
	now    func() time.Time
}

// NewIssuer requires a secret of at least 32 bytes.
func NewIssuer(secret []byte, ttl time.Duration, now func() time.Time) (*Issuer, error) {
	if len(secret) < 32 {
		return nil, errors.New("auth: JWT secret must be at least 32 bytes")
	}
	return &Issuer{secret: secret, ttl: ttl, now: now}, nil
}

// TTL returns the access-token lifetime.
func (i *Issuer) TTL() time.Duration { return i.ttl }

type claims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

// Issue signs an access token for p.
func (i *Issuer) Issue(p Principal) (string, error) {
	now := i.now()
	c := claims{Role: p.Role, RegisteredClaims: jwt.RegisteredClaims{
		Issuer: issuer, Audience: jwt.ClaimStrings{audience}, Subject: p.UserID.String(),
		IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(i.ttl)),
	}}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(i.secret)
}

// Verify parses and validates an access token.
func (i *Issuer) Verify(token string) (Principal, error) {
	var c claims
	_, err := jwt.ParseWithClaims(token, &c, func(t *jwt.Token) (any, error) { return i.secret, nil },
		jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer(issuer), jwt.WithAudience(audience),
		jwt.WithExpirationRequired(), jwt.WithTimeFunc(i.now), jwt.WithLeeway(5*time.Second))
	if err != nil {
		return Principal{}, fmt.Errorf("auth: invalid access token: %w", err)
	}
	id, err := uuid.Parse(c.Subject)
	if err != nil {
		return Principal{}, fmt.Errorf("auth: bad subject: %w", err)
	}
	return Principal{UserID: id, Role: c.Role}, nil
}

// newOpaqueToken returns a random URL-safe token and its storage hash.
func newOpaqueToken() (token, hash string) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	token = base64.RawURLEncoding.EncodeToString(b)
	return token, hashToken(token)
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

var randRead = rand.Read
