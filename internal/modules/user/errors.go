package user

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/optikklabs/query/internal/shared/errorcode"
	modulecommon "github.com/optikklabs/query/internal/shared/httputil"
)

type ServiceErrorCode string

const (
	ServiceErrorValidation   ServiceErrorCode = errorcode.Validation
	ServiceErrorUnauthorized ServiceErrorCode = errorcode.Unauthorized
	ServiceErrorNotFound     ServiceErrorCode = errorcode.NotFound
	ServiceErrorInternal     ServiceErrorCode = errorcode.Internal
)

// ServiceError represents an application service error.
type ServiceError struct {
	Code    ServiceErrorCode
	Message string
	Cause   error
}

func (e *ServiceError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

func (e *ServiceError) Unwrap() error {
	return e.Cause
}

func NewValidationError(message string, cause error) error {
	return &ServiceError{Code: ServiceErrorValidation, Message: message, Cause: cause}
}

func NewUnauthorizedError(message string, cause error) error {
	return &ServiceError{Code: ServiceErrorUnauthorized, Message: message, Cause: cause}
}

func NewNotFoundError(message string, cause error) error {
	return &ServiceError{Code: ServiceErrorNotFound, Message: message, Cause: cause}
}

func NewInternalError(message string, cause error) error {
	return &ServiceError{Code: ServiceErrorInternal, Message: message, Cause: cause}
}

func RespondServiceError(w http.ResponseWriter, r *http.Request, err error, fallbackMessage string) {
	var serviceErr *ServiceError
	if errors.As(err, &serviceErr) {
		status := http.StatusInternalServerError
		switch serviceErr.Code {
		case ServiceErrorValidation:
			status = http.StatusBadRequest
		case ServiceErrorUnauthorized:
			status = http.StatusUnauthorized
		case ServiceErrorNotFound:
			status = http.StatusNotFound
		case ServiceErrorInternal:
			status = http.StatusInternalServerError
		}

		message := serviceErr.Message
		if message == "" {
			message = fallbackMessage
		}
		if message == "" {
			message = "Internal error"
		}

		code := string(serviceErr.Code)
		if code == "" {
			code = string(ServiceErrorInternal)
		}
		modulecommon.RespondError(w, r, status, code, message)
		return
	}

	if fallbackMessage == "" {
		fallbackMessage = "Internal error"
	}
	modulecommon.RespondError(w, r, http.StatusInternalServerError, string(ServiceErrorInternal), fallbackMessage)
}
