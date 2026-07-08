// Пackage lib реализует общие утилиты: JWT-аутентификацию и circuit breaker.
package lib

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims — payload JWT-токена. Содержит ID и email пользователя,
// а также стандартные registered claims (срок действия, время создания).
type Claims struct {
	UserID uint64 `json:"user_id"` // ID пользователя
	Email  string `json:"email"`   // Email пользователя
	jwt.RegisteredClaims
}

// GenerateToken создаёт подписанный HS256 JWT с указанными userID и email.
// Токен истекает через ttl от момента создания. Используется при входе/регистрации.
func GenerateToken(userID uint64, email string, secret string, ttl time.Duration) (string, error) {
	claims := &Claims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// ValidateToken парсит и валидирует подписанный JWT.
// Возвращает Claims при успехе или ошибку при истечении срока,
// повреждении токена или неверной подписи.
func ValidateToken(tokenStr string, secret string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, jwt.ErrSignatureInvalid
	}

	return claims, nil
}
