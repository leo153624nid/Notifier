package domain

import (
	"errors"
)

var ErrNotFound = errors.New("notification not found")
var ErrInvalidID = errors.New("invalid notification id")
