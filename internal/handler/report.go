package handler

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/turkprogrammer/taskflow/internal/service"
)

// ReportHandler реализует read-only эндпоинты аналитики для команд.
// Запрашивает агрегированные данные напрямую из ReportStore.
type ReportHandler struct {
	reportStore service.ReportStore // интерфейс доступа к аналитическим запросам
}

// NewReportHandler создаёт новый ReportHandler с указанным store.
func NewReportHandler(reportStore service.ReportStore) *ReportHandler {
	return &ReportHandler{reportStore: reportStore}
}

// TeamDashboard обрабатывает GET /api/v1/teams/{id}/dashboard.
// Возвращает сводные метрики команды: задачи по статусам,完成率, статистику участников.
func (h *ReportHandler) TeamDashboard(w http.ResponseWriter, r *http.Request) {
	// Парсинг ID команды из URL
	teamIDStr := r.PathValue("id")
	teamID, err := strconv.ParseUint(teamIDStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid team id"})
		return
	}

	// Получение данных дашборда
	dashboard, err := h.reportStore.GetTeamDashboard(r.Context(), teamID)
	if err != nil {
		slog.Error("team dashboard failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, dashboard)
}

// TopUsers обрабатывает GET /api/v1/reports/top-users.
// Возвращает топ-3 создателей задач в каждой команде за текущий месяц.
func (h *ReportHandler) TopUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.reportStore.GetTopUsersPerTeam(r.Context())
	if err != nil {
		slog.Error("top users report failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, users)
}

// InefficientTasks обрабатывает GET /api/v1/reports/inefficient-tasks.
// Возвращает задачи, где исполнитель не является участником команды (валидация целостности).
func (h *ReportHandler) InefficientTasks(w http.ResponseWriter, r *http.Request) {
	tasks, err := h.reportStore.GetInefficientTasks(r.Context())
	if err != nil {
		slog.Error("inefficient tasks report failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, tasks)
}
