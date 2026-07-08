package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/huandu/go-sqlbuilder"
	"github.com/redis/go-redis/v9"
)

// Task представляет задачу внутри команды.
type Task struct {
	// ID — уникальный идентификатор задачи.
	ID uint64 `json:"id"`
	// Title — краткое описание задачи (обязательное поле).
	Title string `json:"title"`
	// Description — подробное описание задачи (опциональное).
	Description *string `json:"description"`
	// Status — текущий статус: "todo", "in_progress" или "done".
	Status string `json:"status"`
	// AssigneeID — ID исполнителя (может быть nil, если задача не назначена).
	AssigneeID *uint64 `json:"assignee_id"`
	// TeamID — ID команды, которой принадлежит задача.
	TeamID uint64 `json:"team_id"`
	// CreatedBy — ID пользователя, создавшего задачу.
	CreatedBy uint64 `json:"created_by"`
	// CreatedAt — timestamp создания задачи.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt — timestamp последнего обновления задачи.
	UpdatedAt time.Time `json:"updated_at"`
}

// TaskHistory представляет запись аудита об изменении поля задачи.
type TaskHistory struct {
	// ID — уникальный идентификатор записи истории.
	ID uint64 `json:"id"`
	// TaskID — ID задачи, к которой относится изменение.
	TaskID uint64 `json:"task_id"`
	// FieldChanged — название изменённого поля (например, "status", "title").
	FieldChanged string `json:"field_changed"`
	// OldValue — предыдущее значение поля (может быть nil).
	OldValue *string `json:"old_value"`
	// NewValue — новое значение поля (может быть nil).
	NewValue *string `json:"new_value"`
	// ChangedBy — ID пользователя, выполнившего изменение.
	ChangedBy uint64 `json:"changed_by"`
	// CreatedAt — timestamp записи изменения.
	CreatedAt time.Time `json:"created_at"`
}

// TaskFilter содержит параметры фильтрации и пагинации для списка задач.
type TaskFilter struct {
	// TeamID — фильтр по ID команды (nil = без фильтра).
	TeamID *uint64
	// Status — фильтр по статусу (nil = все статусы).
	Status *string
	// AssigneeID — фильтр по исполнителю (nil = все исполнители).
	AssigneeID *uint64
	// Page — номер страницы (начинается с 1, по умолчанию 1).
	Page int
	// PerPage — количество элементов на странице (по умолчанию 20).
	PerPage int
}

// TaskStore реализует CRUD-операции для задач через MySQL + Redis кеш.
type TaskStore struct {
	// db — подключение к MySQL.
	db *sql.DB
	// rdb — клиент Redis для кеширования списков задач.
	rdb *redis.Client
}

// NewTaskStore создаёт новый TaskStore с подключением к БД и Redis.
func NewTaskStore(db *sql.DB, rdb *redis.Client) *TaskStore {
	return &TaskStore{db: db, rdb: rdb}
}

// Create создаёт новую задачу в БД.
// Устанавливает ID, CreatedAt, UpdatedAt. Инвалидирует кеш задач команды.
func (s *TaskStore) Create(ctx context.Context, t *Task) error {
	if t == nil {
		return errors.New("task is nil")
	}
	now := time.Now()
	t.CreatedAt = now
	t.UpdatedAt = now

	// Вставка записи в таблицу tasks
	result, err := s.db.ExecContext(ctx,
		"INSERT INTO tasks (title, description, status, assignee_id, team_id, created_by) VALUES (?, ?, ?, ?, ?, ?)",
		t.Title, t.Description, t.Status, t.AssigneeID, t.TeamID, t.CreatedBy,
	)
	if err != nil {
		return fmt.Errorf("creating task: %w", err)
	}

	// Получение сгенерированного ID
	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("getting task last insert id: %w", err)
	}

	if id < 0 {
		return fmt.Errorf("negative task id: %d", id)
	}
	t.ID = uint64(id)

	// Инвалидация кеша задач команды после создания
	if s.rdb != nil {
		s.invalidateTeamTasksCache(ctx, t.TeamID)
	}

	return nil
}

