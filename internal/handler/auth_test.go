package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.uber.org/mock/gomock"

	"github.com/turkprogrammer/taskflow/internal/lib"
	"github.com/turkprogrammer/taskflow/internal/service"
	"github.com/turkprogrammer/taskflow/internal/service/mocks"
	"github.com/turkprogrammer/taskflow/internal/store"
)

const testSecret = "test-jwt-secret"

func newAuthHandler(t *testing.T) (*AuthHandler, *mocks.MockUserStore) {
	t.Helper()
	ctrl := gomock.NewController(t)
	mockUser := mocks.NewMockUserStore(ctrl)
	svc := service.NewAuthService(mockUser, testSecret, 72)
	return NewAuthHandler(svc, testSecret), mockUser
}

func TestAuthHandler_Register_Success(t *testing.T) {
	h, mock := newAuthHandler(t)

	mock.EXPECT().GetByEmail(gomock.Any(), "new@test.com").Return(nil, nil)
	mock.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ interface{}, u *store.User) error {
		u.ID = 1
		return nil
	})

	body, _ := json.Marshal(map[string]string{"email": "new@test.com", "password": "secret", "name": "New"})
	req := httptest.NewRequest("POST", "/api/v1/register", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.Register(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["token"] == nil || resp["token"] == "" {
		t.Fatal("expected non-empty token")
	}
}

func TestAuthHandler_Register_DuplicateEmail(t *testing.T) {
	h, mock := newAuthHandler(t)

	mock.EXPECT().GetByEmail(gomock.Any(), "dup@test.com").Return(&store.User{ID: 1}, nil)

	body, _ := json.Marshal(map[string]string{"email": "dup@test.com", "password": "secret", "name": "Dup"})
	req := httptest.NewRequest("POST", "/api/v1/register", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.Register(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", w.Code)
	}
}

func TestAuthHandler_Register_MissingFields(t *testing.T) {
	h, _ := newAuthHandler(t)

	body, _ := json.Marshal(map[string]string{"email": "", "password": "secret", "name": "Test"})
	req := httptest.NewRequest("POST", "/api/v1/register", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.Register(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestAuthHandler_Register_InvalidJSON(t *testing.T) {
	h, _ := newAuthHandler(t)

	req := httptest.NewRequest("POST", "/api/v1/register", bytes.NewReader([]byte("not json")))
	w := httptest.NewRecorder()

	h.Register(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestAuthHandler_Login_Success(t *testing.T) {
	h, mock := newAuthHandler(t)

	hash, _ := lib.GenerateToken(0, "", "", 0)
	_ = hash
	mock.EXPECT().GetByEmail(gomock.Any(), "user@test.com").Return(&store.User{
		ID:           1,
		Email:        "user@test.com",
		PasswordHash: "$2a$10$USyUvZmt2p0/UqJ1NF1Qzevl90hMWCkyNttsglt96vmgLJKsN16AS",
		Name:         "User",
	}, nil)

	body, _ := json.Marshal(map[string]string{"email": "user@test.com", "password": "any-password"})
	req := httptest.NewRequest("POST", "/api/v1/login", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.Login(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["token"] == nil || resp["token"] == "" {
		t.Fatal("expected non-empty token")
	}
}

func TestAuthHandler_Login_InvalidCredentials(t *testing.T) {
	h, mock := newAuthHandler(t)

	mock.EXPECT().GetByEmail(gomock.Any(), "missing@test.com").Return(nil, service.ErrInvalidCredentials)

	body, _ := json.Marshal(map[string]string{"email": "missing@test.com", "password": "wrong"})
	req := httptest.NewRequest("POST", "/api/v1/login", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.Login(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestAuthHandler_Login_MissingFields(t *testing.T) {
	h, _ := newAuthHandler(t)

	body, _ := json.Marshal(map[string]string{"email": ""})
	req := httptest.NewRequest("POST", "/api/v1/login", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.Login(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestAuthHandler_Middleware_ValidToken(t *testing.T) {
	h, _ := newAuthHandler(t)

	token, err := lib.GenerateToken(42, "user@test.com", testSecret, 72*time.Hour)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	var capturedUserID uint64
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := UserIDFromContext(r.Context())
		if ok {
			capturedUserID = id
		}
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	h.Middleware(inner).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if capturedUserID != 42 {
		t.Fatalf("expected user ID 42, got %d", capturedUserID)
	}
}

func TestAuthHandler_Middleware_MissingHeader(t *testing.T) {
	h, _ := newAuthHandler(t)

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	h.Middleware(inner).ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestAuthHandler_Middleware_InvalidFormat(t *testing.T) {
	h, _ := newAuthHandler(t)

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "InvalidToken")
	w := httptest.NewRecorder()

	h.Middleware(inner).ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestAuthHandler_Middleware_ExpiredToken(t *testing.T) {
	h, _ := newAuthHandler(t)

	token, _ := lib.GenerateToken(1, "user@test.com", testSecret, -1)

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	h.Middleware(inner).ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}
