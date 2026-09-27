package core_http_response

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	core_errors "notifier/internal/core/errors"
	core_logger "notifier/internal/core/logger"

	"go.uber.org/zap"
)

type APIError struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

type HTTPResponseHandler struct {
	logger *core_logger.Logger
	rw     http.ResponseWriter
}

func NewHTTPResponseHandler(
	l *core_logger.Logger,
	rw http.ResponseWriter,
) *HTTPResponseHandler {
	return &HTTPResponseHandler{
		logger: l,
		rw:     rw,
	}
}

func (h *HTTPResponseHandler) ErrorResponse(msg string, err error) {
	var (
		statusCode int
		logFunc    func(string, ...zap.Field)
	)

	switch {
	case errors.Is(err, core_errors.ErrInvalidArgument):
		statusCode = http.StatusBadRequest
		logFunc = h.logger.Warn

	case errors.Is(err, core_errors.ErrInvalidNotification):
		statusCode = http.StatusBadRequest
		logFunc = h.logger.Warn

	case errors.Is(err, core_errors.ErrUnsupportedChannel):
		statusCode = http.StatusBadRequest
		logFunc = h.logger.Warn

	case errors.Is(err, core_errors.ErrNotFound):
		statusCode = http.StatusNotFound
		logFunc = h.logger.Debug

	case errors.Is(err, core_errors.ErrConflict):
		statusCode = http.StatusConflict
		logFunc = h.logger.Warn

	default:
		statusCode = http.StatusInternalServerError
		logFunc = h.logger.Error
	}

	logFunc(
		msg,
		zap.Error(err),
	)

	h.errorResponse(
		statusCode,
		err,
		msg,
	)
}

func (h *HTTPResponseHandler) PanicResponse(p any, msg string) {
	statusCode := http.StatusInternalServerError
	err := fmt.Errorf("unexpected panic: %v", p)

	h.logger.Error(msg, zap.Error(err))

	h.errorResponse(
		statusCode,
		err,
		msg,
	)
}

func (h *HTTPResponseHandler) errorResponse(
	statusCode int,
	err error,
	msg string,
) {
	h.rw.WriteHeader(statusCode)

	response := APIError{
		Message: msg,
		Error:   err.Error(),
	}

	if err := json.NewEncoder(h.rw).Encode(response); err != nil {
		h.logger.Error("failed encode error response", zap.Error(err))
	}
}
