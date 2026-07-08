// Command api — HTTP JSON API сервер Task Flow.
// Связывает stores, services и handlers; выполняет миграции БД;
// настраивает rate limiting, Prometheus метрики и структурированное логирование;
// обрабатывает все эндпоинты /api/v1/* с graceful shutdown по SIGINT/SIGTERM.
package main

import (
	"embed"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/turkprogrammer/taskflow/internal/config"
	"github.com/turkprogrammer/taskflow/internal/handler"
	"github.com/turkprogrammer/taskflow/internal/service"
	"github.com/turkprogrammer/taskflow/internal/store"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS // embedded SQL-миграции для goose

func main() {
	// Настройка структурированного JSON-логирования
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{})))

	// 1. Загрузка конфигурации
	cfg, err := config.Load("config.yaml")
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// 2. Подключение к MySQL + миграции
	db := initDB(cfg, migrationsFS)

	// 3. Подключение к Redis
	rdb := initRedis(cfg)

	// 4. Инициализация компонентов (Store → Service → Handler)
	userStore := store.NewUserStore(db)
	teamStore := store.NewTeamStore(db, rdb)
	taskStore := store.NewTaskStore(db, rdb)
	reportStore := store.NewReportStore(db)

	authSvc := service.NewAuthService(userStore, cfg.JWT.Secret, cfg.JWT.TTL())
	teamSvc := service.NewTeamService(teamStore)
	taskSvc := service.NewTaskService(taskStore)

	h := &handlers{
		Auth:    handler.NewAuthHandler(authSvc, cfg.JWT.Secret),
		Team:    handler.NewTeamHandler(teamSvc),
		Task:    handler.NewTaskHandler(taskSvc),
		Report:  handler.NewReportHandler(reportStore),
		Metrics: handler.NewMetrics(),
	}

	// 5. Rate limiting (100 req/min, burst 5)
	rateLimiter := handler.NewRateLimiter(100.0/60.0, 5)
	rateLimiter.Cleanup(5*time.Minute, 10*time.Minute)

	// 6. Регистрация маршрутов
	mux := new(http.ServeMux)
	auth := func(next http.HandlerFunc) http.Handler {
		return h.Auth.Middleware(rateLimiter.Middleware(next))
	}
	registerRoutes(mux, h, auth)

	// 7. Middleware chain: логирование → метрики → маршруты
	handler := withLogging(h.Metrics.Middleware(mux))

	// 8. Запуск сервера
	srv := &http.Server{
		Addr:    ":" + strconv.Itoa(cfg.Server.Port),
		Handler: handler,
	}
	runServer(srv, cfg)

	// 9. Ожидание сигнала завершения
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	slog.Info("shutting down", "signal", sig)

	// 10. Graceful shutdown
	shutdown(srv, rdb, db, rateLimiter)
}
