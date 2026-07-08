package service

import (
	"context"

	"github.com/turkprogrammer/taskflow/internal/store"
)

// TeamService управляет жизненным циклом команд: создание, список, приглашение участников.
// Каждая команда должна иметь хотя бы одного owner (создателя).
type TeamService struct {
	teams TeamStore // интерфейс доступа к данным команд
}

// NewTeamService создаёт новый TeamService с переданным store.
func NewTeamService(teams TeamStore) *TeamService {
	return &TeamService{teams: teams}
}

// Create создаёт новую команду и автоматически добавляет создателя как owner.
// Использует транзакцию для保证原子ности операции.
func (s *TeamService) Create(ctx context.Context, name string, userID uint64) (*store.Team, error) {
	team := &store.Team{Name: name, CreatedBy: userID}
	if err := s.teams.CreateWithOwner(ctx, team, userID); err != nil {
		return nil, err
	}
	return team, nil
}

// List возвращает все команды, в которых состоит указанный пользователь.
func (s *TeamService) List(ctx context.Context, userID uint64) ([]*store.Team, error) {
	return s.teams.ListByUserID(ctx, userID)
}

// Invite добавляет пользователя в команду.
// Только owner или admin могут приглашать новых участников.
// Возвращает ErrAccessDenied, если приглашающий не имеет достаточных прав.
func (s *TeamService) Invite(ctx context.Context, teamID, inviterID, inviteeID uint64) error {
	// Проверка роли приглашающего
	role, err := s.teams.GetMemberRole(ctx, teamID, inviterID)
	if err != nil {
		return ErrAccessDenied
	}

	// Только owner и admin могут приглашать
	if role != "owner" && role != "admin" {
		return ErrAccessDenied
	}

	// Добавление участника с ролью "member"
	return s.teams.AddMember(ctx, teamID, inviteeID, "member")
}
