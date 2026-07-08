package service

import (
	"context"
	"fmt"

	"github.com/turkprogrammer/taskflow/internal/store"
)

// TaskService обрабатывает CRUD-операции для задач в команде.
// Проверяет членство в команде перед каждым изменением и записывает
// историю изменений по полям для аудита.
type TaskService struct {
	tasks TaskStore // интерфейс доступа к данным задач
}

// NewTaskService создаёт новый TaskService с переданным store.
func NewTaskService(tasks TaskStore) *TaskService {
	return &TaskService{tasks: tasks}
}

// CreateTaskInput — входные данные для создания задачи.
// Вызывающий должен быть участником команды TeamID.
type CreateTaskInput struct {
	// Title — краткое описание задачи (обязательное).
	Title string
	// Description — подробное описание (может быть nil).
	Description *string
	// AssigneeID — ID исполнителя (может быть nil, если задача не назначена).
	AssigneeID *uint64
	// TeamID — ID команды, которой принадлежит задача (обязательное).
	TeamID uint64
	// CreatedBy — ID аутентифицированного пользователя (устанавливается хендлером).
	CreatedBy uint64
}

// Create добавляет новую задачу в команду после проверки членства.
// Новые задачи всегда создаются со статусом "todo".
func (s *TaskService) Create(ctx context.Context, input CreateTaskInput) (*store.Task, error) {
	// Проверка членства в команде
	member, err := s.tasks.IsMember(ctx, input.TeamID, input.CreatedBy)
	if err != nil {
		return nil, err
	}
	if !member {
		return nil, ErrNotTeamMember
	}

	// Создание задачи со статусом "todo"
	status := "todo"
	task := &store.Task{
		Title:       input.Title,
		Description: input.Description,
		Status:      status,
		AssigneeID:  input.AssigneeID,
		TeamID:      input.TeamID,
		CreatedBy:   input.CreatedBy,
	}
	if err := s.tasks.Create(ctx, task); err != nil {
		return nil, err
	}

	return task, nil
}

// ListTaskFilter — параметры фильтрации и пагинации для списка задач.
type ListTaskFilter struct {
	// TeamID — фильтр по ID команды (nil = все команды пользователя).
	TeamID *uint64
	// Status — фильтр по статусу (nil = все статусы).
	Status *string
	// AssigneeID — фильтр по исполнителю (nil = все исполнители).
	AssigneeID *uint64
	// Page — номер страницы (1-based).
	Page int
	// PerPage — количество элементов на странице (по умолчанию 20).
	PerPage int
}

// TaskListResult — результат постраничного списка задач с метаданными пагинации.
type TaskListResult struct {
	// Tasks — текущая страница результатов.
	Tasks []*store.Task `json:"tasks"`
	// Total — общее количество задач, соответствующих фильтру.
	Total int `json:"total"`
	// Page — текущий номер страницы.
	Page int `json:"page"`
	// PerPage — количество элементов на странице.
	PerPage int `json:"per_page"`
	// TotalPages — общее количество страниц.
	TotalPages int `json:"total_pages"`
}

// List возвращает страницу задач с фильтрацией.
// userID используется для авторизации; пользователь видит задачи только своих команд.
func (s *TaskService) List(ctx context.Context, userID uint64, filter ListTaskFilter) (*TaskListResult, error) {
	// Делегирование запроса store-слою
	tasks, total, err := s.tasks.List(ctx, store.TaskFilter{
		TeamID:     filter.TeamID,
		Status:     filter.Status,
		AssigneeID: filter.AssigneeID,
		Page:       filter.Page,
		PerPage:    filter.PerPage,
	})
	if err != nil {
		return nil, err
	}

	// Расчёт метаданных пагинации
	perPage := filter.PerPage
	if perPage <= 0 {
		perPage = 20
	}

	totalPages := total / perPage
	if total%perPage != 0 {
		totalPages++
	}

	return &TaskListResult{
		Tasks:      tasks,
		Total:      total,
		Page:       filter.Page,
		PerPage:    perPage,
		TotalPages: totalPages,
	}, nil
}

// GetByID возвращает задачу по её ID.
// Возвращает ошибку, если задача не найдена.
func (s *TaskService) GetByID(ctx context.Context, taskID uint64) (*store.Task, error) {
	return s.tasks.GetByID(ctx, taskID)
}

// Update применяет частичные изменения к задаче.
// Проверяет членство пользователя в команде и записывает историю изменений.
// Параметры title, description, status, assigneeID — nil означает "без изменений".
func (s *TaskService) Update(ctx context.Context, taskID, userID uint64, title, description, status *string, assigneeID *uint64) (*store.Task, error) {
	// Получение текущей задачи
	task, err := s.tasks.GetByID(ctx, taskID)
	if err != nil {
		return nil, err
	}

	// Проверка членства в команде
	member, err := s.tasks.IsMember(ctx, task.TeamID, userID)
	if err != nil {
		return nil, err
	}
	if !member {
		return nil, ErrNotTeamMember
	}

	// Обновление заголовка с записью в историю
	if title != nil && *title != task.Title {
		old := task.Title
		task.Title = *title
		if err := s.tasks.AppendHistory(ctx, taskID, userID, "title", old, *title); err != nil {
			return nil, err
		}
	}

	// Обновление описания с записью в историю
	if description != nil && (*description != "" || task.Description != nil) {
		old := ""
		if task.Description != nil {
			old = *task.Description
		}
		task.Description = description
		if old != *description {
			if err := s.tasks.AppendHistory(ctx, taskID, userID, "description", old, *description); err != nil {
				return nil, err
			}
		}
	}

	// Обновление статуса с записью в историю
	if status != nil && *status != task.Status {
		old := task.Status
		task.Status = *status
		if err := s.tasks.AppendHistory(ctx, taskID, userID, "status", old, *status); err != nil {
			return nil, err
		}
	}

	// Обновление исполнителя с записью в историю
	if assigneeID != nil && (task.AssigneeID == nil || *assigneeID != *task.AssigneeID) {
		old := fmt.Sprintf("%d", task.AssigneeID)
		task.AssigneeID = assigneeID
		if err := s.tasks.AppendHistory(ctx, taskID, userID, "assignee_id", old, fmt.Sprintf("%d", *assigneeID)); err != nil {
			return nil, err
		}
	}

	// Сохранение изменений в БД
	if err := s.tasks.Update(ctx, task); err != nil {
		return nil, err
	}

	return task, nil
}

// IsMember проверяет, является ли пользователь участником указанной команды.
func (s *TaskService) IsMember(ctx context.Context, teamID, userID uint64) (bool, error) {
	return s.tasks.IsMember(ctx, teamID, userID)
}

// GetHistory возвращает полную историю изменений задачи (отсортировано по убыванию даты).
func (s *TaskService) GetHistory(ctx context.Context, taskID uint64) ([]*store.TaskHistory, error) {
	return s.tasks.GetHistory(ctx, taskID)
}
