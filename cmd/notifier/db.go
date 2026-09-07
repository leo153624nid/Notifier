package main

import (
	"database/sql"
	"errors"
	"fmt"

	"notifier/internal/notification"
)

func insertNotification(db *sql.DB, n Notification) (int, error) {
	const op = "insertNotification"

	var id int
	err := db.QueryRow(
		`INSERT INTO notifications (recipient, subject, body, channel, is_urgent) 
		VALUES ($1, $2, $3, $4, $5) 
		RETURNING id`,
		n.Recipient, n.Subject, n.Body, n.Channel, n.IsUrgent,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("%s: scan: %w", op, err)
	}

	return id, nil
}

func getAllNotifications(db *sql.DB) ([]Notification, error) {
	const op = "getAllNotifications"

	rows, err := db.Query(
		`SELECT id, recipient, subject, body, channel, is_urgent FROM notifications ORDER BY id`,
	)
	if err != nil {
		return nil, fmt.Errorf("%s: query: %w", op, err)
	}

	defer rows.Close()

	var result []Notification
	for rows.Next() {
		var n Notification
		err := rows.Scan(&n.ID, &n.Recipient, &n.Subject, &n.Body, &n.Channel, &n.IsUrgent)
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

func notificationById(db *sql.DB, id int) (Notification, error) {
	const op = "notificationById"

	var n Notification
	err := db.QueryRow(
		`SELECT id, recipient, subject, body, channel, is_urgent FROM notifications WHERE id=$1`,
		id,
	).Scan(&n.ID, &n.Recipient, &n.Subject, &n.Body, &n.Channel, &n.IsUrgent)

	if errors.Is(err, sql.ErrNoRows) {
		return Notification{}, fmt.Errorf("%s: scan: %w", op, notification.ErrNotFound)
	}
	if err != nil {
		return Notification{}, fmt.Errorf("%s: scan: %w", op, err)
	}

	return n, nil
}
