// Пackage config реализует загрузку конфигурации из YAML-файла с перезаписью через ENV.
package config

import (
	"log/slog"
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

// Config — верхнеуровневая структура конфигурации приложения.
type Config struct {
	Server ServerConfig `yaml:"server"` // настройки HTTP-сервера
	DB     DBConfig     `yaml:"db"`     // настройки MySQL
	Redis  RedisConfig  `yaml:"redis"`  // настройки Redis
	JWT    JWTConfig    `yaml:"jwt"`    // настройки JWT
}

// ServerConfig — настройки HTTP-сервера.
type ServerConfig struct {
	Port int `yaml:"port"` // порт прослушивания (по умолчанию 8080)
}

// DBConfig — настройки подключения к MySQL и параметры connection pool.
type DBConfig struct {
	Host               string `yaml:"host"`                 // хост MySQL
	Port               int    `yaml:"port"`                 // порт MySQL
	User               string `yaml:"user"`                 // пользователь MySQL
	Password           string `yaml:"password"`             // пароль MySQL
	Name               string `yaml:"name"`                 // имя базы данных
	MaxOpenConns       int    `yaml:"max_open_conns"`       // макс. открытых соединений
	MaxIdleConns       int    `yaml:"max_idle_conns"`       // макс. idle-соединений
	MaxLifetimeSeconds int    `yaml:"max_lifetime_seconds"` // макс. время жизни соединения (сек)
}

// DSN формирует строку подключения DSN для MySQL.
// Включает charset=utf8mb4, parseTime=true для корректной работы с time.Time.
func (c DBConfig) DSN() string {
	return c.User + ":" + c.Password + "@tcp(" + c.Host + ":" + strconv.Itoa(c.Port) + ")/" + c.Name + "?charset=utf8mb4&parseTime=true&loc=Local"
}

// MaxLifetime конвертирует MaxLifetimeSeconds в time.Duration для sql.ConnMaxLifetime.
func (c DBConfig) MaxLifetime() time.Duration {
	return time.Duration(c.MaxLifetimeSeconds) * time.Second
}

// RedisConfig — настройки подключения к Redis.
type RedisConfig struct {
	Addr     string `yaml:"addr"`     // адрес Redis (host:port)
	Password string `yaml:"password"` // пароль Redis (пусто если нет)
	DB       int    `yaml:"db"`       // номер БД Redis
}

// JWTConfig — настройки JWT-аутентификации.
type JWTConfig struct {
	Secret   string `yaml:"secret"`    // HMAC-ключ для подписи токенов
	TTLHours int    `yaml:"ttl_hours"` // время жизни токена в часах
}

// TTL конвертирует TTLHours в time.Duration для использования как срок действия токена.
func (c JWTConfig) TTL() time.Duration {
	return time.Duration(c.TTLHours) * time.Hour
}

// Load загружает конфигурацию из YAML-файла и применяет перезаписи из ENV.
// Вызывается один раз при старте приложения.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	// Перезапись значений из переменных окружения
	overrideFromEnv(cfg)

	return cfg, nil
}

// overrideFromEnv применяет перезаписи из переменных окружения.
// Поддерживаются: DB_HOST, DB_PORT, DB_USER, DB_PASSWORD, DB_NAME,
// REDIS_ADDR, REDIS_PASSWORD, JWT_SECRET, SERVER_PORT.
func overrideFromEnv(cfg *Config) {
	if v := os.Getenv("DB_HOST"); v != "" {
		cfg.DB.Host = v
	}
	if v := os.Getenv("DB_PORT"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			slog.Warn("invalid DB_PORT", "value", v, "error", err)
		} else {
			cfg.DB.Port = n
		}
	}
	if v := os.Getenv("DB_USER"); v != "" {
		cfg.DB.User = v
	}
	if v := os.Getenv("DB_PASSWORD"); v != "" {
		cfg.DB.Password = v
	}
	if v := os.Getenv("DB_NAME"); v != "" {
		cfg.DB.Name = v
	}
	if v := os.Getenv("REDIS_ADDR"); v != "" {
		cfg.Redis.Addr = v
	}
	if v := os.Getenv("REDIS_PASSWORD"); v != "" {
		cfg.Redis.Password = v
	}
	if v := os.Getenv("JWT_SECRET"); v != "" {
		cfg.JWT.Secret = v
	}
	if v := os.Getenv("SERVER_PORT"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			slog.Warn("invalid SERVER_PORT", "value", v, "error", err)
		} else {
			cfg.Server.Port = n
		}
	}
}
