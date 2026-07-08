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

	"github.com/redis/go-redis/v9"
)

// execer — абстракция над *sql.DB и *sql.Tx для выполнения запросов
// в контексте транзакций и без них.
type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// Team представляет команду (группу пользователей) в системе.
type Team struct {
	// ID — уникальный идентификатор команды.
	ID uint64 `json:"id"`
	// Name — название команды.
	Name string `json:"name"`
	// CreatedBy — ID пользователя-создателя команды (становится owner).
	CreatedBy uint64 `json:"created_by"`
	// CreatedAt — timestamp создания команды.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt — timestamp последнего обновления.
	UpdatedAt time.Time `json:"updated_at"`
}

// TeamMember представляет запись о членстве пользователя в команде.
type TeamMember struct {
	// ID — уникальный идентификатор записи о членстве.
	ID uint64 `json:"id"`
	// TeamID — ID команды.
	TeamID uint64 `json:"team_id"`
	// UserID — ID пользователя.
	UserID uint64 `json:"user_id"`
	// Role — роль в команде: "owner", "admin" или "member".
	Role string `json:"role"`
	// CreatedAt — timestamp добавления в команду.
	CreatedAt time.Time `json:"created_at"`
}

// TeamStore реализует CRUD-операции для команд через MySQL + Redis кеш.
type TeamStore struct {
	// db — подключение к MySQL.
	db *sql.DB
	// rdb — клиент Redis для кеширования списка команд.
	rdb *redis.Client
}

// NewTeamStore создаёт новый TeamStore с подключением к БД и Redis.
func NewTeamStore(db *sql.DB, rdb *redis.Client) *TeamStore {
	return &TeamStore{db: db, rdb: rdb}
}

// Create создаёт новую команду в БД. Устанавливает ID и timestamps.
func (s *TeamStore) Create(ctx context.Context, t *Team) error {
	return s.create(ctx, s.db, t)
}

