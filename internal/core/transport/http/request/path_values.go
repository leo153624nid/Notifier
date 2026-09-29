package core_http_request

import (
	"fmt"
	"net/http"
	"strconv"

	core_errors "notifier/internal/core/errors"
)

func GetIntPathValue(r *http.Request, key string) (int, error) {
	pathValue := r.PathValue(key)
	if pathValue == "" {
		return 0, fmt.Errorf("no path value for key: %s, %w", key, core_errors.ErrInvalidArgument)
	}

	val, err := strconv.Atoi(pathValue)
	if err != nil {
		return 0, fmt.Errorf("wrong path value for key: %s, %w", key, core_errors.ErrInvalidArgument)
	}

	return val, nil
}
