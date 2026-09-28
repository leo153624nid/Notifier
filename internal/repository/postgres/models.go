package postgres

import "notifier/internal/core/domain"

type NotificationModel struct {
	Recipient string
	Subject   string
	Body      string
	Channel   string
	Status    string
	ID        int
	IsUrgent  bool
}

func (model *NotificationModel) toDomain() domain.Notification {
	return domain.NewNotification(
		model.ID,
		model.Recipient,
		model.Subject,
		model.Body,
		model.Channel,
		model.Status,
		model.IsUrgent,
	)
}

func listModelsToDomain(models []NotificationModel) []domain.Notification {
	result := make([]domain.Notification, len(models))

	for _, m := range models {
		result = append(result, m.toDomain())
	}

	return result
}
