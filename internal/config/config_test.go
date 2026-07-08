package config

import (
	"os"
	"testing"
	"time"
)

func TestLoad_Defaults(t *testing.T) {
	content := []byte(`
server:
  port: 9090
db:
  host: localhost
  port: 3306
  user: test
  password: test
  name: testdb
  max_open_conns: 5
  max_idle_conns: 2
  max_lifetime_seconds: 300
redis:
  addr: localhost:6379
  password: ""
  db: 0
jwt:
  secret: test-secret
  ttl_hours: 24
`)

	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(content); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	cfg, err := Load(tmpFile.Name())
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Server.Port != 9090 {
		t.Fatalf("expected port 9090, got %d", cfg.Server.Port)
	}
	if cfg.DB.Host != "localhost" {
		t.Fatalf("expected host localhost, got %s", cfg.DB.Host)
	}
	if cfg.DB.MaxOpenConns != 5 {
		t.Fatalf("expected max_open_conns 5, got %d", cfg.DB.MaxOpenConns)
	}
	if cfg.JWT.Secret != "test-secret" {
		t.Fatalf("expected secret test-secret, got %s", cfg.JWT.Secret)
	}
	if cfg.JWT.TTL() != 24*time.Hour {
		t.Fatalf("expected TTL 24h, got %v", cfg.JWT.TTL())
	}
}

func TestLoad_EnvOverride(t *testing.T) {
	os.Setenv("DB_HOST", "env-host")
	os.Setenv("DB_PORT", "3316")
	os.Setenv("DB_USER", "env-user")
	os.Setenv("DB_PASSWORD", "env-pass")
	os.Setenv("REDIS_ADDR", "env-redis:6380")
	os.Setenv("JWT_SECRET", "env-secret")
	os.Setenv("SERVER_PORT", "8081")
	defer func() {
		os.Unsetenv("DB_HOST")
		os.Unsetenv("DB_PORT")
		os.Unsetenv("DB_USER")
		os.Unsetenv("DB_PASSWORD")
		os.Unsetenv("REDIS_ADDR")
		os.Unsetenv("JWT_SECRET")
		os.Unsetenv("SERVER_PORT")
	}()

	content := []byte(`
server:
  port: 8080
db:
  host: yaml-host
  port: 3306
  user: yaml-user
  password: yaml-pass
  name: yamldb
  max_open_conns: 10
  max_idle_conns: 5
  max_lifetime_seconds: 300
redis:
  addr: yaml-redis:6379
  password: ""
  db: 0
jwt:
  secret: yaml-secret
  ttl_hours: 72
`)

	tmpFile, _ := os.CreateTemp("", "config-*.yaml")
	defer os.Remove(tmpFile.Name())
	tmpFile.Write(content)
	tmpFile.Close()

	cfg, err := Load(tmpFile.Name())
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.DB.Host != "env-host" {
		t.Fatalf("expected env-host, got %s", cfg.DB.Host)
	}
	if cfg.DB.Port != 3316 {
		t.Fatalf("expected 3316, got %d", cfg.DB.Port)
	}
	if cfg.DB.User != "env-user" {
		t.Fatalf("expected env-user, got %s", cfg.DB.User)
	}
	if cfg.Redis.Addr != "env-redis:6380" {
		t.Fatalf("expected env-redis:6380, got %s", cfg.Redis.Addr)
	}
	if cfg.JWT.Secret != "env-secret" {
		t.Fatalf("expected env-secret, got %s", cfg.JWT.Secret)
	}
	if cfg.Server.Port != 8081 {
		t.Fatalf("expected 8081, got %d", cfg.Server.Port)
	}
}

func TestDSN(t *testing.T) {
	cfg := DBConfig{
		User:     "myuser",
		Password: "mypass",
		Host:     "myhost",
		Port:     3307,
		Name:     "mydb",
	}
	dsn := cfg.DSN()
	expected := "myuser:mypass@tcp(myhost:3307)/mydb?charset=utf8mb4&parseTime=true&loc=Local"
	if dsn != expected {
		t.Fatalf("expected %s, got %s", expected, dsn)
	}
}
