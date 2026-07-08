package lib

import (
	"testing"
	"time"
)

func TestJWT_GenerateAndValidate(t *testing.T) {
	secret := "my-secret-key"
	ttl := time.Hour

	token, err := GenerateToken(42, "user@test.com", secret, ttl)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	claims, err := ValidateToken(token, secret)
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}
	if claims.UserID != 42 {
		t.Fatalf("expected UserID 42, got %d", claims.UserID)
	}
	if claims.Email != "user@test.com" {
		t.Fatalf("expected email user@test.com, got %s", claims.Email)
	}
}

func TestJWT_InvalidToken(t *testing.T) {
	_, err := ValidateToken("invalid-token", "secret")
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
}

func TestJWT_WrongSecret(t *testing.T) {
	secret := "real-secret"
	wrongSecret := "wrong-secret"
	ttl := time.Hour

	token, err := GenerateToken(1, "test@test.com", secret, ttl)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	_, err = ValidateToken(token, wrongSecret)
	if err == nil {
		t.Fatal("expected error for wrong secret")
	}
}

func TestJWT_ExpiredToken(t *testing.T) {
	secret := "test-secret"
	token, err := GenerateToken(1, "test@test.com", secret, -time.Hour)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	_, err = ValidateToken(token, secret)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}
