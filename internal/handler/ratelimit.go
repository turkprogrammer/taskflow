package handler

import (
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// RateLimiter реализует per-user ограничение частоты запросов (token bucket).
// Каждый аутентифицированный пользователь получает свой rate.Limiter.
// Неаутентифицированные запросы пропускаются без ограничений.
type RateLimiter struct {
	mu        sync.Mutex                    // мьютекс для потокобезопасного доступа
	clients   map[uint64]*rate.Limiter      // лимитеры по user ID
	lastSeen  map[uint64]time.Time          // время последнего запроса для очистки
	rate      rate.Limit                    // количество запросов в секунду
	burst     int                           // максимальный размер "всплеска"
	stopClean chan struct{}                  // канал для остановки фоновой очистки
	closeOnce sync.Once                     // защита от двойного закрытия канала
}

// NewRateLimiter создаёт новый RateLimiter с указанным rate и burst.
// Пример: rate=100/60, burst=5 → ~100 запросов в минуту с всплесками до 5.
func NewRateLimiter(r rate.Limit, burst int) *RateLimiter {
	return &RateLimiter{
		clients:   make(map[uint64]*rate.Limiter),
		lastSeen:  make(map[uint64]time.Time),
		rate:      r,
		burst:     burst,
		stopClean: make(chan struct{}),
	}
}

// Middleware — HTTP middleware для per-user rate limiting.
// Если пользователь превышает лимит, возвращает 429 Too Many Requests.
// Размещается ПОСЛЕ AuthHandler.Middleware (для получения userID из контекста).
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Пропуск неаутентифицированных запросов
		userID, ok := UserIDFromContext(r.Context())
		if !ok {
			next.ServeHTTP(w, r)
			return
		}

		// Получение или создание лимитера для пользователя
		rl.mu.Lock()
		limiter, exists := rl.clients[userID]
		if !exists {
			limiter = rate.NewLimiter(rl.rate, rl.burst)
			rl.clients[userID] = limiter
		}
		rl.lastSeen[userID] = time.Now()
		rl.mu.Unlock()

		// Проверка лимита
		if !limiter.Allow() {
			writeJSON(w, http.StatusTooManyRequests, map[string]string{
				"error": "rate limit exceeded (100 req/min)",
			})
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Cleanup запускает фоновую горутину для периодической очистки неактивных лимитеров.
// Вызывается один раз при старте приложения для предотвращения утечки памяти.
func (rl *RateLimiter) Cleanup(interval time.Duration, idle time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				// Удаление лимитеров для пользователей без активности > idle
				rl.mu.Lock()
				now := time.Now()
				for id, last := range rl.lastSeen {
					if now.Sub(last) > idle {
						delete(rl.clients, id)
						delete(rl.lastSeen, id)
					}
				}
				rl.mu.Unlock()
			case <-rl.stopClean:
				return
			}
		}
	}()
}

// StopCleanup останавливает фоновую горутину очистки.
// Безопасно для повторного вызова. Вызывается при graceful shutdown.
func (rl *RateLimiter) StopCleanup() {
	rl.closeOnce.Do(func() {
		close(rl.stopClean)
	})
}
