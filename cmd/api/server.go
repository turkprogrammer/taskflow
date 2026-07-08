package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/turkprogrammer/taskflow/internal/config"
	"github.com/turkprogrammer/taskflow/internal/handler"
)

// handlers — контейнер всех HTTP-хендлеров.
type handlers struct {
	Auth    *handler.AuthHandler
	Team    *handler.TeamHandler
	Task    *handler.TaskHandler
	Report  *handler.ReportHandler
	Metrics *handler.Metrics
}

// registerRoutes регистрирует все маршруты API в http.ServeMux.
func registerRoutes(mux *http.ServeMux, h *handlers, authMiddleware func(http.HandlerFunc) http.Handler) {
	// Health check (без авторизации)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"status":"ok"}`)); err != nil {
			slog.Error("failed to write health response", "error", err)
		}
	})

	// Auth эндпоинты (без авторизации)
	mux.HandleFunc("POST /api/v1/register", h.Auth.Register)
	mux.HandleFunc("POST /api/v1/login", h.Auth.Login)

	// Teams (с авторизацией)
	mux.Handle("POST /api/v1/teams", authMiddleware(h.Team.Create))
	mux.Handle("GET /api/v1/teams", authMiddleware(h.Team.List))
	mux.Handle("POST /api/v1/teams/{id}/invite", authMiddleware(h.Team.Invite))

	// Tasks (с авторизацией)
	mux.Handle("POST /api/v1/tasks", authMiddleware(h.Task.Create))
	mux.Handle("GET /api/v1/tasks", authMiddleware(h.Task.List))
	mux.Handle("PUT /api/v1/tasks/{id}", authMiddleware(h.Task.Update))
	mux.Handle("GET /api/v1/tasks/{id}/history", authMiddleware(h.Task.GetHistory))

	// Reports (с авторизацией)
	mux.Handle("GET /api/v1/teams/{id}/dashboard", authMiddleware(h.Report.TeamDashboard))
	mux.Handle("GET /api/v1/reports/top-users", authMiddleware(h.Report.TopUsers))
	mux.Handle("GET /api/v1/reports/inefficient-tasks", authMiddleware(h.Report.InefficientTasks))

	// Prometheus метрики (без авторизации)
	mux.Handle("GET /metrics", h.Metrics.Handler())
}

// runServer запускает HTTP-сервер в отдельной горутине.
func runServer(srv *http.Server, cfg *config.Config) {
	go func() {
		slog.Info("server starting", "port", cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()
}

// shutdown выполняет graceful shutdown сервера и закрывает соединения.
func shutdown(srv *http.Server, rdb *redis.Client, db interface{ Close() error }, rateLimiter *handler.RateLimiter) {
	slog.Info("shutting down")

	// Завершение с таймаутом 10 секунд
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
	}

	// Остановка фоновой очистки rate limiter
	rateLimiter.StopCleanup()

	// Закрытие соединения с Redis
	if err := rdb.Close(); err != nil {
		slog.Error("redis close error", "error", err)
	}

	// Закрытие соединения с БД
	if err := db.Close(); err != nil {
		slog.Error("database close error", "error", err)
	}

	slog.Info("server stopped")
}

// withLogging — middleware для структурированного логирования запросов/ответов.
func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"remote", r.RemoteAddr,
		)
		next.ServeHTTP(w, r)
		slog.Info("response",
			"method", r.Method,
			"path", r.URL.Path,
			"duration", time.Since(start),
		)
	})
}
