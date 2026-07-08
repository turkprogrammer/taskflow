package service

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/turkprogrammer/taskflow/internal/service/mocks"
	"github.com/turkprogrammer/taskflow/internal/store"
)

func TestTeamService_Create_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTeamStore := mocks.NewMockTeamStore(ctrl)
	svc := NewTeamService(mockTeamStore)

	mockTeamStore.EXPECT().
		CreateWithOwner(gomock.Any(), gomock.Any(), uint64(42)).
		DoAndReturn(func(_ context.Context, team *store.Team, _ uint64) error {
			team.ID = 1
			return nil
		})

	team, err := svc.Create(context.Background(), "My Team", 42)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if team.ID != 1 {
		t.Fatalf("expected team ID 1, got %d", team.ID)
	}
	if team.Name != "My Team" {
		t.Fatalf("expected 'My Team', got %s", team.Name)
	}
	if team.CreatedBy != 42 {
		t.Fatalf("expected CreatedBy 42, got %d", team.CreatedBy)
	}
}

func TestTeamService_List_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTeamStore := mocks.NewMockTeamStore(ctrl)
	svc := NewTeamService(mockTeamStore)

	expected := []*store.Team{
		{ID: 1, Name: "Team A", CreatedBy: 42},
		{ID: 2, Name: "Team B", CreatedBy: 42},
	}

	mockTeamStore.EXPECT().
		ListByUserID(gomock.Any(), uint64(42)).
		Return(expected, nil)

	teams, err := svc.List(context.Background(), 42)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(teams) != 2 {
		t.Fatalf("expected 2 teams, got %d", len(teams))
	}
}

func TestTeamService_Invite_OwnerCanInvite(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTeamStore := mocks.NewMockTeamStore(ctrl)
	svc := NewTeamService(mockTeamStore)

	mockTeamStore.EXPECT().
		GetMemberRole(gomock.Any(), uint64(1), uint64(42)).
		Return("owner", nil)

	mockTeamStore.EXPECT().
		AddMember(gomock.Any(), uint64(1), uint64(99), "member").
		Return(nil)

	err := svc.Invite(context.Background(), 1, 42, 99)
	if err != nil {
		t.Fatalf("Invite failed: %v", err)
	}
}

func TestTeamService_Invite_MemberCannotInvite(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTeamStore := mocks.NewMockTeamStore(ctrl)
	svc := NewTeamService(mockTeamStore)

	mockTeamStore.EXPECT().
		GetMemberRole(gomock.Any(), uint64(1), uint64(42)).
		Return("member", nil)

	err := svc.Invite(context.Background(), 1, 42, 99)
	if err == nil || err.Error() != "access denied" {
		t.Fatalf("expected permission error, got %v", err)
	}
}

func TestTeamService_Create_AddMemberError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTeamStore := mocks.NewMockTeamStore(ctrl)
	svc := NewTeamService(mockTeamStore)

	mockTeamStore.EXPECT().
		CreateWithOwner(gomock.Any(), gomock.Any(), uint64(42)).
		Return(errors.New("db error"))

	_, err := svc.Create(context.Background(), "My Team", 42)
	if err == nil {
		t.Fatal("expected error when AddMember fails")
	}
}

func TestTeamService_Invite_NonMember(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTeamStore := mocks.NewMockTeamStore(ctrl)
	svc := NewTeamService(mockTeamStore)

	mockTeamStore.EXPECT().
		GetMemberRole(gomock.Any(), uint64(1), uint64(42)).
		Return("", errors.New("not found"))

	err := svc.Invite(context.Background(), 1, 42, 99)
	if err == nil || err.Error() != "access denied" {
		t.Fatalf("expected 'access denied', got %v", err)
	}
}
