package store

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
)

var testDB *sql.DB

func TestMain(m *testing.M) {
	pool, err := dockertest.NewPool("")
	if err != nil {
		slog.Warn("docker not available, skipping integration tests", "error", err)
		os.Exit(m.Run())
	}

	resource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "mysql",
		Tag:        "8.0",
		Env: []string{
			"MYSQL_ROOT_PASSWORD=rootpass",
			"MYSQL_DATABASE=testdb",
			"MYSQL_USER=testuser",
			"MYSQL_PASSWORD=testpass",
		},
	}, func(hc *docker.HostConfig) {
		hc.AutoRemove = true
		hc.RestartPolicy = docker.RestartPolicy{Name: "no"}
	})
	if err != nil {
		slog.Warn("failed to start mysql container, skipping integration tests", "error", err)
		os.Exit(m.Run())
	}

	defer func() {
		if err := pool.Purge(resource); err != nil {
			slog.Error("failed to purge resource", "error", err)
		}
	}()

	dsn := fmt.Sprintf("testuser:testpass@tcp(localhost:%s)/testdb?charset=utf8mb4&parseTime=true&loc=Local", resource.GetPort("3306/tcp"))

	if err := pool.Retry(func() error {
		db, err := sql.Open("mysql", dsn)
		if err != nil {
			return err
		}
		return db.Ping()
	}); err != nil {
		slog.Warn("failed to connect to mysql, skipping integration tests", "error", err)
		os.Exit(m.Run())
	}

	testDB, err = sql.Open("mysql", dsn)
	if err != nil {
		slog.Warn("failed to open mysql, skipping integration tests", "error", err)
		os.Exit(m.Run())
	}
	testDB.SetMaxOpenConns(5)
	testDB.SetMaxIdleConns(2)
	testDB.SetConnMaxLifetime(5 * time.Minute)

	if err := runMigrations(testDB); err != nil {
		slog.Error("migrations failed", "error", err)
		os.Exit(1)
	}

	code := m.Run()
	testDB.Close()
	os.Exit(code)
}