// GetByID находит задачу по уникальному идентификатору.
// Возвращает sql.ErrNoRows, если задача не найдена.
func (s *TaskStore) GetByID(ctx context.Context, id uint64) (*Task, error) {
	t := &Task{}
	err := s.db.QueryRowContext(ctx,
		"SELECT id, title, description, status, assignee_id, team_id, created_by, created_at, updated_at FROM tasks WHERE id = ?",
		id,
	).Scan(&t.ID, &t.Title, &t.Description, &t.Status, &t.AssigneeID, &t.TeamID, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		return nil, fmt.Errorf("getting task by id: %w", err)
	}

	return t, nil
}

// List возвращает страницу задач, соответствующих фильтру, и общее количество.
// Фильтрует по TeamID, Status, AssigneeID (если не nil).
// При фильтрации только по team_id результаты кешируются в Redis на 5 минут.
func (s *TaskStore) List(ctx context.Context, filter TaskFilter) ([]*Task, int, error) {
	// Определяем, можно ли кешировать результат (только фильтр по team_id)
	cacheable := s.rdb != nil && filter.TeamID != nil && filter.Status == nil && filter.AssigneeID == nil
	if cacheable {
		if filter.PerPage <= 0 {
			filter.PerPage = 20
		}
		if filter.Page <= 0 {
			filter.Page = 1
		}
		// Попытка получить из кеша
		if cached, total, err := s.getCachedTeamTasks(ctx, *filter.TeamID, filter.Page, filter.PerPage); err == nil && cached != nil {
			return cached, total, nil
		}
	}

	// Построение динамического SQL-запроса
	sb := sqlbuilder.NewSelectBuilder()
	sb.From("tasks")

	// Добавление условий фильтрации
	if filter.TeamID != nil {
		sb.Where(sb.Equal("team_id", *filter.TeamID))
	}
	if filter.Status != nil {
		sb.Where(sb.Equal("status", *filter.Status))
	}
	if filter.AssigneeID != nil {
		sb.Where(sb.Equal("assignee_id", *filter.AssigneeID))
	}

	// Подсчёт общего количества записей (для пагинации)
	countBuilder := sb.Clone()
	countBuilder.Select("COUNT(*)")
	countSQL, countArgs := countBuilder.Build()
	var total int
	err := s.db.QueryRowContext(ctx, countSQL, countArgs...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("counting tasks: %w", err)
	}

	// Установка значений по умолчанию для пагинации
	if filter.PerPage <= 0 {
		filter.PerPage = 20
	}
	if filter.Page <= 0 {
		filter.Page = 1
	}
	offset := (filter.Page - 1) * filter.PerPage

	// Построение запроса данных с сортировкой и лимитом
	sb.Select("id", "title", "description", "status", "assignee_id", "team_id", "created_by", "created_at", "updated_at")
	sb.OrderBy("created_at DESC")
	sb.Limit(filter.PerPage)
	sb.Offset(offset)
	dataSQL, dataArgs := sb.Build()

	// Выполнение запроса данных
	rows, err := s.db.QueryContext(ctx, dataSQL, dataArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("querying tasks: %w", err)
	}
	defer rows.Close()

	// Сканирование результатов в структуры
	var tasks []*Task
	for rows.Next() {
		t := &Task{}
		if err := rows.Scan(&t.ID, &t.Title, &t.Description, &t.Status, &t.AssigneeID, &t.TeamID, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scanning task row: %w", err)
		}
		tasks = append(tasks, t)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterating task rows: %w", err)
	}

	// Сохранение в кеш (если кешируется)
	if cacheable && filter.TeamID != nil {
		s.setCachedTeamTasks(ctx, *filter.TeamID, filter.Page, filter.PerPage, tasks, total)
	}

	return tasks, total, nil
}

// Update обновляет существующую задачу в БД.
// Автоматически устанавливает UpdatedAt. Инвалидирует кеш задач команды.
func (s *TaskStore) Update(ctx context.Context, t *Task) error {
	t.UpdatedAt = time.Now()

	// Обновление записи в таблице tasks
	_, err := s.db.ExecContext(ctx,
		"UPDATE tasks SET title = ?, description = ?, status = ?, assignee_id = ?, updated_at = ? WHERE id = ?",
		t.Title, t.Description, t.Status, t.AssigneeID, t.UpdatedAt, t.ID,
	)
	if err != nil {
		return fmt.Errorf("updating task %d: %w", t.ID, err)
	}

	// Инвалидация кеша задач команды после обновления
	if s.rdb != nil {
		s.invalidateTeamTasksCache(ctx, t.TeamID)
	}

	return nil
}

