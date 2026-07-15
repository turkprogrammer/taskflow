# Task Flow

REST API сервис для управления задачами в командах с поддержкой ролевой модели, истории изменений и сложными SQL-запросами.

[![Go Version](https://img.shields.io/badge/go-1.26-blue)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)
[![Build](https://img.shields.io/badge/build-passing-brightgreen)]
[![Coverage](https://img.shields.io/badge/coverage-87%25-success)]

## Технологический стек

| Компонент | Технология |
|-----------|------------|
| Язык | Go 1.26 |
| БД | MySQL 8 |
| Кеш | Redis 7 |
| Контейнеризация | Docker + Docker Compose |
| Аутентификация | JWT (golang-jwt/jwt/v5) |
| Метрики | Prometheus (prometheus/client_golang) |
| Rate Limiting | golang.org/x/time/rate |
| Circuit Breaker | sony/gobreaker |
| Миграции | pressly/goose |
| Тесты | go.uber.org/mock, dockertest |

## Структура базы данных

### Таблицы (6)

```
users               — Пользователи
teams               — Команды
team_members        — Связь пользователь ↔ команда (M:N) + роль
tasks               — Задачи
task_history        — История изменений задач (аудит)
task_comments       — Комментарии к задачам
```

### Внешние ключи (10 связей)

```
teams.created_by        → users.id
team_members.user_id    → users.id
team_members.team_id    → teams.id
tasks.assignee_id       → users.id
tasks.team_id           → teams.id
tasks.created_by        → users.id
task_history.task_id    → tasks.id
task_history.changed_by → users.id
task_comments.task_id   → tasks.id
task_comments.user_id   → users.id
```

### Составные индексы

- `idx_tasks_team_status` — (team_id, status)
- `idx_tasks_assignee_status` — (assignee_id, status)
- `uq_team_member` — (team_id, user_id) UNIQUE

## Архитектура приложения

Проект построен на **трёхуровневой архитектуре** (Handler → Service → Store) с ручным constructor injection — без DI-контейнеров, ORM или лишних абстракций. Каждый слой имеет ровно одну ответственность и тестируется независимо.

- **Handler Layer** — принимает HTTP-запросы, парсит параметры, возвращает JSON. Без бизнес-логики. Хендлер получает сервис через конструктор.
- **Service Layer** — бизнес-логика: валидация прав (проверка членства в команде, ролевая модель), координация между store-запросами. Нет прямого доступа к БД — только через Store interface.
- **Store Layer** — доступ к данным (MySQL) и кеш (Redis). Чистые SQL-запросы без ORM. Единственный слой, который знает о БД.

Сборка компонентов происходит в `main()` — это единственное место, где слои пересекаются:

```
main()
  → config.Load()                # конфигурация (YAML + ENV)
  → initDB(), initRedis()        # подключение к MySQL + Redis
  → NewUserStore(db)             # Store layer
  → NewTeamStore(db, rdb)
  → NewTaskStore(db, rdb)
  → NewReportStore(db)
  → NewAuthService(userStore)    # Service layer (через интерфейсы)
  → NewTeamService(teamStore)
  → NewTaskService(taskStore)
  → NewAuthHandler(authSvc)      # Handler layer (через сервисы)
  → NewTeamHandler(teamSvc)
  → NewTaskHandler(taskSvc)
  → NewReportHandler(reportStore)
  → registerRoutes()             # привязка к http.ServeMux
  → runServer()                  # запуск + graceful shutdown
```

Разделение подтверждено статическим анализом: три слоя формируют изолированные кластеры зависимостей, а `main()` выступает единственным «мостом» между ними.

### Трёхуровневая архитектура

```
┌─────────────────────────────────────────────────────────┐
│                    HTTP Request                         │
└─────────────────────────┬───────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────┐
│                   Handler Layer                         │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐  │
│  │   Auth   │ │   Team   │ │   Task   │ │  Report  │  │
│  │ Handler  │ │ Handler  │ │ Handler  │ │ Handler  │  │
│  └────┬─────┘ └────┬─────┘ └────┬─────┘ └────┬─────┘  │
│       │             │            │             │         │
│  ┌────┴─────────────┴────────────┴─────────────┴────┐  │
│  │              Middleware Layer                      │  │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────────────┐  │  │
│  │  │ Auth JWT │ │   Rate   │ │ Prometheus       │  │  │
│  │  │          │ │ Limiter  │ │ Metrics          │  │  │
│  │  └──────────┘ └──────────┘ └──────────────────┘  │  │
│  └──────────────────────────────────────────────────┘  │
└─────────────────────────┬───────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────┐
│                   Service Layer                         │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐               │
│  │   Auth   │ │   Team   │ │   Task   │               │
│  │ Service  │ │ Service  │ │ Service  │               │
│  └────┬─────┘ └────┬─────┘ └────┬─────┘               │
└───────┼─────────────┼────────────┼──────────────────────┘
        │             │            │
        ▼             ▼            ▼
┌─────────────────────────────────────────────────────────┐
│                    Store Layer                          │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐  │
│  │   User   │ │   Team   │ │   Task   │ │  Report  │  │
│  │  Store   │ │  Store   │ │  Store   │ │  Store   │  │
│  └────┬─────┘ └────┬─────┘ └────┬─────┘ └────┬─────┘  │
│       │             │            │             │         │
│  ┌────┴─────────────┴────────────┴─────────────┴────┐  │
│  │           MySQL + Redis Cache (TTL 5 мин)        │  │
│  └──────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────┘
```

### Поток запроса (Request Flow)

```
Клиент → HTTP Request
         │
         ▼
    ┌─────────────────┐
    │  withLogging()  │ ← Логирование метода, пути, длительности
    └────────┬────────┘
             │
             ▼
    ┌─────────────────┐
    │ Metrics.Middleware │ ← Подсчёт запросов, ошибок, длительности
    └────────┬────────┘
             │
             ▼
    ┌─────────────────┐
    │  Auth.Middleware │ ← Валидация JWT, извлечение userID
    └────────┬────────┘
             │
             ▼
    ┌─────────────────┐
    │ RateLimiter     │ ← Проверка лимита (100 req/min/user)
    │   .Middleware   │
    └────────┬────────┘
             │
             ▼
    ┌─────────────────┐
    │  HandlerFunc    │ ← Бизнес-логика запроса
    └────────┬────────┘
             │
             ▼
    ┌─────────────────┐
    │    Service      │ ← Валидация, проверка прав, трансформация
    └────────┬────────┘
             │
             ▼
    ┌─────────────────┐
    │     Store       │ ← SQL-запросы, кеш Redis
    └────────┬────────┘
             │
             ▼
    ┌─────────────────┐
    │  MySQL / Redis  │
    └─────────────────┘
```

### Жизненный цикл приложения

```
┌──────────────────┐
│   main() start   │
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│  Загрузка        │ ← config.yaml + ENV overrides
│  конфигурации    │
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│  Подключение     │ ← sql.Open + SetMaxOpenConns/IdleConns
│  к MySQL         │
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│  Запуск          │ ← goose.Up (автоматические миграции)
│  миграций        │
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│  Подключение     │ ← redis.NewClient
│  к Redis         │
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│  Инициализация   │ ← Store → Service → Handler
│  компонентов     │
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│  Регистрация     │ ← mux.HandleFunc / mux.Handle
│  маршрутов       │
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│  Запуск HTTP     │ ← srv.ListenAndServe()
│  сервера         │
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│  Ожидание        │ ← signal.Notify(SIGINT, SIGTERM)
│  сигнала         │
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│  Graceful        │ ← srv.Shutdown(ctx) + Close DB/Redis
│  Shutdown        │
└──────────────────┘
```

## API Эндпоинты

### Аутентификация (без авторизации)

| Метод | Путь | Описание |
|-------|------|----------|
| POST | `/api/v1/register` | Регистрация нового пользователя |
| POST | `/api/v1/login` | Аутентификация, возврат JWT |

### Команды

| Метод | Путь | Описание | Доступ |
|-------|------|----------|--------|
| POST | `/api/v1/teams` | Создать команду (создатель = owner) | Авторизован |
| GET | `/api/v1/teams` | Список команд пользователя | Авторизован |
| POST | `/api/v1/teams/{id}/invite` | Пригласить пользователя | owner/admin |

### Задачи

| Метод | Путь | Описание | Доступ |
|-------|------|----------|--------|
| POST | `/api/v1/tasks` | Создать задачу | Член команды |
| GET | `/api/v1/tasks` | Список задач (фильтрация + пагинация) | Авторизован |
| PUT | `/api/v1/tasks/{id}` | Обновить задачу | Член команды |
| GET | `/api/v1/tasks/{id}/history` | История изменений | Член команды |

**Параметры фильтрации задач:**
- `team_id` — ID команды
- `status` — статус (todo, in_progress, done)
- `assignee_id` — ID исполнителя
- `page` — номер страницы (по умолчанию 1)
- `per_page` — элементов на странице (по умолчанию 20)

### Отчёты

| Метод | Путь | Описание | Доступ |
|-------|------|----------|--------|
| GET | `/api/v1/teams/{id}/dashboard` | Дашборд команды | Авторизован |
| GET | `/api/v1/reports/top-users` | Топ-3 создателей задач за месяц | Авторизован |
| GET | `/api/v1/reports/inefficient-tasks` | Задачи с нарушением целостности | Авторизован |

### Наблюдаемость

| Метод | Путь | Описание |
|-------|------|----------|
| GET | `/health` | Health check |
| GET | `/metrics` | Prometheus метрики |

## Сложные SQL-запросы

### а) Дашборд команды (JOIN 3+ таблиц + агрегация)

```sql
-- Количество задач по статусам
SELECT status, COUNT(*) as cnt FROM tasks WHERE team_id = ? GROUP BY status

-- Задачи done за 7 дней
SELECT COUNT(*) FROM tasks
WHERE team_id = ? AND status = 'done'
  AND updated_at >= NOW() - INTERVAL 7 DAY

-- Количество участников команды
SELECT COUNT(*) FROM team_members WHERE team_id = ?

-- Статистика по пользователям (JOIN users, team_members, tasks)
SELECT u.id, u.name, COUNT(t.id) as task_count
FROM users u
JOIN team_members tm ON tm.user_id = u.id
LEFT JOIN tasks t ON t.assignee_id = u.id AND t.team_id = ?
WHERE tm.team_id = ?
GROUP BY u.id, u.name
ORDER BY task_count DESC
```

### б) Топ-3 создателей за месяц (оконная функция)

```sql
SELECT team_id, user_id, user_name, task_count, pos
FROM (
    SELECT
        t.team_id,
        t.created_by as user_id,
        u.name as user_name,
        COUNT(*) as task_count,
        ROW_NUMBER() OVER (PARTITION BY t.team_id ORDER BY COUNT(*) DESC) as pos
    FROM tasks t
    JOIN users u ON u.id = t.created_by
    WHERE t.created_at >= DATE_FORMAT(NOW(), '%Y-%m-01')
    GROUP BY t.team_id, t.created_by, u.name
) ranked
WHERE pos <= 3
ORDER BY team_id, pos
```

### в) Валидация целостности (подзапрос)

```sql
-- Задачи, где assignee не является членом команды
SELECT t.id, t.title, t.assignee_id, u.name, t.team_id, tm.name
FROM tasks t
JOIN users u ON u.id = t.assignee_id
JOIN teams tm ON tm.id = t.team_id
WHERE t.assignee_id IS NOT NULL
  AND t.assignee_id NOT IN (
      SELECT tm2.user_id FROM team_members tm2 WHERE tm2.team_id = t.team_id
  )
```

## Оптимизация

| Компонент | Реализация |
|-----------|------------|
| Redis кеш | Список команд пользователя + список задач команды, TTL 5 мин, инвалидация при мутациях |
| Индексы | 16 индексов, включая составные (team_id, status), (assignee_id, status) |
| Connection pooling | MaxOpenConns=25, MaxIdleConns=10, MaxLifetime=300с |
| Пагинация | LIMIT/OFFSET на уровне БД |

## Тестирование

```bash
# Все тесты
go test ./...

# Только service layer
go test ./internal/service/...

# Только handler layer
go test ./internal/handler/...

# С coverage
go test ./internal/... -cover

# Интеграционные тесты (требуют Docker)
go test ./internal/store/...
```

### Coverage по слоям

| Слой | Coverage |
|------|----------|
| service/ | 87.6% |
| handler/ | 81.5% |
| lib/ | 92.6% |
| config/ | 80.0% |
| store/ | 47.8% (интеграционные) |

## Структура проекта

```
cmd/api/
  main.go                 — Точка входа, wire-up компонентов
  db.go                   — initDB, initRedis, connection pool
  server.go               — registerRoutes, runServer, shutdown, withLogging
  migrations/             — SQL миграции (goose)

internal/
  config/
    config.go             — YAML/ENV конфигурация
  handler/
    auth.go               — HTTP хендлеры аутентификации
    team.go               — HTTP хендлеры команд
    task.go               — HTTP хендлеры задач
    report.go             — HTTP хендлеры отчётов
    metrics.go            — Prometheus метрики
    ratelimit.go          — Rate limiter (token bucket)
  lib/
    jwt.go                — Генерация/валидация JWT
    email.go              — Circuit breaker + mock email sender
  migrate/
    migrate.go            — Запуск миграций (goose)
  service/
    auth.go               — Бизнес-логика аутентификации
    team.go               — Бизнес-логика команд
    task.go               — Бизнес-логика задач
    interfaces.go         — Интерфейсы для тестирования
    errors.go             — Бизнес-ошибки
  store/
    user.go               — DAO пользователей (MySQL)
    team.go               — DAO команд (MySQL + Redis кеш)
    task.go               — DAO задач (MySQL)
    report.go             — SQL-запросы отчётов

docker-compose.yml       — Локальный стек (MySQL, Redis, phpMyAdmin)
config.example.yaml      — Шаблон конфигурации
```

## Установка и настройка

### Предварительные требования

| Компонент | Минимальная версия | Проверка |
|-----------|-------------------|----------|
| Go | 1.26+ | `go version` |
| Docker | 20.10+ | `docker --version` |
| Docker Compose | 2.0+ | `docker compose version` |
| Git | 2.30+ | `git --version` |

### Шаг 1: Клонирование репозитория

```bash
git clone https://github.com/turkprogrammer/taskflow
cd task_flow
```

### Шаг 2: Установка Go зависимостей

```bash
go mod download
```

### Шаг 3: Настройка конфигурации

```bash
cp config.example.yaml config.yaml
```

Отредактируйте `config.yaml` при необходимости:

```yaml
server:
  port: 8080          # Порт API сервера

db:
  host: localhost      # Адрес MySQL
  port: 3306           # Порт MySQL
  user: taskflow       # Пользователь MySQL
  password: taskflow   # Пароль MySQL
  name: taskflow       # Имя базы данных
  max_open_conns: 25   # Максимум открытых соединений
  max_idle_conns: 10   # Максимум idle соединений
  max_lifetime_seconds: 300  # Время жизни соединения (сек)

redis:
  addr: localhost:6379 # Адрес Redis
  password: ""         # Пароль Redis (пусто если нет)
  db: 0                # Номер БД Redis

jwt:
  secret: change-me-in-production  # Секрет для подписи JWT
  ttl_hours: 72                    # Время жизни токена (часы)
```

**Переменные окружения** перезаписывают значения из YAML:

| Переменная | Описание | Значение по умолчанию |
|------------|----------|----------------------|
| `DB_HOST` | Адрес MySQL | localhost |
| `DB_PORT` | Порт MySQL | 3306 |
| `DB_USER` | Пользователь MySQL | taskflow |
| `DB_PASSWORD` | Пароль MySQL | taskflow |
| `DB_NAME` | Имя базы данных | taskflow |
| `REDIS_ADDR` | Адрес Redis | localhost:6379 |
| `REDIS_PASSWORD` | Пароль Redis | (пусто) |
| `JWT_SECRET` | Секрет JWT | change-me-in-production |
| `SERVER_PORT` | Порт сервера | 8080 |

### Шаг 4: Запуск инфраструктуры

```bash
docker compose up -d
```

Это запустит:
- **MySQL 8** на порту `3306`
- **Redis 7** на порту `6379`
- **phpMyAdmin** на порту `8081` (для управления БД)

Проверка статуса контейнеров:

```bash
docker compose ps
```

### Шаг 5: Запуск приложения

```bash
go run ./cmd/api
```

### Шаг 6: Проверка работоспособности

```bash
# Health check
curl http://localhost:8080/health
# Ответ: {"status":"ok"}

# Регистрация пользователя
curl -X POST http://localhost:8080/api/v1/register \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"secret123","name":"Test User"}'

# Авторизация
curl -X POST http://localhost:8080/api/v1/login \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"secret123"}'
```

Пример запуска с переменными окружения:

```bash
DB_HOST=192.168.1.100 JWT_SECRET=my-secret go run ./cmd/api
```

## Лицензия

MIT
