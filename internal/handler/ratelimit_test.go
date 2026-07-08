package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

func TestRateLimiter_AllowsRequests(t *testing.T) {
	rl := NewRateLimiter(rate.Limit(100.0/60.0), 5)
	defer rl.StopCleanup()

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	ctx := context.WithValue(context.Background(), UserIDKey, uint64(1))
	req := httptest.NewRequest("GET", "/test", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	rl.Middleware(inner).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestRateLimiter_BlocksWhenExceeded(t *testing.T) {
	rl := NewRateLimiter(rate.Limit(1.0/60.0), 1)
	defer rl.StopCleanup()

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	ctx := context.WithValue(context.Background(), UserIDKey, uint64(1))

	req := httptest.NewRequest("GET", "/test", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	rl.Middleware(inner).ServeHTTP(w, req)

	req = httptest.NewRequest("GET", "/test", nil).WithContext(ctx)
	w = httptest.NewRecorder()
	rl.Middleware(inner).ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", w.Code)
	}
}

func TestRateLimiter_SkipsUnauthenticated(t *testing.T) {
	rl := NewRateLimiter(rate.Limit(1.0/60.0), 1)
	defer rl.StopCleanup()

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	rl.Middleware(inner).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestRateLimiter_Cleanup(t *testing.T) {
	rl := NewRateLimiter(rate.Limit(100.0/60.0), 5)
	rl.Cleanup(50*time.Millisecond, 100*time.Millisecond)

	ctx := context.WithValue(context.Background(), UserIDKey, uint64(1))
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	rl.Middleware(inner).ServeHTTP(w, req)

	rl.mu.Lock()
	if _, exists := rl.clients[uint64(1)]; !exists {
		rl.mu.Unlock()
		t.Fatal("expected client to be tracked")
	}
	rl.mu.Unlock()

	time.Sleep(200 * time.Millisecond)

	rl.mu.Lock()
	_, exists := rl.clients[uint64(1)]
	rl.mu.Unlock()

	if exists {
		t.Fatal("expected client to be cleaned up")
	}

	rl.StopCleanup()
}

func TestRateLimiter_StopCleanupIdempotent(t *testing.T) {
	rl := NewRateLimiter(rate.Limit(100.0/60.0), 5)
	rl.Cleanup(50*time.Millisecond, 100*time.Millisecond)
	rl.StopCleanup()
	rl.StopCleanup()
}
