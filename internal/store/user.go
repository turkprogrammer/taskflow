// Пackage store реализует уровень доступа к данным (MySQL + Redis кеш).
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// User представляет зарегистрированного пользователя в системе.
type User struct {
	// ID — уникальный идентификатор пользователя (автоинкремент).
	ID uint64 `json:"id"`
	// Email — адрес электронной почты, используется для входа в систему.
	Email string `json:"email"`
	// PasswordHash — bcrypt-хеш пароля. Не возвращается в JSON (json:"-").
	PasswordHash string `json:"-"`
	// Name — отображаемое имя пользователя.
	Name string `json:"name"`
	// CreatedAt —.timestamp создания записи.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt — timestamp последнего обновления записи.
	UpdatedAt time.Time `json:"updated_at"`
}

// UserStore реализует CRUD-операции для пользователей через MySQL.
type UserStore struct {
	// db — подключение к MySQL.
	db *sql.DB
}

// NewUserStore создаёт новый UserStore с переданным подключением к БД.
func NewUserStore(db *sql.DB) *UserStore {
	return &UserStore{db: db}
}

// Create создаёт нового пользователя в БД.
// Устанавливает ID, CreatedAt, UpdatedAt. Возвращает ошибку при проблемах с вставкой.
func (s *UserStore) Create(ctx context.Context, u *User) error {
	if u == nil {
		return errors.New("user is nil")
	}
	now := time.Now()
	u.CreatedAt = now
	u.UpdatedAt = now

	// Вставка записи в таблицу users
	result, err := s.db.ExecContext(ctx,
		"INSERT INTO users (email, password_hash, name) VALUES (?, ?, ?)",
		u.Email, u.PasswordHash, u.Name,
	)
	if err != nil {
		return fmt.Errorf("creating user: %w", err)
	}

	// Получение сгенерированного ID после вставки
	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("getting user last insert id: %w", err)
	}

	if id < 0 {
		return fmt.Errorf("negative user id: %d", id)
	}
	u.ID = uint64(id)
	return nil
}

// GetByEmail находит пользователя по адресу электронной почты.
// Возвращает sql.ErrNoRows, если пользователь не найден.
func (s *UserStore) GetByEmail(ctx context.Context, email string) (*User, error) {
	u := &User{}
	err := s.db.QueryRowContext(ctx,
		"SELECT id, email, password_hash, name, created_at, updated_at FROM users WHERE email = ?",
		email,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Name, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		return nil, fmt.Errorf("getting user by email: %w", err)
	}

	return u, nil
}

// GetByID находит пользователя по его уникальному идентификатору.
// Возвращает sql.ErrNoRows, если пользователь не найден.
func (s *UserStore) GetByID(ctx context.Context, id uint64) (*User, error) {
	u := &User{}
	err := s.db.QueryRowContext(ctx,
		"SELECT id, email, password_hash, name, created_at, updated_at FROM users WHERE id = ?",
		id,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Name, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		return nil, fmt.Errorf("getting user by id: %w", err)
	}

	return u, nil
}
