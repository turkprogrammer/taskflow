// Пackage migrate реализует программный запуск миграций БД через goose.
package migrate

import (
	"database/sql"
	"embed"
	"fmt"
	"log/slog"

	"github.com/pressly/goose/v3"
)

// Run применяет все незапущенные миграции из embedded-файловой системы.
// Использует goose с диалектом "mysql" и таблицей "schema_migrations" для отслеживания.
// Вызывается при старте приложения перед началом обработки запросов.
func Run(db *sql.DB, fsys embed.FS, dir string) error {
	// Настройка goose
	goose.SetBaseFS(fsys)
	goose.SetDialect("mysql")
	goose.SetTableName("schema_migrations")

	// Применение миграций (Up)
	if err := goose.Up(db, dir); err != nil {
		return fmt.Errorf("running migrations: %w", err)
	}

	slog.Info("migrations applied successfully")
	return nil
}
