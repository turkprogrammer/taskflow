package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/turkprogrammer/taskflow/internal/service"
	"github.com/turkprogrammer/taskflow/internal/service/mocks"
	"github.com/turkprogrammer/taskflow/internal/store"
)

func newTaskHandler(t *testing.T) (*TaskHandler, *mocks.MockTaskStore) {
	t.Helper()
	ctrl := gomock.NewController(t)
	mockTask := mocks.NewMockTaskStore(ctrl)
	svc := service.NewTaskService(mockTask)
	return NewTaskHandler(svc), mockTask
}

func withUserID(ctx context.Context, userID uint64) context.Context {
	return context.WithValue(ctx, UserIDKey, userID)
}

func TestTaskHandler_Create_Success(t *testing.T) {
	h, mock := newTaskHandler(t)

	mock.EXPECT().IsMember(gomock.Any(), uint64(1), uint64(42)).Return(true, nil)
	mock.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ interface{}, task *store.Task) error {
		task.ID = 10
		return nil
	})

	body, _ := json.Marshal(map[string]interface{}{"title": "New Task", "team_id": 1})
	req := httptest.NewRequest("POST", "/api/v1/tasks", bytes.NewReader(body))
	req = req.WithContext(withUserID(req.Context(), 42))
	w := httptest.NewRecorder()

	h.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTaskHandler_Create_NotMember(t *testing.T) {
	h, mock := newTaskHandler(t)

	mock.EXPECT().IsMember(gomock.Any(), uint64(1), uint64(99)).Return(false, nil)

	body, _ := json.Marshal(map[string]interface{}{"title": "Task", "team_id": 1})
	req := httptest.NewRequest("POST", "/api/v1/tasks", bytes.NewReader(body))
	req = req.WithContext(withUserID(req.Context(), 99))
	w := httptest.NewRecorder()

	h.Create(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestTaskHandler_Create_MissingTitle(t *testing.T) {
	h, _ := newTaskHandler(t)

	body, _ := json.Marshal(map[string]interface{}{"team_id": 1})
	req := httptest.NewRequest("POST", "/api/v1/tasks", bytes.NewReader(body))
	req = req.WithContext(withUserID(req.Context(), 42))
	w := httptest.NewRecorder()

	h.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestTaskHandler_Create_MissingTeamID(t *testing.T) {
	h, _ := newTaskHandler(t)

	body, _ := json.Marshal(map[string]interface{}{"title": "Task"})
	req := httptest.NewRequest("POST", "/api/v1/tasks", bytes.NewReader(body))
	req = req.WithContext(withUserID(req.Context(), 42))
	w := httptest.NewRecorder()

	h.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestTaskHandler_Create_Unauthorized(t *testing.T) {
	h, _ := newTaskHandler(t)

	body, _ := json.Marshal(map[string]interface{}{"title": "Task", "team_id": 1})
	req := httptest.NewRequest("POST", "/api/v1/tasks", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.Create(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestTaskHandler_List_Success(t *testing.T) {
	h, mock := newTaskHandler(t)

	mock.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*store.Task{
		{ID: 1, Title: "Task 1"},
	}, 1, nil)

	req := httptest.NewRequest("GET", "/api/v1/tasks?team_id=1", nil)
	req = req.WithContext(withUserID(req.Context(), 42))
	w := httptest.NewRecorder()

	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTaskHandler_List_InvalidTeamID(t *testing.T) {
	h, _ := newTaskHandler(t)

	req := httptest.NewRequest("GET", "/api/v1/tasks?team_id=abc", nil)
	req = req.WithContext(withUserID(req.Context(), 42))
	w := httptest.NewRecorder()

	h.List(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestTaskHandler_List_InvalidAssigneeID(t *testing.T) {
	h, _ := newTaskHandler(t)

	req := httptest.NewRequest("GET", "/api/v1/tasks?assignee_id=abc", nil)
	req = req.WithContext(withUserID(req.Context(), 42))
	w := httptest.NewRecorder()

	h.List(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestTaskHandler_Update_Success(t *testing.T) {
	h, mock := newTaskHandler(t)

	mock.EXPECT().GetByID(gomock.Any(), uint64(1)).Return(&store.Task{ID: 1, Status: "todo", TeamID: 1}, nil)
	mock.EXPECT().IsMember(gomock.Any(), uint64(1), uint64(42)).Return(true, nil)
	mock.EXPECT().AppendHistory(gomock.Any(), uint64(1), uint64(42), "status", "todo", "done").Return(nil)
	mock.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

	body, _ := json.Marshal(map[string]interface{}{"status": "done"})
	req := httptest.NewRequest("PUT", "/api/v1/tasks/1", bytes.NewReader(body))
	req = req.WithContext(withUserID(req.Context(), 42))
	req.SetPathValue("id", "1")
	w := httptest.NewRecorder()

	h.Update(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTaskHandler_Update_InvalidID(t *testing.T) {
	h, _ := newTaskHandler(t)

	body, _ := json.Marshal(map[string]interface{}{"status": "done"})
	req := httptest.NewRequest("PUT", "/api/v1/tasks/abc", bytes.NewReader(body))
	req = req.WithContext(withUserID(req.Context(), 42))
	req.SetPathValue("id", "abc")
	w := httptest.NewRecorder()

	h.Update(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestTaskHandler_Update_NotMember(t *testing.T) {
	h, mock := newTaskHandler(t)

	mock.EXPECT().GetByID(gomock.Any(), uint64(1)).Return(&store.Task{ID: 1, Status: "todo", TeamID: 1}, nil)
	mock.EXPECT().IsMember(gomock.Any(), uint64(1), uint64(99)).Return(false, nil)

	body, _ := json.Marshal(map[string]interface{}{"status": "done"})
	req := httptest.NewRequest("PUT", "/api/v1/tasks/1", bytes.NewReader(body))
	req = req.WithContext(withUserID(req.Context(), 99))
	req.SetPathValue("id", "1")
	w := httptest.NewRecorder()

	h.Update(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestTaskHandler_GetHistory_Success(t *testing.T) {
	h, mock := newTaskHandler(t)

	mock.EXPECT().GetByID(gomock.Any(), uint64(1)).Return(&store.Task{ID: 1, TeamID: 1}, nil)
	mock.EXPECT().IsMember(gomock.Any(), uint64(1), uint64(42)).Return(true, nil)
	mock.EXPECT().GetHistory(gomock.Any(), uint64(1)).Return([]*store.TaskHistory{
		{ID: 1, TaskID: 1, FieldChanged: "status"},
	}, nil)

	req := httptest.NewRequest("GET", "/api/v1/tasks/1/history", nil)
	req = req.WithContext(withUserID(req.Context(), 42))
	req.SetPathValue("id", "1")
	w := httptest.NewRecorder()

	h.GetHistory(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTaskHandler_GetHistory_TaskNotFound(t *testing.T) {
	h, mock := newTaskHandler(t)

	mock.EXPECT().GetByID(gomock.Any(), uint64(999)).Return(nil, errors.New("not found"))

	req := httptest.NewRequest("GET", "/api/v1/tasks/999/history", nil)
	req = req.WithContext(withUserID(req.Context(), 42))
	req.SetPathValue("id", "999")
	w := httptest.NewRecorder()

	h.GetHistory(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestTaskHandler_GetHistory_NotMember(t *testing.T) {
	h, mock := newTaskHandler(t)

	mock.EXPECT().GetByID(gomock.Any(), uint64(1)).Return(&store.Task{ID: 1, TeamID: 1}, nil)
	mock.EXPECT().IsMember(gomock.Any(), uint64(1), uint64(99)).Return(false, nil)

	req := httptest.NewRequest("GET", "/api/v1/tasks/1/history", nil)
	req = req.WithContext(withUserID(req.Context(), 99))
	req.SetPathValue("id", "1")
	w := httptest.NewRecorder()

	h.GetHistory(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestTaskHandler_GetHistory_InvalidID(t *testing.T) {
	h, _ := newTaskHandler(t)

	req := httptest.NewRequest("GET", "/api/v1/tasks/abc/history", nil)
	req = req.WithContext(withUserID(req.Context(), 42))
	req.SetPathValue("id", "abc")
	w := httptest.NewRecorder()

	h.GetHistory(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}
