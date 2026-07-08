package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/turkprogrammer/taskflow/internal/service"
	"github.com/turkprogrammer/taskflow/internal/service/mocks"
	"github.com/turkprogrammer/taskflow/internal/store"
)

func newTeamHandler(t *testing.T) (*TeamHandler, *mocks.MockTeamStore) {
	t.Helper()
	ctrl := gomock.NewController(t)
	mockTeam := mocks.NewMockTeamStore(ctrl)
	svc := service.NewTeamService(mockTeam)
	return NewTeamHandler(svc), mockTeam
}

func TestTeamHandler_Create_Success(t *testing.T) {
	h, mock := newTeamHandler(t)

	mock.EXPECT().CreateWithOwner(gomock.Any(), gomock.Any(), uint64(42)).DoAndReturn(
		func(_ interface{}, team *store.Team, _ uint64) error {
			team.ID = 1
			return nil
		},
	)

	body, _ := json.Marshal(map[string]string{"name": "My Team"})
	req := httptest.NewRequest("POST", "/api/v1/teams", bytes.NewReader(body))
	req = req.WithContext(withUserID(req.Context(), 42))
	w := httptest.NewRecorder()

	h.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTeamHandler_Create_MissingName(t *testing.T) {
	h, _ := newTeamHandler(t)

	body, _ := json.Marshal(map[string]string{"name": ""})
	req := httptest.NewRequest("POST", "/api/v1/teams", bytes.NewReader(body))
	req = req.WithContext(withUserID(req.Context(), 42))
	w := httptest.NewRecorder()

	h.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestTeamHandler_Create_Unauthorized(t *testing.T) {
	h, _ := newTeamHandler(t)

	body, _ := json.Marshal(map[string]string{"name": "Team"})
	req := httptest.NewRequest("POST", "/api/v1/teams", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.Create(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestTeamHandler_List_Success(t *testing.T) {
	h, mock := newTeamHandler(t)

	mock.EXPECT().ListByUserID(gomock.Any(), uint64(42)).Return([]*store.Team{
		{ID: 1, Name: "Team A"},
		{ID: 2, Name: "Team B"},
	}, nil)

	req := httptest.NewRequest("GET", "/api/v1/teams", nil)
	req = req.WithContext(withUserID(req.Context(), 42))
	w := httptest.NewRecorder()

	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTeamHandler_List_Empty(t *testing.T) {
	h, mock := newTeamHandler(t)

	mock.EXPECT().ListByUserID(gomock.Any(), uint64(42)).Return(nil, nil)

	req := httptest.NewRequest("GET", "/api/v1/teams", nil)
	req = req.WithContext(withUserID(req.Context(), 42))
	w := httptest.NewRecorder()

	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestTeamHandler_Invite_Success(t *testing.T) {
	h, mock := newTeamHandler(t)

	mock.EXPECT().GetMemberRole(gomock.Any(), uint64(1), uint64(42)).Return("owner", nil)
	mock.EXPECT().AddMember(gomock.Any(), uint64(1), uint64(99), "member").Return(nil)

	body, _ := json.Marshal(map[string]interface{}{"user_id": 99})
	req := httptest.NewRequest("POST", "/api/v1/teams/1/invite", bytes.NewReader(body))
	req = req.WithContext(withUserID(req.Context(), 42))
	req.SetPathValue("id", "1")
	w := httptest.NewRecorder()

	h.Invite(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTeamHandler_Invite_AccessDenied(t *testing.T) {
	h, mock := newTeamHandler(t)

	mock.EXPECT().GetMemberRole(gomock.Any(), uint64(1), uint64(42)).Return("member", nil)

	body, _ := json.Marshal(map[string]interface{}{"user_id": 99})
	req := httptest.NewRequest("POST", "/api/v1/teams/1/invite", bytes.NewReader(body))
	req = req.WithContext(withUserID(req.Context(), 42))
	req.SetPathValue("id", "1")
	w := httptest.NewRecorder()

	h.Invite(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestTeamHandler_Invite_MissingUserID(t *testing.T) {
	h, _ := newTeamHandler(t)

	body, _ := json.Marshal(map[string]interface{}{"user_id": 0})
	req := httptest.NewRequest("POST", "/api/v1/teams/1/invite", bytes.NewReader(body))
	req = req.WithContext(withUserID(req.Context(), 42))
	req.SetPathValue("id", "1")
	w := httptest.NewRecorder()

	h.Invite(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestTeamHandler_Invite_InvalidTeamID(t *testing.T) {
	h, _ := newTeamHandler(t)

	body, _ := json.Marshal(map[string]interface{}{"user_id": 99})
	req := httptest.NewRequest("POST", "/api/v1/teams/abc/invite", bytes.NewReader(body))
	req = req.WithContext(withUserID(req.Context(), 42))
	req.SetPathValue("id", "abc")
	w := httptest.NewRecorder()

	h.Invite(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}
