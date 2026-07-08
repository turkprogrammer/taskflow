# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2026-07-07

### Added

- User registration and JWT-based authentication
- Team management (create, list, invite with role-based access)
- Task CRUD with filtering, pagination, and full audit history
- Complex SQL reports: team dashboard (JOIN + aggregation), top users per team (window function), integrity validation query
- Redis caching for team list with TTL-based invalidation
- MySQL connection pooling and database indexes
- Rate limiting middleware (100 req/min per user)
- Circuit breaker pattern for resilient external service calls
- Prometheus metrics (request count, duration, error rate)
- Graceful shutdown on SIGINT/SIGTERM
- JSON logging with slog
- Configurable via YAML file with environment variable overrides
- Docker Compose setup for MySQL 8 and Redis 7
- Unit tests for service layer (85.3% coverage)
- Integration tests with dockertest (MySQL)
- Comprehensive godoc comments across all packages
- README, CONTRIBUTING, and AI-friendly llms.txt documentation
