package store

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"notifier/internal/notification"
)

type PostgresStore struct {
	db     *pgxpool.Pool
	logger *slog.Logger
}

func NewPostgresStore(db *pgxpool.Pool, logger *slog.Logger) *PostgresStore {
	return &PostgresStore{
		db:     db,
		logger: logger,
	}
}

func (s *PostgresStore) Save(ctx context.Context, n notification.Notification) (int, error) {
	const op = "PostgresStore.Save"

	var id int
	err := s.db.QueryRow(
		ctx,
		`INSERT INTO notifications (recipient, subject, body, channel, is_urgent, status)
		VALUES ($1, $2, $3, $4, $5, 'pending')
		RETURNING id`,
		n.Recipient, n.Subject, n.Body, n.Channel, n.IsUrgent,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("%s: scan: %w", op, err)
	}

	return id, nil
}

func (s *PostgresStore) GetAll(ctx context.Context) ([]notification.Notification, error) {
	const op = "PostgresStore.GetAll"

	rows, err := s.db.Query(
		ctx,
		`SELECT id, recipient, subject, body, channel, is_urgent, status FROM notifications ORDER BY id`,
	)
	if err != nil {
		return nil, fmt.Errorf("%s: query: %w", op, err)
	}
	defer rows.Close()

	result, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (notification.Notification, error) {
		var n notification.Notification
		scanErr := row.Scan(&n.ID, &n.Recipient, &n.Subject, &n.Body, &n.Channel, &n.IsUrgent, &n.Status)
		return n, scanErr
	})
	if err != nil {
		return nil, fmt.Errorf("%s: collect: %w", op, err)
	}

	return result, nil
}

func (s *PostgresStore) GetById(ctx context.Context, id int) (notification.Notification, error) {
	const op = "PostgresStore.GetById"

	var n notification.Notification
	err := s.db.QueryRow(
		ctx,
		`SELECT id, recipient, subject, body, channel, is_urgent, status FROM notifications WHERE id=$1`,
		id,
	).Scan(&n.ID, &n.Recipient, &n.Subject, &n.Body, &n.Channel, &n.IsUrgent, &n.Status)

	if errors.Is(err, pgx.ErrNoRows) {
		return notification.Notification{}, fmt.Errorf("%s: scan: %w", op, notification.ErrNotFound)
	}
	if err != nil {
		return notification.Notification{}, fmt.Errorf("%s: scan: %w", op, err)
	}

	return n, nil
}

func (s *PostgresStore) UpdateStatus(ctx context.Context, id int, status string) error {
	const op = "PostgresStore.UpdateStatus"

	tag, err := s.db.Exec(
		ctx,
		`UPDATE notifications SET status=$1 WHERE id=$2`,
		status, id,
	)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%s: %w", op, notification.ErrNotFound)
	}

	return nil
}
