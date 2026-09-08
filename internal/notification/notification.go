package notification

import (
	"fmt"
)

type Notification struct {
	ID        int    `json:"id"`
	Recipient string `json:"to"`
	Subject   string `json:"subject"`
	Body      string `json:"body"`
	Channel   string `json:"channel"`
	IsUrgent  bool   `json:"urgent"`
	Status    string `json:"status"`
}

func (n Notification) Validate() error {
	const op = "Notification.Validate"

	if n.Recipient == "" {
		return fmt.Errorf("%s: recipient is required", op)
	}
	if n.Channel == "" {
		return fmt.Errorf("%s: channel is required", op)
	}
	return nil
}

func (n Notification) String() string {
	return fmt.Sprintf("Notification{to:%s, subject:%s, channel:%s}", n.Recipient, n.Subject, n.Channel)
}