// create — внутренний метод создания команды. Принимает execer для работы
// как с прямым запросом, так и внутри транзакции.
func (s *TeamStore) create(ctx context.Context, exec execer, t *Team) error {
	if t == nil {
		return errors.New("team is nil")
	}
	now := time.Now()
	t.CreatedAt = now
	t.UpdatedAt = now

	result, err := exec.ExecContext(ctx,
		"INSERT INTO teams (name, created_by) VALUES (?, ?)",
		t.Name, t.CreatedBy,
	)
	if err != nil {
		return fmt.Errorf("creating team: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("getting team last insert id: %w", err)
	}

	if id < 0 {
		return fmt.Errorf("negative team id: %d", id)
	}
	t.ID = uint64(id)
	return nil
}

// CreateWithOwner атомарно создаёт команду и добавляет создателя как owner.
// Использует транзакцию для保证 целостности данных.
func (s *TeamStore) CreateWithOwner(ctx context.Context, t *Team, ownerID uint64) error {
	// Начало транзакции
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() // откат при ошибке (безопасно если уже закоммичено)

	if err := s.create(ctx, tx, t); err != nil {
		return fmt.Errorf("creating team: %w", err)
	}
	if err := addMember(ctx, tx, t.ID, ownerID, "owner"); err != nil {
		return fmt.Errorf("adding owner: %w", err)
	}

	// Фиксация транзакции
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	// Инвалидация кеша после изменения данных
	s.invalidateCache(ctx, ownerID)
	return nil
}

// GetByID находит команду по уникальному идентификатору.
// Возвращает sql.ErrNoRows, если команда не найдена.
func (s *TeamStore) GetByID(ctx context.Context, id uint64) (*Team, error) {
	t := &Team{}
	err := s.db.QueryRowContext(ctx,
		"SELECT id, name, created_by, created_at, updated_at FROM teams WHERE id = ?",
		id,
	).Scan(&t.ID, &t.Name, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		return nil, fmt.Errorf("getting team by id: %w", err)
	}

	return t, nil
}

// ListByUserID возвращает все команды, в которых состоит пользователь.
// Результаты кешируются в Redis на 5 минут (cache-aside паттерн).
func (s *TeamStore) ListByUserID(ctx context.Context, userID uint64) ([]*Team, error) {
	// Попытка получить данные из кеша
	cached, err := s.getCached(ctx, userID)
	if err == nil && cached != nil {
		return cached, nil
	}

	// Запрос к БД при отсутствии в кеше
	rows, err := s.db.QueryContext(ctx,
		`SELECT t.id, t.name, t.created_by, t.created_at, t.updated_at
		 FROM teams t
		 JOIN team_members tm ON tm.team_id = t.id
		 WHERE tm.user_id = ?
		 ORDER BY t.created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("querying teams for user %d: %w", userID, err)
	}
	defer rows.Close()

	var teams []*Team
	for rows.Next() {
		t := &Team{}
		if err := rows.Scan(&t.ID, &t.Name, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning team row: %w", err)
		}
		teams = append(teams, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating team rows: %w", err)
	}

	// Сохранение результата в кеш (если данные есть)
	if teams != nil {
		s.setCache(ctx, userID, teams)
	}

	return teams, nil
}

// AddMember добавляет пользователя в команду с указанной ролью.
// Инвалидирует кеш списка команд пользователя.
func (s *TeamStore) AddMember(ctx context.Context, teamID, userID uint64, role string) error {
	if err := addMember(ctx, s.db, teamID, userID, role); err != nil {
		return err
	}
	s.invalidateCache(ctx, userID)
	return nil
}

// addMember — внутренний метод добавления участника. Работает с любым execer.
func addMember(ctx context.Context, exec execer, teamID, userID uint64, role string) error {
	_, err := exec.ExecContext(ctx,
		"INSERT INTO team_members (team_id, user_id, role) VALUES (?, ?, ?)",
		teamID, userID, role,
	)
	if err != nil {
		return fmt.Errorf("adding team member: %w", err)
	}
	return nil
}

// GetMemberRole возвращает роль пользователя в команде.
// Возвращает пустую строку и sql.ErrNoRows, если пользователь не является участником.
func (s *TeamStore) GetMemberRole(ctx context.Context, teamID, userID uint64) (string, error) {
	var role string
	err := s.db.QueryRowContext(ctx,
		"SELECT role FROM team_members WHERE team_id = ? AND user_id = ?",
		teamID, userID,
	).Scan(&role)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", err
		}
		return "", fmt.Errorf("getting member role: %w", err)
	}

	return role, nil
}

// GetMembers возвращает всех участников указанной команды.
func (s *TeamStore) GetMembers(ctx context.Context, teamID uint64) ([]*TeamMember, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, team_id, user_id, role, created_at FROM team_members WHERE team_id = ?",
		teamID,
	)
	if err != nil {
		return nil, fmt.Errorf("querying team members: %w", err)
	}
	defer rows.Close()

	var members []*TeamMember
	for rows.Next() {
		m := &TeamMember{}
		if err := rows.Scan(&m.ID, &m.TeamID, &m.UserID, &m.Role, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning team member row: %w", err)
		}
		members = append(members, m)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating team member rows: %w", err)
	}
	return members, nil
}

// getCached — получение списка команд пользователя из Redis кеша.
func (s *TeamStore) getCached(ctx context.Context, userID uint64) ([]*Team, error) {
	if s.rdb == nil {
		return nil, nil
	}
	data, err := s.rdb.Get(ctx, teamCacheKey(userID)).Bytes()
	if err != nil {
		return nil, fmt.Errorf("getting team cache: %w", err)
	}
	var teams []*Team
	if err := json.Unmarshal(data, &teams); err != nil {
		return nil, fmt.Errorf("unmarshaling team cache: %w", err)
	}
	return teams, nil
}

// setCache — сохранение списка команд пользователя в Redis с TTL 5 минут.
func (s *TeamStore) setCache(ctx context.Context, userID uint64, teams []*Team) {
	if s.rdb == nil {
		return
	}
	data, err := json.Marshal(teams)
	if err != nil {
		slog.Warn("failed to marshal teams for cache", "error", err)
		return
	}
	if err := s.rdb.Set(ctx, teamCacheKey(userID), data, 5*time.Minute).Err(); err != nil {
		slog.Warn("failed to set team cache", "error", err)
	}
}

// invalidateCache — удаление кеша списка команд пользователя из Redis.
func (s *TeamStore) invalidateCache(ctx context.Context, userID uint64) {
	if s.rdb == nil {
		return
	}
	if err := s.rdb.Del(ctx, teamCacheKey(userID)).Err(); err != nil {
		slog.Warn("failed to invalidate team cache", "error", err)
	}
}

// teamCacheKey — генерация ключа кеша для списка команд пользователя.
func teamCacheKey(userID uint64) string {
	return "teams:user:" + strconv.FormatUint(userID, 10)
}
