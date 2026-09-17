package domain

import "errors"

var ErrUserExists = errors.New("User already exists")
var ErrInvalidCredentials = errors.New("Invalid credentials")
