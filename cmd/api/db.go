package main

import (
	"context"
	"database/sql"
	"embed"
	"log/slog"
	"os"

	_ "github.com/go-sql-driver/mysql"
	"github.com/redis/go-redis/v9"

	"github.com/turkprogrammer/taskflow/internal/config"
	"github.com/turkprogrammer/taskflow/internal/migrate"
)

// initDB подключается к MySQL, настраивает connection pool и выполняет миграции.
func initDB(cfg *config.Config, migrationsFS embed.FS) *sql.DB {
	db, err := sql.Open("mysql", cfg.DB.DSN())
	if err != nil {
		slog.Error("failed to open database", "error", err)
		os.Exit(1)
	}

	// Настройка параметров connection pool
	db.SetMaxOpenConns(cfg.DB.MaxOpenConns)
	db.SetMaxIdleConns(cfg.DB.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.DB.MaxLifetime())

	// Проверка доступности БД
	if err := db.Ping(); err != nil {
		slog.Error("failed to ping database", "error", err)
		os.Exit(1)
	}
	slog.Info("database connected")

	// Запуск автоматических миграций (goose)
	if err := migrate.Run(db, migrationsFS, "migrations"); err != nil {
		slog.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	return db
}

// initRedis подключается к Redis и проверяет доступность.
func initRedis(cfg *config.Config) *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	if err := rdb.Ping(context.Background()).Err(); err != nil {
		slog.Error("failed to connect to redis", "error", err)
		os.Exit(1)
	}
	slog.Info("redis connected")

	return rdb
}
