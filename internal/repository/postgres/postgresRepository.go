package postgres

import (
	"github.com/jackc/pgx/v5/pgxpool"

	core_logger "notifier/internal/core/logger"
)

type PostgresRepository struct {
	db     *pgxpool.Pool
	logger *core_logger.Logger
}

func NewPostgresRepository(
	db *pgxpool.Pool,
	logger *core_logger.Logger,
) *PostgresRepository {
	return &PostgresRepository{
		db:     db,
		logger: logger,
	}
}
