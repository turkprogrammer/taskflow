package lib

import (
	"errors"
	"log/slog"
	"time"

	"github.com/sony/gobreaker"
)

// CircuitBreaker — обёртка над sony/gobreaker для отказоустойчивости.
// Защищает вызовы внешних сервисов от каскадных сбоев: отклоняет вызовы
// при превышении порога ошибок и периодически проверяет восстановление.
type CircuitBreaker struct {
	cb *gobreaker.CircuitBreaker
}

// NewCircuitBreaker создаёт circuit breaker, который переходит в открытое состояние
// после maxFailures последовательных ошибок и остаётся открытым timeout перед
// допуском одного пробного вызова (half-open).
func NewCircuitBreaker(maxFailures int, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		cb: gobreaker.NewCircuitBreaker(gobreaker.Settings{
			Name:        "email",            // имя для логирования
			MaxRequests: 1,                  // допустимые запросы в half-open
			Interval:    0,                  // без сброса счётчика по времени
			Timeout:     timeout,            // время восстановления после открытия
			ReadyToTrip: func(counts gobreaker.Counts) bool {
				// Переход в open при достижении порога ошибок
				return counts.ConsecutiveFailures >= uint32(maxFailures)
			},
			OnStateChange: func(_ string, from, to gobreaker.State) {
				slog.Info("circuit breaker state changed", "from", from, "to", to)
			},
		}),
	}
}

// Execute выполняет функцию с защитой circuit breaker.
// Возвращает ErrCircuitOpen, если цепь разомкнута (открыта).
func (cb *CircuitBreaker) Execute(fn func() error) error {
	_, err := cb.cb.Execute(func() (interface{}, error) {
		return nil, fn()
	})
	if errors.Is(err, gobreaker.ErrOpenState) {
		return ErrCircuitOpen
	}
	return err
}

// EmailSender — интерфейс для отправки транзакционных email-уведомлений.
// Реализации должны обрабатывать транзентные ошибки через circuit breaker.
type EmailSender interface {
	Send(to, subject, body string) error
}

// MockEmailSender — мок для тестирования отправки email.
// Когда ShouldFail=true, метод Send возвращает ошибку через circuit breaker.
type MockEmailSender struct {
	ShouldFail bool          // флаг для имитации ошибок
	cb         *CircuitBreaker // circuit breaker для тестирования
}

// NewMockEmailSender создаёт мок-отправитель. Установите shouldFail=true
// для тестирования сценариев ошибок и retry.
func NewMockEmailSender(shouldFail bool) *MockEmailSender {
	return &MockEmailSender{
		ShouldFail: shouldFail,
		cb:         NewCircuitBreaker(3, 30*time.Second), // 3 ошибки → 30 сек восстановления
	}
}

// ErrCircuitOpen — возвращается circuit breaker, когда цепь разомкнута.
var ErrCircuitOpen = &circuitError{"circuit breaker is open"}

type circuitError struct{ msg string }

func (e *circuitError) Error() string { return e.msg }

// Send имитирует отправку email. При ShouldFail=true возвращает ошибку,
// что позволяет тестировать переходы circuit breaker.
func (m *MockEmailSender) Send(to, subject, body string) error {
	return m.cb.Execute(func() error {
		if m.ShouldFail {
			return &emailError{"email service unavailable"}
		}
		slog.Info("email sent", "to", to, "subject", subject)
		return nil
	})
}

type emailError struct{ msg string }

func (e *emailError) Error() string { return e.msg }