func runMigrations(db *sql.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
			email VARCHAR(255) NOT NULL UNIQUE,
			password_hash VARCHAR(255) NOT NULL,
			name VARCHAR(255) NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		`CREATE TABLE IF NOT EXISTS teams (
			id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			created_by BIGINT UNSIGNED NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE CASCADE
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		`CREATE TABLE IF NOT EXISTS team_members (
			id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
			team_id BIGINT UNSIGNED NOT NULL,
			user_id BIGINT UNSIGNED NOT NULL,
			role ENUM('owner', 'admin', 'member') NOT NULL DEFAULT 'member',
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE KEY uq_team_member (team_id, user_id),
			FOREIGN KEY (team_id) REFERENCES teams(id) ON DELETE CASCADE,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		`CREATE TABLE IF NOT EXISTS tasks (
			id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
			title VARCHAR(255) NOT NULL,
			description TEXT,
			status ENUM('todo', 'in_progress', 'done') NOT NULL DEFAULT 'todo',
			assignee_id BIGINT UNSIGNED,
			team_id BIGINT UNSIGNED NOT NULL,
			created_by BIGINT UNSIGNED NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			FOREIGN KEY (assignee_id) REFERENCES users(id) ON DELETE SET NULL,
			FOREIGN KEY (team_id) REFERENCES teams(id) ON DELETE CASCADE,
			FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE CASCADE
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		`CREATE TABLE IF NOT EXISTS task_history (
			id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
			task_id BIGINT UNSIGNED NOT NULL,
			field_changed VARCHAR(255) NOT NULL,
			old_value TEXT,
			new_value TEXT,
			changed_by BIGINT UNSIGNED NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE,
			FOREIGN KEY (changed_by) REFERENCES users(id) ON DELETE CASCADE
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		`CREATE TABLE IF NOT EXISTS task_comments (
			id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
			task_id BIGINT UNSIGNED NOT NULL,
			user_id BIGINT UNSIGNED NOT NULL,
			content TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
	}

	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	}
	return nil
}

func TestUserStore_CreateAndGetByEmail(t *testing.T) {
	if testDB == nil {
		t.Skip("integration test requires docker")
	}

	store := NewUserStore(testDB)
	ctx := context.Background()

	user := &User{Email: "test@example.com", PasswordHash: "hash", Name: "Test"}
	if err := store.Create(ctx, user); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if user.ID == 0 {
		t.Fatal("expected non-zero ID")
	}

	fetched, err := store.GetByEmail(ctx, "test@example.com")
	if err != nil {
		t.Fatalf("GetByEmail failed: %v", err)
	}
	if fetched.Email != "test@example.com" {
		t.Fatalf("expected email test@example.com, got %s", fetched.Email)
	}
}

func TestUserStore_GetByID(t *testing.T) {
	if testDB == nil {
		t.Skip("integration test requires docker")
	}

	store := NewUserStore(testDB)
	ctx := context.Background()

	user := &User{Email: "idtest@example.com", PasswordHash: "hash", Name: "ID Test"}
	store.Create(ctx, user)

	fetched, err := store.GetByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if fetched.ID != user.ID {
		t.Fatalf("expected ID %d, got %d", user.ID, fetched.ID)
	}
}

func TestTeamStore_CreateAndList(t *testing.T) {
	if testDB == nil {
		t.Skip("integration test requires docker")
	}

	ctx := context.Background()
	userStore := NewUserStore(testDB)
	teamStore := NewTeamStore(testDB, nil)

	owner := &User{Email: "teamowner@example.com", PasswordHash: "hash", Name: "Owner"}
	userStore.Create(ctx, owner)

	team := &Team{Name: "Test Team", CreatedBy: owner.ID}
	if err := teamStore.Create(ctx, team); err != nil {
		t.Fatalf("Create team failed: %v", err)
	}

	teamStore.AddMember(ctx, team.ID, owner.ID, "owner")

	teams, err := teamStore.ListByUserID(ctx, owner.ID)
	if err != nil {
		t.Fatalf("ListByUserID failed: %v", err)
	}
	if len(teams) != 1 {
		t.Fatalf("expected 1 team, got %d", len(teams))
	}
}

func TestTeamStore_GetByID(t *testing.T) {
	if testDB == nil {
		t.Skip("integration test requires docker")
	}

	ctx := context.Background()
	userStore := NewUserStore(testDB)
	teamStore := NewTeamStore(testDB, nil)

	user := &User{Email: "getbyidteam@example.com", PasswordHash: "hash", Name: "GetByID"}
	userStore.Create(ctx, user)

	team := &Team{Name: "Find Me", CreatedBy: user.ID}
	teamStore.Create(ctx, team)

	fetched, err := teamStore.GetByID(ctx, team.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if fetched.Name != "Find Me" {
		t.Fatalf("expected 'Find Me', got %s", fetched.Name)
	}
}

func TestTeamStore_GetMemberRole(t *testing.T) {
	if testDB == nil {
		t.Skip("integration test requires docker")
	}

	ctx := context.Background()
	userStore := NewUserStore(testDB)
	teamStore := NewTeamStore(testDB, nil)

	user := &User{Email: "memberrole@example.com", PasswordHash: "hash", Name: "Role"}
	userStore.Create(ctx, user)

	team := &Team{Name: "Role Team", CreatedBy: user.ID}
	teamStore.Create(ctx, team)
	teamStore.AddMember(ctx, team.ID, user.ID, "admin")

	role, err := teamStore.GetMemberRole(ctx, team.ID, user.ID)
	if err != nil {
		t.Fatalf("GetMemberRole failed: %v", err)
	}
	if role != "admin" {
		t.Fatalf("expected 'admin', got %s", role)
	}
}

func TestTaskStore_IsMember(t *testing.T) {
	if testDB == nil {
		t.Skip("integration test requires docker")
	}

	ctx := context.Background()
	userStore := NewUserStore(testDB)
	teamStore := NewTeamStore(testDB, nil)
	taskStore := NewTaskStore(testDB, nil)

	user := &User{Email: "ismember@example.com", PasswordHash: "hash", Name: "IsMember"}
	userStore.Create(ctx, user)

	team := &Team{Name: "Member Team", CreatedBy: user.ID}
	teamStore.Create(ctx, team)
	teamStore.AddMember(ctx, team.ID, user.ID, "owner")

	member, err := taskStore.IsMember(ctx, team.ID, user.ID)
	if err != nil {
		t.Fatalf("IsMember failed: %v", err)
	}
	if !member {
		t.Fatal("expected user to be member")
	}

	nonMember, err := taskStore.IsMember(ctx, team.ID, 99999)
	if err != nil {
		t.Fatalf("IsMember (non-member) failed: %v", err)
	}
	if nonMember {
		t.Fatal("expected user not to be member")
	}
}

func TestTeamStore_GetMembers(t *testing.T) {
	if testDB == nil {
		t.Skip("integration test requires docker")
	}

	ctx := context.Background()
	userStore := NewUserStore(testDB)
	teamStore := NewTeamStore(testDB, nil)

	owner := &User{Email: "getmembers_owner@example.com", PasswordHash: "hash", Name: "Owner"}
	userStore.Create(ctx, owner)
	member := &User{Email: "getmembers_user@example.com", PasswordHash: "hash", Name: "Member"}
	userStore.Create(ctx, member)

	team := &Team{Name: "GetMembers Team", CreatedBy: owner.ID}
	teamStore.Create(ctx, team)
	teamStore.AddMember(ctx, team.ID, owner.ID, "owner")
	teamStore.AddMember(ctx, team.ID, member.ID, "member")

	members, err := teamStore.GetMembers(ctx, team.ID)
	if err != nil {
		t.Fatalf("GetMembers failed: %v", err)
	}
	if len(members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(members))
	}
}

func TestTaskStore_List_Filters(t *testing.T) {
	if testDB == nil {
		t.Skip("integration test requires docker")
	}

	ctx := context.Background()
	userStore := NewUserStore(testDB)
	teamStore := NewTeamStore(testDB, nil)
	taskStore := NewTaskStore(testDB, nil)

	user := &User{Email: "listfilter@example.com", PasswordHash: "hash", Name: "ListFilter"}
	userStore.Create(ctx, user)
	assignee := &User{Email: "assignee@example.com", PasswordHash: "hash", Name: "Assignee"}
	userStore.Create(ctx, assignee)

	team := &Team{Name: "List Team", CreatedBy: user.ID}
	teamStore.Create(ctx, team)
	teamStore.AddMember(ctx, team.ID, user.ID, "owner")

	for i := 0; i < 3; i++ {
		taskStore.Create(ctx, &Task{
			Title: fmt.Sprintf("Task %d", i), Status: "todo", TeamID: team.ID, CreatedBy: user.ID,
		})
	}
	taskStore.Create(ctx, &Task{
		Title: "In Progress", Status: "in_progress", TeamID: team.ID, CreatedBy: user.ID,
	})

	statusFilter := "in_progress"
	tasks, total, err := taskStore.List(ctx, TaskFilter{TeamID: &team.ID, Status: &statusFilter, Page: 1, PerPage: 10})
	if err != nil {
		t.Fatalf("List with status filter failed: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected 1 task with status 'in_progress', got %d", total)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}

	tasksNoTeam, totalNoTeam, err := taskStore.List(ctx, TaskFilter{Page: 1, PerPage: 10})
	if err != nil {
		t.Fatalf("List without team filter failed: %v", err)
	}
	if totalNoTeam != 4 {
		t.Fatalf("expected 4 total tasks, got %d", totalNoTeam)
	}
	if len(tasksNoTeam) != 4 {
		t.Fatalf("expected 4 tasks, got %d", len(tasksNoTeam))
	}
}

func TestTaskStore_CRUD(t *testing.T) {
	if testDB == nil {
		t.Skip("integration test requires docker")
	}

	ctx := context.Background()
	userStore := NewUserStore(testDB)
	teamStore := NewTeamStore(testDB, nil)
	taskStore := NewTaskStore(testDB, nil)

	user := &User{Email: "taskuser@example.com", PasswordHash: "hash", Name: "Task User"}
	userStore.Create(ctx, user)

	team := &Team{Name: "Task Team", CreatedBy: user.ID}
	teamStore.Create(ctx, team)
	teamStore.AddMember(ctx, team.ID, user.ID, "owner")

	task := &Task{Title: "Test Task", Status: "todo", TeamID: team.ID, CreatedBy: user.ID}
	if err := taskStore.Create(ctx, task); err != nil {
		t.Fatalf("Create task failed: %v", err)
	}
	if task.ID == 0 {
		t.Fatal("expected non-zero task ID")
	}

	fetched, err := taskStore.GetByID(ctx, task.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if fetched.Title != "Test Task" {
		t.Fatalf("expected 'Test Task', got %s", fetched.Title)
	}

	fetched.Status = "done"
	if err := taskStore.Update(ctx, fetched); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	taskStore.AppendHistory(ctx, task.ID, user.ID, "status", "todo", "done")

	history, err := taskStore.GetHistory(ctx, task.ID)
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(history))
	}

	tasks, total, err := taskStore.List(ctx, TaskFilter{TeamID: &team.ID, Page: 1, PerPage: 10})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected total 1, got %d", total)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
}
