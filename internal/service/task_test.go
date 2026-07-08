package service

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/turkprogrammer/taskflow/internal/service/mocks"
	"github.com/turkprogrammer/taskflow/internal/store"
)

// Тест: успешное создание задачи
func TestTaskService_Create_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTaskStore := mocks.NewMockTaskStore(ctrl)
	svc := NewTaskService(mockTaskStore)

	// Мок: пользователь является участником команды
	mockTaskStore.EXPECT().
		IsMember(gomock.Any(), uint64(1), uint64(42)).
		Return(true, nil)

	// Мок: успешное создание задачи
	mockTaskStore.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, task *store.Task) error {
			task.ID = 10
			return nil
		})

	task, err := svc.Create(context.Background(), CreateTaskInput{
		Title:     "Test Task",
		TeamID:    1,
		CreatedBy: 42,
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if task.ID != 10 {
		t.Fatalf("expected task ID 10, got %d", task.ID)
	}
	if task.Title != "Test Task" {
		t.Fatalf("expected 'Test Task', got %s", task.Title)
	}
	if task.Status != "todo" {
		t.Fatalf("expected status 'todo', got %s", task.Status)
	}
}

// Тест: ошибка проверки членства в команде
func TestTaskService_Create_IsMemberError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTaskStore := mocks.NewMockTaskStore(ctrl)
	svc := NewTaskService(mockTaskStore)

	// Мок: ошибка при проверке членства
	mockTaskStore.EXPECT().
		IsMember(gomock.Any(), uint64(1), uint64(42)).
		Return(false, errors.New("db error"))

	_, err := svc.Create(context.Background(), CreateTaskInput{
		Title:     "Task",
		TeamID:    1,
		CreatedBy: 42,
	})
	if err == nil {
		t.Fatal("expected error when IsMember fails")
	}
}

// Тест: пользователь не является участником команды
func TestTaskService_Create_NotMember(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTaskStore := mocks.NewMockTaskStore(ctrl)
	svc := NewTaskService(mockTaskStore)

	// Мок: пользователь не состоит в команде
	mockTaskStore.EXPECT().
		IsMember(gomock.Any(), uint64(1), uint64(99)).
		Return(false, nil)

	_, err := svc.Create(context.Background(), CreateTaskInput{
		Title:     "Task",
		TeamID:    1,
		CreatedBy: 99,
	})
	if err == nil || err.Error() != "you are not a member of this team" {
		t.Fatalf("expected membership error, got %v", err)
	}
}

// Тест: успешный список задач с фильтрацией
func TestTaskService_List_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTaskStore := mocks.NewMockTaskStore(ctrl)
	svc := NewTaskService(mockTaskStore)

	teamID := uint64(1)
	expected := []*store.Task{
		{ID: 1, Title: "Task 1", TeamID: 1},
		{ID: 2, Title: "Task 2", TeamID: 1},
	}

	// Мок: возврат списка задач
	mockTaskStore.EXPECT().
		List(gomock.Any(), store.TaskFilter{TeamID: &teamID, Page: 1, PerPage: 20}).
		Return(expected, 2, nil)

	result, err := svc.List(context.Background(), 42, ListTaskFilter{
		TeamID:  &teamID,
		Page:    1,
		PerPage: 20,
	})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if result.Total != 2 {
		t.Fatalf("expected total 2, got %d", result.Total)
	}
	if len(result.Tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(result.Tasks))
	}
}

// Тест: получение задачи по ID
func TestTaskService_GetByID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTaskStore := mocks.NewMockTaskStore(ctrl)
	svc := NewTaskService(mockTaskStore)

	// Мок: возврат задачи по ID
	mockTaskStore.EXPECT().
		GetByID(gomock.Any(), uint64(5)).
		Return(&store.Task{ID: 5, Title: "Task 5"}, nil)

	task, err := svc.GetByID(context.Background(), 5)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if task.Title != "Task 5" {
		t.Fatalf("expected 'Task 5', got %s", task.Title)
	}
}

