package service

import (
	"context"

	"github.com/turkprogrammer/taskflow/internal/store"
)

// UserStore — контракт доступа к данным пользователей.
// Реализации: SQL-хранилище и in-memory моки для тестов.
type UserStore interface {
	// Create сохраняет нового пользователя. ID устанавливается хранилищем.
	Create(ctx context.Context, u *store.User) error
	// GetByEmail ищет пользователя по email. Возвращает store.ErrNotFound при отсутствии.
	GetByEmail(ctx context.Context, email string) (*store.User, error)
	// GetByID ищет пользователя по первичному ключу. Возвращает store.ErrNotFound при отсутствии.
	GetByID(ctx context.Context, id uint64) (*store.User, error)
}

// TeamStore — контракт доступа к данным команд и их участников.
type TeamStore interface {
	// Create сохраняет новую команду и устанавливает её ID.
	Create(ctx context.Context, t *store.Team) error
	// CreateWithOwner атомарно создаёт команду и добавляет создателя как owner.
	CreateWithOwner(ctx context.Context, t *store.Team, ownerID uint64) error
	// GetByID загружает команду по первичному ключу. Возвращает store.ErrNotFound при отсутствии.
	GetByID(ctx context.Context, id uint64) (*store.Team, error)
	// ListByUserID возвращает все команды пользователя.
	ListByUserID(ctx context.Context, userID uint64) ([]*store.Team, error)
	// AddMember добавляет пользователя в команду с указанной ролью (owner, admin, member).
	AddMember(ctx context.Context, teamID, userID uint64, role string) error
	// GetMemberRole возвращает роль пользователя в команде.
	// Возвращает store.ErrNotFound, если пользователь не является участником.
	GetMemberRole(ctx context.Context, teamID, userID uint64) (string, error)
	// GetMembers возвращает всех участников команды.
	GetMembers(ctx context.Context, teamID uint64) ([]*store.TeamMember, error)
}

// TaskStore — контракт доступа к данным задач и истории изменений.
type TaskStore interface {
	// Create сохраняет новую задачу и устанавливает её ID.
	Create(ctx context.Context, t *store.Task) error
	// GetByID загружает задачу по первичному ключу. Возвращает store.ErrNotFound при отсутствии.
	GetByID(ctx context.Context, id uint64) (*store.Task, error)
	// List возвращает страницу задач по фильтру и общее количество записей.
	List(ctx context.Context, filter store.TaskFilter) ([]*store.Task, int, error)
	// Update применяет изменения к уже сохранённой задаче.
	Update(ctx context.Context, t *store.Task) error
	// AppendHistory записывает изменение одного поля задачи в журнал аудита.
	AppendHistory(ctx context.Context, taskID, changedBy uint64, field, oldVal, newVal string) error
	// GetHistory возвращает полную историю изменений задачи (новые записи первыми).
	GetHistory(ctx context.Context, taskID uint64) ([]*store.TaskHistory, error)
	// IsMember проверяет, является ли пользователь участником команды.
	IsMember(ctx context.Context, teamID, userID uint64) (bool, error)
}

// ReportStore — контракт доступа к аналитическим запросам.
type ReportStore interface {
	// GetTeamDashboard возвращает агрегированные данные дашборда команды.
	GetTeamDashboard(ctx context.Context, teamID uint64) (*store.TeamDashboard, error)
	// GetTopUsersPerTeam возвращает топ пользователей по количеству задач.
	GetTopUsersPerTeam(ctx context.Context) ([]store.TopUser, error)
	// GetInefficientTasks возвращает задачи с нарушением целостности данных.
	GetInefficientTasks(ctx context.Context) ([]store.InefficientTask, error)
}
