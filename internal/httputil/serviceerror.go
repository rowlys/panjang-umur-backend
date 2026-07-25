package httputil

import (
	"errors"
	"net/http"
)

type ServiceError struct {
	Code    int
	Message string
}

func (e *ServiceError) Error() string {
	return e.Message
}

func ResolveServiceError(err error) (int, string) {
	var serviceErr *ServiceError
	if errors.As(err, &serviceErr) {
		return serviceErr.Code, serviceErr.Message
	}
	return http.StatusInternalServerError, "Internal server error"
}
