package store

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"notifier/internal/notification"
)

type PostgresStore struct {
	db     *sql.DB
	logger *slog.Logger
}

func NewPostgresStore(db *sql.DB, logger *slog.Logger) *PostgresStore {
	return &PostgresStore{
		db:     db,
		logger: logger,
	}
}

func (s *PostgresStore) Save(n notification.Notification) (int, error) {
	const op = "PostgresStore.Save"

	var id int
	err := s.db.QueryRow(
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

func (s *PostgresStore) GetAll() ([]notification.Notification, error) {
	const op = "PostgresStore.GetAll"
	var result []notification.Notification

	rows, err := s.db.Query(
		`SELECT id, recipient, subject, body, channel, is_urgent, status FROM notifications ORDER BY id`,
	)
	if err != nil {
		return nil, fmt.Errorf("%s: query: %w", op, err)
	}

	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			s.logger.Error("%s: close: %w", op, closeErr)
		}
	}()

	for rows.Next() {
		var n notification.Notification
		err = rows.Scan(&n.ID, &n.Recipient, &n.Subject, &n.Body, &n.Channel, &n.IsUrgent, &n.Status)
		if err != nil {
			return nil, fmt.Errorf("%s: scan: %w", op, err)
		}

		result = append(result, n)
	}

	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("%s: rows: %w", op, err)
	}

	return result, nil
}

func (s *PostgresStore) GetById(id int) (notification.Notification, error) {
	const op = "PostgresStore.GetById"

	var n notification.Notification
	err := s.db.QueryRow(
		`SELECT id, recipient, subject, body, channel, is_urgent, status FROM notifications WHERE id=$1`,
		id,
	).Scan(&n.ID, &n.Recipient, &n.Subject, &n.Body, &n.Channel, &n.IsUrgent, &n.Status)

	if errors.Is(err, sql.ErrNoRows) {
		return notification.Notification{}, fmt.Errorf("%s: scan: %w", op, notification.ErrNotFound)
	}
	if err != nil {
		return notification.Notification{}, fmt.Errorf("%s: scan: %w", op, err)
	}

	return n, nil
}

func (s *PostgresStore) UpdateStatus(id int, status string) error {
	const op = "PostgresStore.UpdateStatus"

	result, err := s.db.Exec(
		`UPDATE notifications SET status=$1 WHERE id=$2`,
		status, id,
	)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("%s: %w", op, notification.ErrNotFound)
	}

	return nil
}
