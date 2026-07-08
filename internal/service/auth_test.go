package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/mock/gomock"

	"github.com/turkprogrammer/taskflow/internal/lib"
	"github.com/turkprogrammer/taskflow/internal/service/mocks"
	"github.com/turkprogrammer/taskflow/internal/store"
)

func TestAuthService_Register_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserStore := mocks.NewMockUserStore(ctrl)
	svc := NewAuthService(mockUserStore, "test-secret", 72)

	mockUserStore.EXPECT().
		GetByEmail(gomock.Any(), "new@test.com").
		Return(nil, errors.New("not found"))

	mockUserStore.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, u *store.User) error {
			u.ID = 1
			return nil
		})

	resp, err := svc.Register(context.Background(), RegisterInput{
		Email:    "new@test.com",
		Password: "secret123",
		Name:     "New User",
	})
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if resp.User.ID != 1 {
		t.Fatalf("expected user ID 1, got %d", resp.User.ID)
	}
	if resp.User.Email != "new@test.com" {
		t.Fatalf("expected email new@test.com, got %s", resp.User.Email)
	}
	if resp.Token == "" {
		t.Fatal("expected non-empty token")
	}
}

func TestAuthService_Register_DuplicateEmail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserStore := mocks.NewMockUserStore(ctrl)
	svc := NewAuthService(mockUserStore, "test-secret", 72)

	mockUserStore.EXPECT().
		GetByEmail(gomock.Any(), "dup@test.com").
		Return(&store.User{ID: 1, Email: "dup@test.com"}, nil)

	_, err := svc.Register(context.Background(), RegisterInput{
		Email:    "dup@test.com",
		Password: "secret123",
		Name:     "Dup",
	})
	if err == nil || err.Error() != "email already registered" {
		t.Fatalf("expected 'email already registered', got %v", err)
	}
}

func TestAuthService_Register_CreateError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserStore := mocks.NewMockUserStore(ctrl)
	svc := NewAuthService(mockUserStore, "test-secret", 72)

	mockUserStore.EXPECT().
		GetByEmail(gomock.Any(), "createerr@test.com").
		Return(nil, errors.New("not found"))

	mockUserStore.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(errors.New("db constraint"))

	_, err := svc.Register(context.Background(), RegisterInput{
		Email:    "createerr@test.com",
		Password: "secret123",
		Name:     "Error",
	})
	if err == nil {
		t.Fatal("expected error when Create fails")
	}
}

func TestAuthService_Login_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserStore := mocks.NewMockUserStore(ctrl)
	svc := NewAuthService(mockUserStore, "test-secret", 72)

	mockUserStore.EXPECT().
		GetByEmail(gomock.Any(), "existing@test.com").
		Return(&store.User{
			ID:           1,
			Email:        "existing@test.com",
			Name:         "Existing",
			PasswordHash: "$2a$10$USyUvZmt2p0/UqJ1NF1Qzevl90hMWCkyNttsglt96vmgLJKsN16AS",
		}, nil)

	resp, err := svc.Login(context.Background(), LoginInput{
		Email:    "existing@test.com",
		Password: "any-password",
	})
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	if resp.User.ID != 1 {
		t.Fatalf("expected user ID 1, got %d", resp.User.ID)
	}
	if resp.Token == "" {
		t.Fatal("expected non-empty token")
	}
}

func TestAuthService_Login_UserNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserStore := mocks.NewMockUserStore(ctrl)
	svc := NewAuthService(mockUserStore, "test-secret", 72)

	mockUserStore.EXPECT().
		GetByEmail(gomock.Any(), "missing@test.com").
		Return(nil, errors.New("not found"))

	_, err := svc.Login(context.Background(), LoginInput{
		Email:    "missing@test.com",
		Password: "secret",
	})
	if err == nil || err.Error() != "invalid email or password" {
		t.Fatalf("expected 'invalid email or password', got %v", err)
	}
}

func TestAuthService_Login_NilUserNoError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserStore := mocks.NewMockUserStore(ctrl)
	svc := NewAuthService(mockUserStore, "test-secret", 72)

	mockUserStore.EXPECT().
		GetByEmail(gomock.Any(), "nil@test.com").
		Return(nil, nil)

	_, err := svc.Login(context.Background(), LoginInput{
		Email:    "nil@test.com",
		Password: "secret",
	})
	if err == nil {
		t.Fatal("expected error for nil user")
	}
}

func TestJWT_GenerateAndValidate(t *testing.T) {
	secret := "my-secret-key"
	ttl := time.Hour

	token, err := lib.GenerateToken(42, "user@test.com", secret, ttl)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	claims, err := lib.ValidateToken(token, secret)
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
	_, err := lib.ValidateToken("invalid-token", "secret")
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
}

func TestJWT_WrongSecret(t *testing.T) {
	secret := "real-secret"
	wrongSecret := "wrong-secret"
	ttl := time.Hour

	token, err := lib.GenerateToken(1, "test@test.com", secret, ttl)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	_, err = lib.ValidateToken(token, wrongSecret)
	if err == nil {
		t.Fatal("expected error for wrong secret")
	}
}
