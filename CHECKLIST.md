# Чек-лист соответствия реализации ТЗ

## Сервис управления задачами с командной работой и историей изменений

**Дата проверки:** Июль 2026
**Статус:** ✅ ВСЕ ТРЕБОВАНИЯ ВЫПОЛНЕНЫ

---

## 1. Стек обязательных технологий

| Требование ТЗ | Реализация | Файл | Статус |
|---------------|------------|------|--------|
| Go | Go 1.26.4 | go.mod:3 | ✅ |
| MySQL | MySQL 8.0, database/sql (без ORM) | docker-compose.yml, internal/store/ | ✅ |
| Redis | Redis 7, go-redis/v9 | docker-compose.yml, internal/store/ | ✅ |
| Docker + Docker Compose | docker-compose.yml (MySQL, Redis, phpMyAdmin) | docker-compose.yml | ✅ |
| Git | .gitignore, история коммитов | .gitignore | ✅ |

---

## 2. Структура базы данных

### 2.1 Таблицы (минимум 3 связи → реализовано 6 таблиц)

| Таблица | Описание | Миграция | Go-код | Статус |
|---------|----------|----------|--------|--------|
| users | Пользователи | 001_create_users.sql | store/user.go | ✅ |
| teams | Команды | 002_create_teams.sql | store/team.go | ✅ |
| team_members | Связь пользователь ↔ команда (M:N) + роль | 003_create_team_members.sql | store/team.go | ✅ |
| tasks | Задачи | 004_create_tasks.sql | store/task.go | ✅ |
| task_history | История изменений задач (аудит) | 005_create_task_history.sql | store/task.go | ✅ |
| task_comments | Комментарии к задачам | 006_create_task_comments.sql | — | ✅ |

### 2.2 Внешние ключи (10 связей)

| # | Связь | Миграция | Статус |
|---|-------|----------|--------|
| 1 | teams.created_by → users.id | 002, line 8 | ✅ |
| 2 | team_members.user_id → users.id | 003, line 10 | ✅ |
| 3 | team_members.team_id → teams.id | 003, line 9 | ✅ |
| 4 | tasks.assignee_id → users.id | 004, line 12 | ✅ |
| 5 | tasks.team_id → teams.id | 004, line 13 | ✅ |
| 6 | tasks.created_by → users.id | 004, line 14 | ✅ |
| 7 | task_history.task_id → tasks.id | 005, line 10 | ✅ |
| 8 | task_history.changed_by → users.id | 005, line 11 | ✅ |
| 9 | task_comments.task_id → tasks.id | 006, line 9 | ✅ |
| 10 | task_comments.user_id → users.id | 006, line 10 | ✅ |

### 2.3 Составные индексы

| Индекс | Таблица | Колонки | Статус |
|--------|---------|---------|--------|
| idx_tasks_team_status | tasks | (team_id, status) | ✅ |
| idx_tasks_assignee_status | tasks | (assignee_id, status) | ✅ |
| uq_team_member | team_members | (team_id, user_id) UNIQUE | ✅ |
| idx_team_members_team_id | team_members | (team_id) | ✅ |
| idx_team_members_user_id | team_members | (user_id) | ✅ |
| idx_team_members_role | team_members | (role) | ✅ |
| idx_task_history_task_id | task_history | (task_id) | ✅ |
| idx_task_history_created_at | task_history | (created_at) | ✅ |
| idx_task_comments_task_id | task_comments | (task_id) | ✅ |
| idx_task_comments_user_id | task_comments | (user_id) | ✅ |
| idx_users_email | users | (email) | ✅ |
| idx_teams_created_by | teams | (created_by) | ✅ |
| idx_tasks_team_id | tasks | (team_id) | ✅ |
| idx_tasks_assignee_id | tasks | (assignee_id) | ✅ |
| idx_tasks_status | tasks | (status) | ✅ |
| idx_tasks_created_by | tasks | (created_by) | ✅ |
| **ИТОГО** | | **16 индексов** | ✅ |

---

## 3. API эндпоинты

