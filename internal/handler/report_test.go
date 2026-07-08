package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/turkprogrammer/taskflow/internal/handler/mocks"
	"github.com/turkprogrammer/taskflow/internal/store"
)

func newReportHandler(t *testing.T) (*ReportHandler, *mocks.MockReportStore) {
	t.Helper()
	ctrl := gomock.NewController(t)
	mockReport := mocks.NewMockReportStore(ctrl)
	return NewReportHandler(mockReport), mockReport
}

func TestReportHandler_TeamDashboard_Success(t *testing.T) {
	h, mock := newReportHandler(t)

	mock.EXPECT().GetTeamDashboard(gomock.Any(), uint64(1)).Return(&store.TeamDashboard{
		StatusCounts:  map[string]int{"todo": 5, "done": 3},
		DoneLast7Days: 2,
		MemberCount:   4,
	}, nil)

	req := httptest.NewRequest("GET", "/api/v1/teams/1/dashboard", nil)
	req.SetPathValue("id", "1")
	w := httptest.NewRecorder()

	h.TeamDashboard(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp store.TeamDashboard
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.MemberCount != 4 {
		t.Fatalf("expected member_count 4, got %d", resp.MemberCount)
	}
}

func TestReportHandler_TeamDashboard_InvalidID(t *testing.T) {
	h, _ := newReportHandler(t)

	req := httptest.NewRequest("GET", "/api/v1/teams/abc/dashboard", nil)
	req.SetPathValue("id", "abc")
	w := httptest.NewRecorder()

	h.TeamDashboard(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestReportHandler_TeamDashboard_StoreError(t *testing.T) {
	h, mock := newReportHandler(t)

	mock.EXPECT().GetTeamDashboard(gomock.Any(), uint64(1)).Return(nil, errors.New("db error"))

	req := httptest.NewRequest("GET", "/api/v1/teams/1/dashboard", nil)
	req.SetPathValue("id", "1")
	w := httptest.NewRecorder()

	h.TeamDashboard(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestReportHandler_TopUsers_Success(t *testing.T) {
	h, mock := newReportHandler(t)

	mock.EXPECT().GetTopUsersPerTeam(gomock.Any()).Return([]store.TopUser{
		{TeamID: 1, UserID: 10, UserName: "Alice", TaskCount: 15, Position: 1},
		{TeamID: 1, UserID: 20, UserName: "Bob", TaskCount: 10, Position: 2},
	}, nil)

	req := httptest.NewRequest("GET", "/api/v1/reports/top-users", nil)
	w := httptest.NewRecorder()

	h.TopUsers(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp []store.TopUser
	json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp) != 2 {
		t.Fatalf("expected 2 users, got %d", len(resp))
	}
	if resp[0].UserName != "Alice" {
		t.Fatalf("expected first user Alice, got %s", resp[0].UserName)
	}
}

func TestReportHandler_TopUsers_EmptyResult(t *testing.T) {
	h, mock := newReportHandler(t)

	mock.EXPECT().GetTopUsersPerTeam(gomock.Any()).Return([]store.TopUser{}, nil)

	req := httptest.NewRequest("GET", "/api/v1/reports/top-users", nil)
	w := httptest.NewRecorder()

	h.TopUsers(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestReportHandler_TopUsers_StoreError(t *testing.T) {
	h, mock := newReportHandler(t)

	mock.EXPECT().GetTopUsersPerTeam(gomock.Any()).Return(nil, errors.New("db error"))

	req := httptest.NewRequest("GET", "/api/v1/reports/top-users", nil)
	w := httptest.NewRecorder()

	h.TopUsers(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestReportHandler_InefficientTasks_Success(t *testing.T) {
	h, mock := newReportHandler(t)

	mock.EXPECT().GetInefficientTasks(gomock.Any()).Return([]store.InefficientTask{
		{ID: 1, Title: "Stuck Task", AssigneeID: 5, AssigneeName: "Charlie", TeamID: 1, TeamName: "Team A", TaskRank: 1},
	}, nil)

	req := httptest.NewRequest("GET", "/api/v1/reports/inefficient-tasks", nil)
	w := httptest.NewRecorder()

	h.InefficientTasks(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp []store.InefficientTask
	json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp) != 1 {
		t.Fatalf("expected 1 task, got %d", len(resp))
	}
	if resp[0].Title != "Stuck Task" {
		t.Fatalf("expected title 'Stuck Task', got %s", resp[0].Title)
	}
}

func TestReportHandler_InefficientTasks_EmptyResult(t *testing.T) {
	h, mock := newReportHandler(t)

	mock.EXPECT().GetInefficientTasks(gomock.Any()).Return([]store.InefficientTask{}, nil)

	req := httptest.NewRequest("GET", "/api/v1/reports/inefficient-tasks", nil)
	w := httptest.NewRecorder()

	h.InefficientTasks(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestReportHandler_InefficientTasks_StoreError(t *testing.T) {
	h, mock := newReportHandler(t)

	mock.EXPECT().GetInefficientTasks(gomock.Any()).Return(nil, errors.New("db error"))

	req := httptest.NewRequest("GET", "/api/v1/reports/inefficient-tasks", nil)
	w := httptest.NewRecorder()

	h.InefficientTasks(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}
