// Пackage handler реализует HTTP-хендлеры и middleware для API Task Flow.
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/turkprogrammer/taskflow/internal/lib"
	"github.com/turkprogrammer/taskflow/internal/service"
)

// contextKey — неэкспортируемый тип для типизированных ключей контекста.
type contextKey string

// UserIDKey — ключ контекста для ID аутентифицированного пользователя.
const UserIDKey contextKey = "user_id"

// UserEmailKey — ключ контекста для email аутентифицированного пользователя.
const UserEmailKey contextKey = "user_email"

// AuthHandler реализует регистрацию, вход и JWT-аутентификацию middleware.
type AuthHandler struct {
	svc    *service.AuthService // бизнес-логика аутентификации
	secret string               // HMAC-ключ для подписи/валидации JWT
}

// NewAuthHandler создаёт новый AuthHandler с указанным сервисом и секретом.
func NewAuthHandler(svc *service.AuthService, secret string) *AuthHandler {
	return &AuthHandler{svc: svc, secret: secret}
}

// Register обрабатывает POST /api/v1/register.
// Принимает JSON с email, password, name. Возвращает JWT при успехе.
// Возвращает 409 Conflict, если email уже зарегистрирован.
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Name     string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	// Валидация обязательных полей
	if body.Email == "" || body.Password == "" || body.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email, password, and name are required"})
		return
	}

	resp, err := h.svc.Register(r.Context(), service.RegisterInput{
		Email:    body.Email,
		Password: body.Password,
		Name:     body.Name,
	})
	if err != nil {
		if errors.Is(err, service.ErrEmailAlreadyRegistered) {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "email already registered"})
			return
		}
		slog.Error("register failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusCreated, resp)
}

// Login обрабатывает POST /api/v1/login.
// Принимает JSON с email, password. Возвращает JWT при успехе.
// Возвращает 401 Unauthorized при неверных учётных данных.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	// Валидация обязательных полей
	if body.Email == "" || body.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email and password are required"})
		return
	}

	resp, err := h.svc.Login(r.Context(), service.LoginInput{
		Email:    body.Email,
		Password: body.Password,
	})
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid email or password"})
			return
		}
		slog.Error("login failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// Middleware — HTTP middleware для извлечения и валидации Bearer JWT из заголовка Authorization.
// При успехе добавляет UserIDKey и UserEmailKey в контекст запроса.
// При ошибке возвращает 401 Unauthorized.
func (h *AuthHandler) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Проверка наличия заголовка Authorization
		auth := r.Header.Get("Authorization")
		if auth == "" {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing authorization header"})
			return
		}

		// Проверка формата "Bearer <token>"
		parts := strings.SplitN(auth, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid authorization header"})
			return
		}

		// Валидация JWT-токена
		claims, err := lib.ValidateToken(parts[1], h.secret)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid or expired token"})
			return
		}

		// Добавление данных пользователя в контекст
		ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
		ctx = context.WithValue(ctx, UserEmailKey, claims.Email)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// writeJSON — вспомогательная функция для отправки JSON-ответа.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("failed to encode json response", "error", err)
	}
}

// UserIDFromContext извлекает ID пользователя из контекста запроса.
// Возвращает false, если значение отсутствует или не является uint64.
func UserIDFromContext(ctx context.Context) (uint64, bool) {
	id, ok := ctx.Value(UserIDKey).(uint64)
	return id, ok
}
