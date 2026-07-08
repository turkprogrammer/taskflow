package handler

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// responseWriter — обёртка над http.ResponseWriter для перехвата HTTP-статуса.
type responseWriter struct {
	http.ResponseWriter
	statusCode int // сохранённый статус-код ответа
}

// WriteHeader перехватывает статус-код и передаёт его дальше.
func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Metrics собирает Prometheus-метрики HTTP-запросов: общее количество,
// количество ошибок по статус-кодам и гистограмму длительности.
type Metrics struct {
	requestsTotal   *prometheus.CounterVec   // общее количество запросов
	requestsErrors  *prometheus.CounterVec   // количество ошибок (4xx/5xx)
	requestDuration *prometheus.HistogramVec // длительность запросов в секундах
}

// NewMetrics создаёт и регистрирует Prometheus-коллекторы.
// Паникует при дублировании регистрации (например, при повторном вызове).
func NewMetrics() *Metrics {
	m := &Metrics{
		requestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "taskflow_requests_total",
				Help: "Total number of HTTP requests",
			},
			[]string{"method", "path"},
		),
		requestsErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "taskflow_requests_errors_total",
				Help: "Total number of HTTP request errors",
			},
			[]string{"method", "path", "status"},
		),
		requestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "taskflow_request_duration_seconds",
				Help:    "HTTP request latency in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "path"},
		),
	}

	// Регистрация коллекторов (с обработкой дубликатов)
	if err := prometheus.Register(m.requestsTotal); err != nil {
		if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
			panic(err)
		}
	}
	if err := prometheus.Register(m.requestsErrors); err != nil {
		if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
			panic(err)
		}
	}
	if err := prometheus.Register(m.requestDuration); err != nil {
		if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
			panic(err)
		}
	}

	return m
}

// Middleware — HTTP middleware для сбора Prometheus-метрик.
// Записывает количество запросов, длительность и количество ошибок (4xx/5xx).
func (m *Metrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		// Вызов следующего хендлера
		next.ServeHTTP(rw, r)

		// Запись метрик
		duration := time.Since(start).Seconds()
		path := r.URL.Path
		m.requestsTotal.WithLabelValues(r.Method, path).Inc()
		m.requestDuration.WithLabelValues(r.Method, path).Observe(duration)

		// Учёт ошибок (4xx и 5xx)
		if rw.statusCode >= 400 {
			m.requestsErrors.WithLabelValues(r.Method, path, http.StatusText(rw.statusCode)).Inc()
		}
	})
}

// Handler возвращает стандартный HTTP-хендлер Prometheus для /metrics.
// Монтируется на отдельном эндпоинте без авторизации для скрапинга.
func (m *Metrics) Handler() http.Handler {
	return promhttp.Handler()
}