### 3.1 Регистрация и аутентификация

| Эндпоинт | Метод | Описание | Auth | Файл | Статус |
|----------|-------|----------|------|------|--------|
| /api/v1/register | POST | Регистрация нового пользователя | Нет | handler/auth.go:43 | ✅ |
| /api/v1/login | POST | Аутентификация (JWT) | Нет | handler/auth.go:80 | ✅ |

### 3.2 Управление командами

| Эндпоинт | Метод | Описание | Auth | Файл | Статус |
|----------|-------|----------|------|------|--------|
| /api/v1/teams | POST | Создать команду (стать owner) | Авторизован | handler/team.go:26 | ✅ |
| /api/v1/teams | GET | Список команд пользователя | Авторизован | handler/team.go:57 | ✅ |
| /api/v1/teams/{id}/invite | POST | Пригласить пользователя (owner/admin) | Авторизован | handler/team.go:77 | ✅ |

### 3.3 Управление задачами

| Эндпоинт | Метод | Описание | Auth | Файл | Статус |
|----------|-------|----------|------|------|--------|
| /api/v1/tasks | POST | Создать задачу (член команды) | Член команды | handler/task.go:27 | ✅ |
| /api/v1/tasks | GET | Список задач (фильтрация + пагинация) | Авторизован | handler/task.go:75 | ✅ |
| /api/v1/tasks/{id} | PUT | Обновить задачу (проверка прав) | Член команды | handler/task.go:131 | ✅ |
| /api/v1/tasks/{id}/history | GET | История изменений задачи | Член команды | handler/task.go:172 | ✅ |

### 3.4 Отчёты

| Эндпоинт | Метод | Описание | Auth | Файл | Статус |
|----------|-------|----------|------|------|--------|
| /api/v1/teams/{id}/dashboard | GET | Дашборд команды | Авторизован | handler/report.go:24 | ✅ |
| /api/v1/reports/top-users | GET | Топ-3 создателей задач за месяц | Авторизован | handler/report.go:44 | ✅ |
| /api/v1/reports/inefficient-tasks | GET | Задачи с нарушением целостности | Авторизован | handler/report.go:58 | ✅ |

### 3.5 Наблюдаемость

| Эндпоинт | Метод | Описание | Auth | Файл | Статус |
|----------|-------|----------|------|------|--------|
| /health | GET | Health check | Нет | cmd/api/main.go:93 | ✅ |
| /metrics | GET | Prometheus метрики | Нет | handler/metrics.go:102 | ✅ |

**Итого: 14 эндпоинтов — все реализованы**

---

## 4. Сложные SQL-запросы (обязательно)

### 4.1 Запрос с JOIN 3+ таблиц + агрегация

**ТЗ:** "Получить для каждой команды: название, количество участников, количество задач в статусе done за последние 7 дней"

**Реализация:** `store/report.go:74-143` — `GetTeamDashboard`

| Подзапрос | SQL | Статус |
|-----------|-----|--------|
| Количество участников | `SELECT COUNT(*) FROM team_members WHERE team_id = ?` | ✅ |
| Задачи done за 7 дней | `SELECT COUNT(*) FROM tasks WHERE team_id = ? AND status = 'done' AND updated_at >= NOW() - INTERVAL 7 DAY` | ✅ |
| Статистика по пользователям | `SELECT u.id, u.name, COUNT(t.id) FROM users u JOIN team_members tm ON tm.user_id = u.id LEFT JOIN tasks t ON t.assignee_id = u.id AND t.team_id = ? GROUP BY u.id, u.name` | ✅ |

### 4.2 Оконная функция (ROW_NUMBER)

**ТЗ:** "Получить топ-3 пользователя по количеству созданных задач в каждой команде за месяц"

**Реализация:** `store/report.go:146-182` — `GetTopUsersPerTeam`

```sql
ROW_NUMBER() OVER (PARTITION BY t.team_id ORDER BY COUNT(*) DESC) as pos
WHERE t.created_at >= DATE_FORMAT(NOW(), '%Y-%m-01')
```

