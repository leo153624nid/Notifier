package domain

import (
	"fmt"
	"unicode/utf8"
)

//nolint:govet
type Notification struct {
	ID        int    `json:"id"`
	Recipient string `json:"to"`
	Subject   string `json:"subject"`
	Body      string `json:"body"`
	Channel   string `json:"channel"`
	Status    string `json:"status"`
	IsUrgent  bool   `json:"urgent"`
}

func (n Notification) Validate() error {
	const op = "Notification.Validate"

	if n.Recipient == "" {
		return fmt.Errorf("%s: recipient is required", op)
	}
	if n.Channel == "" {
		return fmt.Errorf("%s: channel is required", op)
	}
	if utf8.RuneCountInString(n.Channel) > 20 {
		return fmt.Errorf("%s: channel is long", op)
	}
	return nil
}

func (n Notification) String() string {
	return fmt.Sprintf("Notification{to:%s, subject:%s, channel:%s}", n.Recipient, n.Subject, n.Channel)
}