// Тест: получение истории изменений задачи
func TestTaskService_GetHistory(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTaskStore := mocks.NewMockTaskStore(ctrl)
	svc := NewTaskService(mockTaskStore)

	// Мок: возврат истории изменений
	mockTaskStore.EXPECT().
		GetHistory(gomock.Any(), uint64(1)).
		Return([]*store.TaskHistory{
			{ID: 1, TaskID: 1, FieldChanged: "status"},
		}, nil)

	history, err := svc.GetHistory(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(history))
	}
}

// Тест: успешное обновление статуса задачи
func TestTaskService_Update_Status(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTaskStore := mocks.NewMockTaskStore(ctrl)
	svc := NewTaskService(mockTaskStore)

	// Мок: получение задачи
	mockTaskStore.EXPECT().
		GetByID(gomock.Any(), uint64(1)).
		Return(&store.Task{ID: 1, Title: "Task", Status: "todo", TeamID: 1}, nil)

	// Мок: проверка членства
	mockTaskStore.EXPECT().
		IsMember(gomock.Any(), uint64(1), uint64(42)).
		Return(true, nil)

	// Мок: запись в историю изменений
	mockTaskStore.EXPECT().
		AppendHistory(gomock.Any(), uint64(1), uint64(42), "status", "todo", "done").
		Return(nil)

	// Мок: обновление задачи
	mockTaskStore.EXPECT().
		Update(gomock.Any(), gomock.Any()).
		Return(nil)

	newStatus := "done"
	task, err := svc.Update(context.Background(), 1, 42, nil, nil, &newStatus, nil)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if task.Status != "done" {
		t.Fatalf("expected status 'done', got %s", task.Status)
	}
}

// Тест: обновление задачи без членства в команде
func TestTaskService_Update_NotMember(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTaskStore := mocks.NewMockTaskStore(ctrl)
	svc := NewTaskService(mockTaskStore)

	// Мок: получение задачи
	mockTaskStore.EXPECT().
		GetByID(gomock.Any(), uint64(1)).
		Return(&store.Task{ID: 1, TeamID: 1}, nil)

	// Мок: пользователь не состоит в команде
	mockTaskStore.EXPECT().
		IsMember(gomock.Any(), uint64(1), uint64(99)).
		Return(false, nil)

	newStatus := "done"
	_, err := svc.Update(context.Background(), 1, 99, nil, nil, &newStatus, nil)
	if err == nil || err.Error() != "you are not a member of this team" {
		t.Fatalf("expected membership error, got %v", err)
	}
}

// Тест: успешное обновление заголовка задачи
func TestTaskService_Update_Title(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTaskStore := mocks.NewMockTaskStore(ctrl)
	svc := NewTaskService(mockTaskStore)

	// Мок: получение задачи
	mockTaskStore.EXPECT().
		GetByID(gomock.Any(), uint64(1)).
		Return(&store.Task{ID: 1, Title: "Old", Status: "todo", TeamID: 1}, nil)

	// Мок: проверка членства
	mockTaskStore.EXPECT().
		IsMember(gomock.Any(), uint64(1), uint64(42)).
		Return(true, nil)

	// Мок: запись в историю
	mockTaskStore.EXPECT().
		AppendHistory(gomock.Any(), uint64(1), uint64(42), "title", "Old", "New Title").
		Return(nil)

	// Мок: обновление задачи
	mockTaskStore.EXPECT().
		Update(gomock.Any(), gomock.Any()).
		Return(nil)

	newTitle := "New Title"
	task, err := svc.Update(context.Background(), 1, 42, &newTitle, nil, nil, nil)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if task.Title != "New Title" {
		t.Fatalf("expected 'New Title', got %s", task.Title)
	}
}