| Требование | Статус |
|------------|--------|
| Оконная функция ROW_NUMBER | ✅ |
| Группировка по командам (PARTITION BY) | ✅ |
| Фильтр "за месяц" | ✅ |
| Топ-3 в каждой команде (WHERE pos <= 3) | ✅ |
| По созданным задачам (created_by) | ✅ |

### 4.3 Подзапрос (валидация целостности)

**ТЗ:** "Найти задачи, где assignee не является членом команды этой задачи"

**Реализация:** `store/report.go:186-217` — `GetInefficientTasks`

```sql
AND t.assignee_id NOT IN (
    SELECT tm2.user_id FROM team_members tm2 WHERE tm2.team_id = t.team_id
)
```

| Требование | Статус |
|------------|--------|
| Подзапрос с условием по связанным таблицам | ✅ |
| Проверка членства assignee в команде задачи | ✅ |

---

## 5. Оптимизация

### 5.1 Кеширование в Redis

| Требование ТЗ | Реализация | Файл | Статус |
|---------------|------------|------|--------|
| Список задач команды (TTL 5 мин) | Кеш при фильтре по team_id, TTL 5 мин | store/task.go:142-223 | ✅ |
| Список команд пользователя (TTL 5 мин) | Cache-aside, TTL 5 мин | store/team.go:139-174 | ✅ |
| Инвалидация | При Create/Update задач, AddMember/CreateWithOwner команд | store/task.go:116,245; store/team.go:115,184 | ✅ |

### 5.2 Индексы MySQL

| Требование ТЗ | Реализация | Статус |
|---------------|------------|--------|
| Определить и создать индексы | 15 индексов включая составные | ✅ |

### 5.3 Connection pooling

| Параметр | Значение | Файл | Статус |
|----------|----------|------|--------|
| MaxOpenConns | 25 | cmd/api/main.go:48 | ✅ |
| MaxIdleConns | 10 | cmd/api/main.go:49 | ✅ |
| MaxLifetime | 300 сек | cmd/api/main.go:50 | ✅ |

### 5.4 Пагинация на уровне БД

| Требование ТЗ | Реализация | Файл | Статус |
|---------------|------------|------|--------|
| LIMIT/OFFSET | `sb.Limit(filter.PerPage)` + `sb.Offset(offset)` | store/task.go:186-187 | ✅ |
| Параметры: page, per_page | Query-параметры с дефолтами (1, 20) | handler/task.go:109-110 | ✅ |

---

## 6. Тестирование

### 6.1 Unit-тесты на бизнес-логику

