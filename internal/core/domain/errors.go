package domain

import (
	"errors"
)

var (
	ErrNotFound              = errors.New("notification not found")
	ErrInvalidNotification   = errors.New("invalid notification")
	ErrInvalidID             = errors.New("invalid notification id")
	ErrUnsupportedChannel    = errors.New("unsupported channel")
	ErrEventAlreadyProcessed = errors.New("event already processed")
)