// GetHistory возвращает всю историю изменений задачи, отсортированную по убыванию даты.
func (s *TaskStore) GetHistory(ctx context.Context, taskID uint64) ([]*TaskHistory, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, task_id, field_changed, old_value, new_value, changed_by, created_at FROM task_history WHERE task_id = ? ORDER BY created_at DESC",
		taskID,
	)
	if err != nil {
		return nil, fmt.Errorf("querying task history: %w", err)
	}
	defer rows.Close()

	var history []*TaskHistory
	for rows.Next() {
		h := &TaskHistory{}
		if err := rows.Scan(&h.ID, &h.TaskID, &h.FieldChanged, &h.OldValue, &h.NewValue, &h.ChangedBy, &h.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning task history row: %w", err)
		}
		history = append(history, h)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating task history rows: %w", err)
	}
	return history, nil
}

// AppendHistory записывает изменение поля задачи в журнал аудита.
func (s *TaskStore) AppendHistory(ctx context.Context, taskID, changedBy uint64, field, oldVal, newVal string) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO task_history (task_id, field_changed, old_value, new_value, changed_by) VALUES (?, ?, ?, ?, ?)",
		taskID, field, oldVal, newVal, changedBy,
	)
	if err != nil {
		return fmt.Errorf("appending task history: %w", err)
	}
	return nil
}

// IsMember проверяет, является ли пользователь участником команды.
// Возвращает true, если пользователь состоит в команде.
func (s *TaskStore) IsMember(ctx context.Context, teamID, userID uint64) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM team_members WHERE team_id = ? AND user_id = ?",
		teamID, userID,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("checking team membership: %w", err)
	}
	return count > 0, nil
}

// taskCacheEntry — структура для сериализации в Redis (задачи + общее количество).
type taskCacheEntry struct {
	Tasks []*Task `json:"tasks"`
	Total int     `json:"total"`
}

// taskCacheKey — генерация ключа кеша для списка задач команды.
func taskCacheKey(teamID uint64, page, perPage int) string {
	return "tasks:team:" + strconv.FormatUint(teamID, 10) + ":" + strconv.Itoa(page) + ":" + strconv.Itoa(perPage)
}

// getCachedTeamTasks — получение списка задач команды из Redis кеша.
func (s *TaskStore) getCachedTeamTasks(ctx context.Context, teamID uint64, page, perPage int) ([]*Task, int, error) {
	data, err := s.rdb.Get(ctx, taskCacheKey(teamID, page, perPage)).Bytes()
	if err != nil {
		return nil, 0, fmt.Errorf("getting task cache: %w", err)
	}
	var entry taskCacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, 0, fmt.Errorf("unmarshaling task cache: %w", err)
	}
	return entry.Tasks, entry.Total, nil
}

// setCachedTeamTasks — сохранение списка задач команды в Redis с TTL 5 минут.
func (s *TaskStore) setCachedTeamTasks(ctx context.Context, teamID uint64, page, perPage int, tasks []*Task, total int) {
	data, err := json.Marshal(taskCacheEntry{Tasks: tasks, Total: total})
	if err != nil {
		slog.Warn("failed to marshal tasks for cache", "error", err)
		return
	}
	if err := s.rdb.Set(ctx, taskCacheKey(teamID, page, perPage), data, 5*time.Minute).Err(); err != nil {
		slog.Warn("failed to set task cache", "error", err)
	}
}

// invalidateTeamTasksCache — удаление всех ключей кеша задач команды из Redis.
func (s *TaskStore) invalidateTeamTasksCache(ctx context.Context, teamID uint64) {
	// SCAN для поиска всех ключей по паттерну tasks:team:{id}:*
	pattern := "tasks:team:" + strconv.FormatUint(teamID, 10) + ":*"
	iter := s.rdb.Scan(ctx, 0, pattern, 100).Iterator()
	for iter.Next(ctx) {
		s.rdb.Del(ctx, iter.Val())
	}
	if err := iter.Err(); err != nil {
		slog.Warn("failed to invalidate task cache", "error", err)
	}
}
