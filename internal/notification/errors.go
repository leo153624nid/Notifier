package notification

import (
	"errors"
)

var ErrNotFound = errors.New("notification not found")
var ErrInvalidId = errors.New("invalid notification id")
