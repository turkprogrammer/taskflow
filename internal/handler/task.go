package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/turkprogrammer/taskflow/internal/service"
)

// TaskHandler реализует CRUD-операции для задач и получение истории изменений.
// Каждый метод требует аутентифицированного пользователя, являющегося участником команды.
type TaskHandler struct {
	svc *service.TaskService // бизнес-логика задач
}

// NewTaskHandler создаёт новый TaskHandler с указанным сервисом.
func NewTaskHandler(svc *service.TaskService) *TaskHandler {
	return &TaskHandler{svc: svc}
}

// Create обрабатывает POST /api/v1/tasks.
// Создаёт новую задачу в команде. Тело запроса: title, team_id (обязательные), description, assignee_id (опциональные).
// Возвращает 403 Forbidden, если пользователь не является участником команды.
func (h *TaskHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var body struct {
		Title       string  `json:"title"`
		Description *string `json:"description"`
		AssigneeID  *uint64 `json:"assignee_id"`
		TeamID      uint64  `json:"team_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	// Валидация обязательных полей
	if body.Title == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "title is required"})
		return
	}
	if body.TeamID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "team_id is required"})
		return
	}

	task, err := h.svc.Create(r.Context(), service.CreateTaskInput{
		Title:       body.Title,
		Description: body.Description,
		AssigneeID:  body.AssigneeID,
		TeamID:      body.TeamID,
		CreatedBy:   userID,
	})
	if err != nil {
		if errors.Is(err, service.ErrNotTeamMember) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "you are not a member of this team"})
			return
		}
		slog.Error("create task failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusCreated, task)
}

// List обрабатывает GET /api/v1/tasks.
// Возвращает список задач с фильтрацией по query-параметрам: team_id, status, assignee_id, page, per_page.
func (h *TaskHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	q := r.URL.Query()

	// Парсинг параметров фильтрации
	var teamID *uint64
	if v := q.Get("team_id"); v != "" {
		id, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid team_id"})
			return
		}
		teamID = &id
	}

	var status *string
	if v := q.Get("status"); v != "" {
		status = &v
	}

	var assigneeID *uint64
	if v := q.Get("assignee_id"); v != "" {
		id, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid assignee_id"})
			return
		}
		assigneeID = &id
	}

	page, _ := strconv.Atoi(q.Get("page"))
	perPage, _ := strconv.Atoi(q.Get("per_page"))

	result, err := h.svc.List(r.Context(), userID, service.ListTaskFilter{
		TeamID:     teamID,
		Status:     status,
		AssigneeID: assigneeID,
		Page:       page,
		PerPage:    perPage,
	})
	if err != nil {
		slog.Error("list tasks failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// Update обрабатывает PUT /api/v1/tasks/{id}.
// Обновляет одну или несколько полей задачи (title, description, status, assignee_id).
// Только участники команды могут обновлять задачи.
func (h *TaskHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	// Парсинг ID задачи из URL
	taskIDStr := r.PathValue("id")
	taskID, err := strconv.ParseUint(taskIDStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid task id"})
		return
	}

	// Декодирование тела запроса (все поля опциональные)
	var body struct {
		Title       *string `json:"title"`
		Description *string `json:"description"`
		Status      *string `json:"status"`
		AssigneeID  *uint64 `json:"assignee_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	task, err := h.svc.Update(r.Context(), taskID, userID, body.Title, body.Description, body.Status, body.AssigneeID)
	if err != nil {
		if errors.Is(err, service.ErrNotTeamMember) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "you are not a member of this team"})
			return
		}
		slog.Error("update task failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, task)
}

// GetHistory обрабатывает GET /api/v1/tasks/{id}/history.
// Возвращает полную историю изменений задачи, показывая кто что изменил и когда.
func (h *TaskHandler) GetHistory(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	// Парсинг ID задачи из URL
	taskIDStr := r.PathValue("id")
	taskID, err := strconv.ParseUint(taskIDStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid task id"})
		return
	}

	// Получение задачи для проверки существования
	task, err := h.svc.GetByID(r.Context(), taskID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "task not found"})
		return
	}

	// Проверка членства в команде задачи
	member, err := h.svc.IsMember(r.Context(), task.TeamID, userID)
	if err != nil || !member {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "access denied"})
		return
	}

	// Получение истории изменений
	history, err := h.svc.GetHistory(r.Context(), taskID)
	if err != nil {
		slog.Error("get history failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, history)
}
