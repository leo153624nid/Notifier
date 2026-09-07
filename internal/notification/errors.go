package notification

import (
	"errors"
)

var ErrNotFound = errors.New("notification not found")
var ErrInvalidId = errors.New("invalid notification id")
var ErrInvalidChannel = errors.New("invalid channel")
var ErrFailedMarshal = errors.New("failed marshal")
var ErrInternal = errors.New("internal server error")
