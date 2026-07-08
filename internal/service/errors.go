package service

import "errors"

// Бизнес-ошибки сервисного слоя.
var (
	// ErrEmailAlreadyRegistered — ошибка при попытке регистрации с уже существующим email.
	ErrEmailAlreadyRegistered = errors.New("email already registered")
	// ErrInvalidCredentials — ошибка при неверном email или пароле.
	ErrInvalidCredentials = errors.New("invalid email or password")
	// ErrAccessDenied — ошибка доступа (недостаточно прав).
	ErrAccessDenied = errors.New("access denied")
	// ErrNotTeamMember — ошибка при попытке выполнить действие без членства в команде.
	ErrNotTeamMember = errors.New("you are not a member of this team")
)
