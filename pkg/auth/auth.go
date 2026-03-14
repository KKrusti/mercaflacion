// Package auth provides password hashing and JWT token utilities.
package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const (
	tokenTTL      = 24 * time.Hour // reduced from 72h for tighter expiry
	bcryptCost    = 12
	jwtSecretEnv  = "JWT_SECRET"
	defaultSecret = "change-me-in-production"
)

// jwtSecret returns the signing secret from the environment, falling back to
// a hardcoded default that is only safe for local development.
func jwtSecret() []byte {
	if s := os.Getenv(jwtSecretEnv); s != "" {
		return []byte(s)
	}
	return []byte(defaultSecret)
}

// HashPassword returns the bcrypt hash of the plain-text password.
func HashPassword(plain string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), bcryptCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

// CheckPassword returns nil when plain matches the stored bcrypt hash.
func CheckPassword(plain, hash string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain))
}

type claims struct {
	UserID  int64 `json:"uid"`
	IsAdmin bool  `json:"adm"`
	jwt.RegisteredClaims
}

// generateJTI returns a cryptographically random 16-byte hex string used as
// the JWT ID (jti claim) for token revocation support.
func generateJTI() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate jti: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// GenerateToken creates a signed JWT for the given user ID valid for tokenTTL.
// isAdmin is embedded in the token claims so the middleware can propagate it
// without an extra database lookup on every request.
func GenerateToken(userID int64, isAdmin bool) (string, error) {
	jti, err := generateJTI()
	if err != nil {
		return "", err
	}
	now := time.Now()
	c := claims{
		UserID:  userID,
		IsAdmin: isAdmin,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(tokenTTL)),
		},
	}
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(jwtSecret())
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}
	return tok, nil
}

// ValidateToken parses and validates a JWT string, returning the user ID,
// admin flag, JTI (unique token identifier used for revocation), and expiry
// time on success.
func ValidateToken(tokenStr string) (userID int64, isAdmin bool, jti string, expiresAt time.Time, err error) {
	tok, err := jwt.ParseWithClaims(tokenStr, &claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return jwtSecret(), nil
	})
	if err != nil {
		return 0, false, "", time.Time{}, fmt.Errorf("parse token: %w", err)
	}

	c, ok := tok.Claims.(*claims)
	if !ok || !tok.Valid {
		return 0, false, "", time.Time{}, errors.New("invalid token claims")
	}

	var exp time.Time
	if c.ExpiresAt != nil {
		exp = c.ExpiresAt.Time
	}
	return c.UserID, c.IsAdmin, c.RegisteredClaims.ID, exp, nil
}
