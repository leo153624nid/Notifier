// Command migrate применяет/откатывает миграции схемы БД auth.
// Живёт отдельно от cmd/auth: в проде схема — ответственность
// деплой-шага (см. docker-compose.yml, сервис migrate), а не самого
// приложения, которое может подниматься несколькими репликами.
//
// Использование:
//
//	migrate up             применить все непринятые миграции
//	migrate down            откатить последнюю применённую миграцию
//	migrate version         показать текущую версию схемы
//	migrate force <version> снять флаг dirty, зафиксировав версию вручную
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"authservice/internal/config"
	"authservice/internal/migrations"
)

func main() {
	flag.Parse()

	cmd := "up"
	if flag.NArg() > 0 {
		cmd = flag.Arg(0)
	}

	if err := run(cmd); err != nil {
		fmt.Fprintf(os.Stderr, "migrate: %s\n", err)
		os.Exit(1)
	}
}

func run(cmd string) error {
	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("source: %w", err)
	}

	dsn := config.LoadPostgresConfig().MigrateDSN()

	m, err := migrate.NewWithSourceInstance("iofs", src, dsn)
	if err != nil {
		return fmt.Errorf("init: %w", err)
	}
	defer func() {
		if srcErr, dbErr := m.Close(); srcErr != nil || dbErr != nil {
			fmt.Fprintf(os.Stderr, "migrate: close: src=%v db=%v\n", srcErr, dbErr)
		}
	}()

	switch cmd {
	case "up":
		err = m.Up()
	case "down":
		err = m.Steps(-1)
	case "version":
		version, dirty, vErr := m.Version()
		if vErr != nil && !errors.Is(vErr, migrate.ErrNilVersion) {
			return fmt.Errorf("version: %w", vErr)
		}
		fmt.Printf("version=%d dirty=%v\n", version, dirty)
		return nil
	case "force":
		if flag.NArg() < 2 {
			return fmt.Errorf("force: usage: migrate force <version>")
		}
		version, convErr := strconv.Atoi(flag.Arg(1))
		if convErr != nil {
			return fmt.Errorf("force: invalid version %q: %w", flag.Arg(1), convErr)
		}
		err = m.Force(version)
	default:
		return fmt.Errorf("unknown command %q (use: up|down|version|force)", cmd)
	}

	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("%s: %w", cmd, err)
	}

	fmt.Println("migrate:", cmd, "ok")
	return nil
}