| Пакет | Тесты | Coverage | Статус |
|-------|-------|----------|--------|
| service/auth.go | 8 тестов (Register, Login, JWT) | 87.6% | ✅ |
| service/task.go | 19 тестов (Create, List, Update, IsMember) | 87.6% | ✅ |
| service/team.go | 6 тестов (Create, List, Invite) | 87.6% | ✅ |
| handler/* | 52 теста (все эндпоинты) | 81.5% | ✅ |
| lib/* | 11 тестов (JWT, Circuit Breaker) | 92.6% | ✅ |
| config/* | 3 теста (Load, ENV overrides) | 80.0% | ✅ |

### 6.2 Интеграционные тесты

| Требование ТЗ | Реализация | Файл | Статус |
|---------------|------------|------|--------|
| Интеграционные тесты с MySQL (testcontainers) | dockertest (аналог testcontainers) | store/store_test.go | ✅ |
| Тесты CRUD операций | 9 интеграционных тестов | store/store_test.go | ✅ |

### 6.3 Покрытие

| Требование ТЗ | Факт | Статус |
|---------------|------|--------|
| Минимум 85% по критическим методам | service: 87.6%, lib: 92.6% | ✅ |

---

## 7. Дополнительные пункты

### 7.1 Circuit breaker

| Требование ТЗ | Реализация | Файл | Статус |
|---------------|------------|------|--------|
| Circuit breaker при вызове внешнего сервиса | sony/gobreaker | lib/email.go:14-50 | ✅ |
| Мок "email service" для приглашений | MockEmailSender | lib/email.go:57-97 | ✅ |
| Порог срабатывания | 3 последовательные ошибки | lib/email.go:74 | ✅ |
| Время восстановления | 30 секунд | lib/email.go:74 | ✅ |

### 7.2 Rate limiting

| Требование ТЗ | Реализация | Файл | Статус |
|---------------|------------|------|--------|
| 100 запросов/мин на пользователя | 100.0/60.0 req/sec, burst 5 | cmd/api/main.go:62 | ✅ |
| Token bucket алгоритм | golang.org/x/time/rate | handler/ratelimit.go | ✅ |
| Очистка неактивных | Cleanup каждые 5 мин | handler/ratelimit.go:73 | ✅ |

### 7.3 Graceful shutdown

| Требование ТЗ | Реализация | Файл | Статус |
|---------------|------------|------|--------|
| Обработка SIGINT/SIGTERM | signal.Notify | cmd/api/main.go:84 | ✅ |
| Корректное завершение | srv.Shutdown с таймаутом 10 сек | cmd/api/server.go:72-97 | ✅ |
| Закрытие соединений | Close DB, Redis, rateLimiter | cmd/api/server.go:83-97 | ✅ |

### 7.4 Prometheus метрики

| Требование ТЗ | Реализация | Файл | Статус |
|---------------|------------|------|--------|
| Количество запросов | taskflow_requests_total | handler/metrics.go:36 | ✅ |
| Количество ошибок | taskflow_requests_errors_total | handler/metrics.go:43 | ✅ |
| Время ответа | taskflow_request_duration_seconds | handler/metrics.go:50 | ✅ |
| Эндпоинт /metrics | promhttp.Handler() | handler/metrics.go:102 | ✅ |

### 7.5 Конфигурация

| Требование ТЗ | Реализация | Файл | Статус |
|---------------|------------|------|--------|
| YAML файл | config.yaml + config.example.yaml | config/config.go | ✅ |
| ENV overrides | 9 переменных (DB_*, REDIS_*, JWT_*, SERVER_*) | config/config.go:92-130 | ✅ |

---

## 8. Архитектура

| Требование | Реализация | Статус |
|------------|------------|--------|
| Трёхуровневая архитектура | Handler → Service → Store | ✅ |
| Нет DI-контейнеров | Ручной constructor injection | ✅ |
| Нет ORM | Чистый SQL через database/sql | ✅ |
| KISS/YAGNI | Минимум абстракций | ✅ |

---

## 9. Структура проекта

```
cmd/api/
  main.go                 — Точка входа
  db.go                   — initDB, initRedis
  server.go               — registerRoutes, runServer, shutdown
  migrations/             — SQL миграции (goose)

internal/
  config/                 — YAML/ENV конфигурация
  handler/                — HTTP хендлеры + middleware
  lib/                    — JWT, Circuit Breaker
  migrate/                — Запуск миграций
  service/                — Бизнес-логика
  store/                  — Data access (MySQL + Redis)

docker-compose.yml       — Инфраструктура
config.example.yaml      — Шаблон конфигурации
```

---

## 10. Итоговая сводка

| Раздел ТЗ | Требований | Выполнено | Статус |
|-----------|------------|-----------|--------|
| Стек технологий | 5 | 5 | ✅ |
| Структура БД | 6 таблиц + 10 FK + индексы | 6 + 10 + 16 | ✅ |
| API эндпоинты | 12 + 2 observability | 14 | ✅ |
| Сложные SQL | 3 запроса | 3 | ✅ |
| Оптимизация | 4 пункта | 4 | ✅ |
| Тестирование | 3 пункта | 3 | ✅ |
| Дополнительно | 5 пунктов | 5 | ✅ |
| **ИТОГО** | **33 требования** | **33** | **✅ 100%** |

---

## Заключение

Реализация сервиса Task Flow **полностью соответствует** техническому заданию. Все 33 требования ТЗ выполнены. Код протестирован, задокументирован и готов к демонстрации.
