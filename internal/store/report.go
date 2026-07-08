package store

import (
	"context"
	"database/sql"
	"fmt"
)

// TeamDashboard содержит агрегированные данные дашборда для одной команды.
type TeamDashboard struct {
	// StatusCounts — количество задач по каждому статусу (todo, in_progress, done).
	StatusCounts map[string]int `json:"status_counts"`
	// DoneLast7Days — количество задач со статусом "done" за последние 7 дней.
	DoneLast7Days int `json:"done_last_7_days"`
	// MemberCount — количество участников команды.
	MemberCount int `json:"member_count"`
	// UserStats — статистика задач по каждому участнику команды.
	UserStats []UserTaskStat `json:"user_stats"`
}

// UserTaskStat содержит количество задач, назначенных на конкретного пользователя.
type UserTaskStat struct {
	// UserID — уникальный идентификатор пользователя.
	UserID uint64 `json:"user_id"`
	// UserName — отображаемое имя пользователя.
	UserName string `json:"user_name"`
	// TaskCount — количество задач, назначенных на пользователя.
	TaskCount int `json:"task_count"`
}

// TopUser представляет ранжированного пользователя в команде по количеству задач.
type TopUser struct {
	// TeamID — ID команды.
	TeamID uint64 `json:"team_id"`
	// UserID — ID пользователя.
	UserID uint64 `json:"user_id"`
	// UserName — отображаемое имя пользователя.
	UserName string `json:"user_name"`
	// TaskCount — количество созданных задач за месяц.
	TaskCount int `json:"task_count"`
	// Position — позиция в рейтинге (1 = первый).
	Position int `json:"position"`
}

// InefficientTask представляет задачу, назначенную на пользователя, который не является участником команды.
// Используется для валидации целостности данных.
type InefficientTask struct {
	// ID — уникальный идентификатор задачи.
	ID uint64 `json:"id"`
	// Title — название задачи.
	Title string `json:"title"`
	// AssigneeID — ID назначенного исполнителя.
	AssigneeID uint64 `json:"assignee_id"`
	// AssigneeName — имя назначенного исполнителя.
	AssigneeName string `json:"assignee_name"`
	// TeamID — ID команды, которой принадлежит задача.
	TeamID uint64 `json:"team_id"`
	// TeamName — название команды.
	TeamName string `json:"team_name"`
	// TaskRank — порядковый номер задачи в команде (начиная с 1).
	TaskRank int `json:"task_rank"`
}

// ReportStore реализует аналитические запросы через MySQL.
type ReportStore struct {
	// db — подключение к MySQL.
	db *sql.DB
}

// NewReportStore создаёт новый ReportStore с подключением к БД.
func NewReportStore(db *sql.DB) *ReportStore {
	return &ReportStore{db: db}
}

