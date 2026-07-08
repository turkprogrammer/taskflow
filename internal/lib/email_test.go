package lib

import (
	"errors"
	"testing"
	"time"
)

func TestCircuitBreaker_Closed(t *testing.T) {
	cb := NewCircuitBreaker(3, time.Second)

	calls := 0
	err := cb.Execute(func() error {
		calls++
		return nil
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

func TestCircuitBreaker_Opens_After_Failures(t *testing.T) {
	cb := NewCircuitBreaker(3, time.Second)

	for i := 0; i < 3; i++ {
		cb.Execute(func() error {
			return errors.New("fail")
		})
	}

	err := cb.Execute(func() error {
		return nil
	})
	if err == nil {
		t.Fatal("expected circuit open error")
	}
}

func TestCircuitBreaker_HalfOpen_After_Timeout(t *testing.T) {
	cb := NewCircuitBreaker(1, 50*time.Millisecond)

	cb.Execute(func() error {
		return errors.New("fail")
	})

	time.Sleep(60 * time.Millisecond)

	calls := 0
	err := cb.Execute(func() error {
		calls++
		return nil
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call in half-open, got %d", calls)
	}
}

func TestMockEmailSender_Success(t *testing.T) {
	sender := NewMockEmailSender(false)

	err := sender.Send("test@test.com", "Subject", "Body")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestMockEmailSender_Failure(t *testing.T) {
	sender := NewMockEmailSender(true)

	err := sender.Send("test@test.com", "Subject", "Body")
	if err == nil {
		t.Fatal("expected error when ShouldFail is true")
	}
}

func TestMockEmailSender_CircuitBreaker_Opens(t *testing.T) {
	sender := NewMockEmailSender(true)

	for i := 0; i < 3; i++ {
		sender.Send("test@test.com", "Subject", "Body")
	}

	err := sender.Send("test@test.com", "Subject", "Body")
	if err == nil {
		t.Fatal("expected circuit open error")
	}
}

func TestCircuitError_Error(t *testing.T) {
	err := ErrCircuitOpen
	if err.Error() != "circuit breaker is open" {
		t.Fatalf("unexpected error message: %s", err.Error())
	}
}
