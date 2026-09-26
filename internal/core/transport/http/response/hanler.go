package core_http_response

import (
	"encoding/json"
	"fmt"
	"net/http"
	core_logger "notifier/internal/core/logger"

	"go.uber.org/zap"
)

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

func (h *HTTPResponseHandler) PanicResponse(p any, msg string) {
	statusCode := http.StatusInternalServerError
	err := fmt.Errorf("unexpected panic: %v", p)

	h.logger.Error(msg, zap.Error(err))
	h.rw.WriteHeader(statusCode)

	response := map[string]string{
		"message": msg,
		"error":   err.Error(),
	}

	if err := json.NewEncoder(h.rw).Encode(response); err != nil {
		h.logger.Error("failed encode panic response", zap.Error(err))
	}
}
