package core_http_utils

import (
	"fmt"
	"net/http"
	core_errors "notifier/internal/core/errors"
	"strconv"
)

func GetIntQueryParam(r *http.Request, key string) (*int, error) {
	param := r.URL.Query().Get(key)
	if param == "" {
		return nil, nil
	}

	val, err := strconv.Atoi(param)
	if err != nil {
		return nil, fmt.Errorf("param not a valid integer: %v: %w", err, core_errors.ErrInvalidArgument)
	}

	return &val, nil
}
