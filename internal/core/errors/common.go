package core_errors

import (
	"errors"
)

var (
	ErrNotFound              = errors.New("not found")
	ErrInvalidArgument       = errors.New("invalid argument")
	ErrInvalidNotification   = errors.New("invalid notification")
	ErrUnsupportedChannel    = errors.New("unsupported channel")
	ErrConflict              = errors.New("conflict")
	ErrEventAlreadyProcessed = errors.New("event already processed")
	ErrAuth                  = errors.New("auth failed")
)