// Тест: обновление описания с nil на значение
func TestTaskService_Update_Description_FromNil(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTaskStore := mocks.NewMockTaskStore(ctrl)
	svc := NewTaskService(mockTaskStore)

	// Мок: получение задачи без описания
	mockTaskStore.EXPECT().
		GetByID(gomock.Any(), uint64(1)).
		Return(&store.Task{ID: 1, Title: "Task", Status: "todo", TeamID: 1}, nil)

	// Мок: проверка членства
	mockTaskStore.EXPECT().
		IsMember(gomock.Any(), uint64(1), uint64(42)).
		Return(true, nil)

	// Мок: запись в историю (описание: "" → "new desc")
	mockTaskStore.EXPECT().
		AppendHistory(gomock.Any(), uint64(1), uint64(42), "description", "", "new desc").
		Return(nil)

	// Мок: обновление задачи
	mockTaskStore.EXPECT().
		Update(gomock.Any(), gomock.Any()).
		Return(nil)

	desc := "new desc"
	task, err := svc.Update(context.Background(), 1, 42, nil, &desc, nil, nil)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if task.Description == nil || *task.Description != "new desc" {
		t.Fatalf("expected 'new desc', got %v", *task.Description)
	}
}

// Тест: обновление существующего описания
func TestTaskService_Update_Description(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTaskStore := mocks.NewMockTaskStore(ctrl)
	svc := NewTaskService(mockTaskStore)

	desc := "new description"
	// Мок: получение задачи с описанием
	mockTaskStore.EXPECT().
		GetByID(gomock.Any(), uint64(1)).
		Return(&store.Task{ID: 1, Title: "Task", Description: &desc, Status: "todo", TeamID: 1}, nil)

	// Мок: проверка членства
	mockTaskStore.EXPECT().
		IsMember(gomock.Any(), uint64(1), uint64(42)).
		Return(true, nil)

	// Мок: запись в историю
	mockTaskStore.EXPECT().
		AppendHistory(gomock.Any(), uint64(1), uint64(42), "description", "new description", "updated desc").
		Return(nil)

	// Мок: обновление задачи
	mockTaskStore.EXPECT().
		Update(gomock.Any(), gomock.Any()).
		Return(nil)

	updatedDesc := "updated desc"
	task, err := svc.Update(context.Background(), 1, 42, nil, &updatedDesc, nil, nil)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if task.Description == nil || *task.Description != "updated desc" {
		t.Fatalf("expected 'updated desc', got %v", *task.Description)
	}
}

// Тест: обновление исполнителя задачи
func TestTaskService_Update_Assignee(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTaskStore := mocks.NewMockTaskStore(ctrl)
	svc := NewTaskService(mockTaskStore)

	// Мок: получение задачи без исполнителя
	mockTaskStore.EXPECT().
		GetByID(gomock.Any(), uint64(1)).
		Return(&store.Task{ID: 1, Title: "Task", Status: "todo", TeamID: 1}, nil)

	// Мок: проверка членства
	mockTaskStore.EXPECT().
		IsMember(gomock.Any(), uint64(1), uint64(42)).
		Return(true, nil)

	// Мок: запись в историю
	mockTaskStore.EXPECT().
		AppendHistory(gomock.Any(), uint64(1), uint64(42), "assignee_id", "0", "5").
		Return(nil)

	// Мок: обновление задачи
	mockTaskStore.EXPECT().
		Update(gomock.Any(), gomock.Any()).
		Return(nil)

	assignee := uint64(5)
	task, err := svc.Update(context.Background(), 1, 42, nil, nil, nil, &assignee)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if task.AssigneeID == nil || *task.AssigneeID != 5 {
		t.Fatalf("expected assignee 5, got %v", task.AssigneeID)
	}
}

// Тест: пагинация по умолчанию (PerPage = 20)
func TestTaskService_List_DefaultPerPage(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTaskStore := mocks.NewMockTaskStore(ctrl)
	svc := NewTaskService(mockTaskStore)

	// Мок: возврат пустого списка
	mockTaskStore.EXPECT().
		List(gomock.Any(), store.TaskFilter{Page: 0, PerPage: 0}).
		Return([]*store.Task{}, 5, nil)

	result, err := svc.List(context.Background(), 42, ListTaskFilter{})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if result.PerPage != 20 {
		t.Fatalf("expected default PerPage 20, got %d", result.PerPage)
	}
	if result.TotalPages != 1 {
		t.Fatalf("expected 1 total page for 5 items with PerPage 20, got %d", result.TotalPages)
	}
}

