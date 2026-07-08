package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/turkprogrammer/taskflow/internal/service"
)

// TeamHandler реализует CRUD-операции для команд и управление участниками.
// Каждый метод требует аутентифицированного пользователя (см. AuthHandler.Middleware).
type TeamHandler struct {
	svc *service.TeamService // бизнес-логика команд
}

// NewTeamHandler создаёт новый TeamHandler с указанным сервисом.
func NewTeamHandler(svc *service.TeamService) *TeamHandler {
	return &TeamHandler{svc: svc}
}

// Create обрабатывает POST /api/v1/teams.
// Создаёт новую команду, создатель автоматически становится owner.
// Тело запроса: { "name": "название команды" }
func (h *TeamHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	// Валидация обязательного поля
	if body.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}

	team, err := h.svc.Create(r.Context(), body.Name, userID)
	if err != nil {
		slog.Error("create team failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusCreated, team)
}

// List обрабатывает GET /api/v1/teams.
// Возвращает все команды, в которых состоит аутентифицированный пользователь.
func (h *TeamHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	teams, err := h.svc.List(r.Context(), userID)
	if err != nil {
		slog.Error("list teams failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, teams)
}

// Invite обрабатывает POST /api/v1/teams/{id}/invite.
// Добавляет пользователя в команду по user_id. Только owner/admin могут приглашать.
// Тело запроса: { "user_id": 123 }
func (h *TeamHandler) Invite(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	// Парсинг ID команды из URL
	teamIDStr := r.PathValue("id")
	teamID, err := strconv.ParseUint(teamIDStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid team id"})
		return
	}

	// Декодирование тела запроса
	var body struct {
		UserID uint64 `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if body.UserID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "user_id is required"})
		return
	}

	// Вызов бизнес-логики приглашения
	if err := h.svc.Invite(r.Context(), teamID, userID, body.UserID); err != nil {
		if errors.Is(err, service.ErrAccessDenied) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "access denied"})
			return
		}
		slog.Error("invite failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "invited"})
}
