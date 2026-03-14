package auth_test

import (
	"testing"

	"basket-cost/pkg/auth"
	"golang.org/x/crypto/bcrypt"
)

func TestHashPassword_And_CheckPassword(t *testing.T) {
	plain := "supersecret123"
	hash, err := auth.HashPassword(plain)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if hash == "" {
		t.Fatal("expected non-empty hash")
	}
	if hash == plain {
		t.Error("hash should not equal the plain-text password")
	}
	if err := auth.CheckPassword(plain, hash); err != nil {
		t.Errorf("CheckPassword with correct password: %v", err)
	}
}

func TestHashPassword_UsesBcryptCostAtLeast12(t *testing.T) {
	hash, err := auth.HashPassword("anypassword")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	cost, err := bcrypt.Cost([]byte(hash))
	if err != nil {
		t.Fatalf("bcrypt.Cost: %v", err)
	}
	if cost < 12 {
		t.Errorf("bcrypt cost too low: want >= 12, got %d", cost)
	}
}

func TestCheckPassword_WrongPassword_ReturnsError(t *testing.T) {
	hash, err := auth.HashPassword("correctpassword")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if err := auth.CheckPassword("wrongpassword", hash); err == nil {
		t.Error("expected error for wrong password, got nil")
	}
}

func TestGenerateToken_And_ValidateToken(t *testing.T) {
	const userID int64 = 42
	token, err := auth.GenerateToken(userID)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	gotID, jti, exp, err := auth.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}
	if gotID != userID {
		t.Errorf("ValidateToken: want userID=%d, got %d", userID, gotID)
	}
	if jti == "" {
		t.Error("ValidateToken: expected non-empty jti")
	}
	if exp.IsZero() {
		t.Error("ValidateToken: expected non-zero expiresAt")
	}
}

func TestValidateToken_InvalidToken_ReturnsError(t *testing.T) {
	_, _, _, err := auth.ValidateToken("not.a.valid.jwt")
	if err == nil {
		t.Error("expected error for invalid token, got nil")
	}
}

func TestValidateToken_TamperedToken_ReturnsError(t *testing.T) {
	token, err := auth.GenerateToken(1)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	// Corrupt the signature by appending an extra character.
	_, _, _, err = auth.ValidateToken(token + "x")
	if err == nil {
		t.Error("expected error for tampered token, got nil")
	}
}
