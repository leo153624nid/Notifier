package service

import "errors"

var (
	ErrInvalidNotification = errors.New("invalid notification")
	ErrUnsupportedChannel  = errors.New("unsupported channel")
)
