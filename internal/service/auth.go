// Пackage service реализует бизнес-логику API: аутентификацию, управление
// командами и задачами. Зависимости от store-слоя внедряются через интерфейсы.
package service

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/turkprogrammer/taskflow/internal/lib"
	"github.com/turkprogrammer/taskflow/internal/store"
)

// AuthService обрабатывает регистрацию и аутентификацию пользователей.
// Хеширует пароли bcrypt и выдаёт подписанные JWT-токены.
type AuthService struct {
	users  UserStore  // интерфейс доступа к данным пользователей
	secret string     // HMAC-ключ для подписи JWT
	ttl    time.Duration // время жизни токена
}

// NewAuthService создаёт новый AuthService с переданными зависимостями.
func NewAuthService(users UserStore, secret string, ttl time.Duration) *AuthService {
	return &AuthService{users: users, secret: secret, ttl: ttl}
}

// RegisterInput — входные данные для регистрации нового пользователя.
type RegisterInput struct {
	// Email — уникальный адрес электронной почты (используется для входа).
	Email string
	// Password — пароль в открытом виде (хешируется перед сохранением).
	Password string
	// Name — отображаемое имя пользователя.
	Name string
}

// AuthResponse — ответ на успешную регистрацию или вход.
type AuthResponse struct {
	// Token — подписанный JWT-токен для авторизации запросов.
	Token string `json:"token"`
	// User — профиль пользователя.
	User *store.User `json:"user"`
}

// Register создаёт нового пользователя, хеширует пароль, сохраняет в БД
// и возвращает подписанный JWT. Возвращает ошибку, если email уже занят.
func (s *AuthService) Register(ctx context.Context, input RegisterInput) (*AuthResponse, error) {
	// Проверка уникальности email
	existing, err := s.users.GetByEmail(ctx, input.Email)
	if err != nil {
		// Если пользователь не найден — продолжаем регистрацию
		slog.Warn("register: GetByEmail error (treating as not found)", "error", err)
	}
	if existing != nil {
		return nil, ErrEmailAlreadyRegistered
	}

	// Хеширование пароля bcrypt
	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	// Создание пользователя в БД
	user := &store.User{
		Email:        input.Email,
		PasswordHash: string(hash),
		Name:         input.Name,
	}
	if err := s.users.Create(ctx, user); err != nil {
		return nil, err
	}

	// Генерация JWT-токена
	token, err := lib.GenerateToken(user.ID, user.Email, s.secret, s.ttl)
	if err != nil {
		return nil, err
	}

	return &AuthResponse{Token: token, User: user}, nil
}

// LoginInput — входные данные для аутентификации.
type LoginInput struct {
	// Email — зарегистрированный адрес электронной почты.
	Email string
	// Password — пароль в открытом виде.
	Password string
}

// Login проверяет email/пароль и возвращает JWT при успехе.
// Возвращает ErrInvalidCredentials при неверных данных (без утечки информации о наличии email).
func (s *AuthService) Login(ctx context.Context, input LoginInput) (*AuthResponse, error) {
	// Поиск пользователя по email
	user, err := s.users.GetByEmail(ctx, input.Email)
	if err != nil {
		return nil, ErrInvalidCredentials
	}
	if user == nil {
		// Защита от программных ошибок (store вернул nil без ошибки)
		return nil, errors.New("store returned nil user without error")
	}

	// Проверка пароля через bcrypt
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	// Генерация JWT-токена
	token, err := lib.GenerateToken(user.ID, user.Email, s.secret, s.ttl)
	if err != nil {
		return nil, err
	}

	return &AuthResponse{Token: token, User: user}, nil
}