// GetTeamDashboard возвращает агрегированные данные дашборда команды:
// количество задач по статусам, задач done за 7 дней, количество участников
// и статистику по каждому пользователю.
// Запрос: JOIN 3+ таблиц (users, team_members, tasks) + агрегация.
func (s *ReportStore) GetTeamDashboard(ctx context.Context, teamID uint64) (*TeamDashboard, error) {
	d := &TeamDashboard{StatusCounts: make(map[string]int)}

	// 1. Подсчёт задач по каждому статусу
	rows, err := s.db.QueryContext(ctx,
		"SELECT status, COUNT(*) as cnt FROM tasks WHERE team_id = ? GROUP BY status",
		teamID,
	)
	if err != nil {
		return nil, fmt.Errorf("querying status counts: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("scanning status count row: %w", err)
		}
		d.StatusCounts[status] = count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating status count rows: %w", err)
	}

	// 2. Подсчёт задач со статусом "done" за последние 7 дней
	err = s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM tasks WHERE team_id = ? AND status = 'done' AND updated_at >= NOW() - INTERVAL 7 DAY",
		teamID,
	).Scan(&d.DoneLast7Days)
	if err != nil {
		return nil, fmt.Errorf("counting done tasks in last 7 days: %w", err)
	}

	// 3. Подсчёт количества участников команды
	err = s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM team_members WHERE team_id = ?",
		teamID,
	).Scan(&d.MemberCount)
	if err != nil {
		return nil, fmt.Errorf("counting team members: %w", err)
	}

	// 4. Статистика задач по каждому участнику (JOIN 3 таблицы)
	userRows, err := s.db.QueryContext(ctx,
		`SELECT u.id, u.name, COUNT(t.id) as task_count
		 FROM users u
		 JOIN team_members tm ON tm.user_id = u.id
		 LEFT JOIN tasks t ON t.assignee_id = u.id AND t.team_id = ?
		 WHERE tm.team_id = ?
		 GROUP BY u.id, u.name
		 ORDER BY task_count DESC`,
		teamID, teamID,
	)
	if err != nil {
		return nil, fmt.Errorf("querying user stats: %w", err)
	}
	defer userRows.Close()

	for userRows.Next() {
		var stat UserTaskStat
		if err := userRows.Scan(&stat.UserID, &stat.UserName, &stat.TaskCount); err != nil {
			return nil, fmt.Errorf("scanning user stat row: %w", err)
		}
		d.UserStats = append(d.UserStats, stat)
	}

	if err := userRows.Err(); err != nil {
		return nil, fmt.Errorf("iterating user stat rows: %w", err)
	}
	return d, nil
}

// GetTopUsersPerTeam возвращает топ-3 создателей задач в каждой команде за текущий месяц.
// Использует оконную функцию ROW_NUMBER() для ранжирования.
func (s *ReportStore) GetTopUsersPerTeam(ctx context.Context) ([]TopUser, error) {
	// Подзапрос с оконной функцией для определения позиции каждого пользователя
	rows, err := s.db.QueryContext(ctx,
		`SELECT team_id, user_id, user_name, task_count, pos
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
		 ORDER BY team_id, pos`,
	)
	if err != nil {
		return nil, fmt.Errorf("querying top users per team: %w", err)
	}
	defer rows.Close()

	var result []TopUser
	for rows.Next() {
		var u TopUser
		if err := rows.Scan(&u.TeamID, &u.UserID, &u.UserName, &u.TaskCount, &u.Position); err != nil {
			return nil, fmt.Errorf("scanning top user row: %w", err)
		}
		result = append(result, u)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating top user rows: %w", err)
	}
	return result, nil
}

// GetInefficientTasks возвращает задачи, где исполнитель не является участником команды.
// Запрос: подзапрос с условием по связанным таблицам (валидация целостности).
func (s *ReportStore) GetInefficientTasks(ctx context.Context) ([]InefficientTask, error) {
	// Подзапрос: найти пользователей, которые НЕ состоят в команде задачи
	rows, err := s.db.QueryContext(ctx,
		`SELECT t.id, t.title, t.assignee_id, u.name, t.team_id, tm.name,
				ROW_NUMBER() OVER (PARTITION BY t.team_id ORDER BY t.created_at) as task_rank
		 FROM tasks t
		 JOIN users u ON u.id = t.assignee_id
		 JOIN teams tm ON tm.id = t.team_id
		 WHERE t.assignee_id IS NOT NULL
		   AND t.assignee_id NOT IN (
			   SELECT tm2.user_id FROM team_members tm2 WHERE tm2.team_id = t.team_id
		   )
		 ORDER BY t.team_id, t.assignee_id, t.created_at`,
	)
	if err != nil {
		return nil, fmt.Errorf("querying inefficient tasks: %w", err)
	}
	defer rows.Close()

	var result []InefficientTask
	for rows.Next() {
		var it InefficientTask
		if err := rows.Scan(&it.ID, &it.Title, &it.AssigneeID, &it.AssigneeName, &it.TeamID, &it.TeamName, &it.TaskRank); err != nil {
			return nil, fmt.Errorf("scanning inefficient task row: %w", err)
		}
		result = append(result, it)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating inefficient task rows: %w", err)
	}
	return result, nil
}