// Тест: пагинация с несколькими страницами
func TestTaskService_List_MultiplePages(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTaskStore := mocks.NewMockTaskStore(ctrl)
	svc := NewTaskService(mockTaskStore)

	teamID := uint64(1)
	mockTaskStore.EXPECT().
		List(gomock.Any(), store.TaskFilter{TeamID: &teamID, Page: 1, PerPage: 2}).
		Return([]*store.Task{
			{ID: 1, Title: "Task 1", TeamID: 1},
			{ID: 2, Title: "Task 2", TeamID: 1},
		}, 5, nil)

	result, err := svc.List(context.Background(), 42, ListTaskFilter{
		TeamID:  &teamID,
		Page:    1,
		PerPage: 2,
	})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if result.TotalPages != 3 {
		t.Fatalf("expected 3 pages (5/2=2.5→3), got %d", result.TotalPages)
	}
}

// Тест: пагинация с точным делением
func TestTaskService_List_ExactPage(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTaskStore := mocks.NewMockTaskStore(ctrl)
	svc := NewTaskService(mockTaskStore)

	mockTaskStore.EXPECT().
		List(gomock.Any(), store.TaskFilter{Page: 1, PerPage: 2}).
		Return([]*store.Task{
			{ID: 1, Title: "T1"}, {ID: 2, Title: "T2"},
		}, 2, nil)

	result, err := svc.List(context.Background(), 42, ListTaskFilter{
		Page:    1,
		PerPage: 2,
	})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if result.TotalPages != 1 {
		t.Fatalf("expected 1 page for 2 items with PerPage 2, got %d", result.TotalPages)
	}
}

// Тест: обновление несуществующей задачи
func TestTaskService_Update_TaskNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTaskStore := mocks.NewMockTaskStore(ctrl)
	svc := NewTaskService(mockTaskStore)

	// Мок: задача не найдена
	mockTaskStore.EXPECT().
		GetByID(gomock.Any(), uint64(999)).
		Return(nil, errors.New("not found"))

	newStatus := "done"
	_, err := svc.Update(context.Background(), 999, 42, nil, nil, &newStatus, nil)
	if err == nil {
		t.Fatal("expected error for non-existent task")
	}
}

// Тест: проверка членства в команде (true)
func TestTaskService_IsMember_True(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTaskStore := mocks.NewMockTaskStore(ctrl)
	svc := NewTaskService(mockTaskStore)

	// Мок: пользователь является участником
	mockTaskStore.EXPECT().
		IsMember(gomock.Any(), uint64(1), uint64(42)).
		Return(true, nil)

	member, err := svc.IsMember(context.Background(), 1, 42)
	if err != nil {
		t.Fatalf("IsMember failed: %v", err)
	}
	if !member {
		t.Fatal("expected user to be member")
	}
}

// Тест: проверка членства в команде (false)
func TestTaskService_IsMember_False(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTaskStore := mocks.NewMockTaskStore(ctrl)
	svc := NewTaskService(mockTaskStore)

	// Мок: пользователь не является участником
	mockTaskStore.EXPECT().
		IsMember(gomock.Any(), uint64(1), uint64(99)).
		Return(false, nil)

	member, err := svc.IsMember(context.Background(), 1, 99)
	if err != nil {
		t.Fatalf("IsMember failed: %v", err)
	}
	if member {
		t.Fatal("expected user not to be member")
	}
}

// Тест: ошибка при проверке членства
func TestTaskService_IsMember_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTaskStore := mocks.NewMockTaskStore(ctrl)
	svc := NewTaskService(mockTaskStore)

	// Мок: ошибка БД
	mockTaskStore.EXPECT().
		IsMember(gomock.Any(), uint64(1), uint64(42)).
		Return(false, errors.New("db error"))

	_, err := svc.IsMember(context.Background(), 1, 42)
	if err == nil {
		t.Fatal("expected error")
	}
}
